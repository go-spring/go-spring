/*
 * Copyright 2025 The Go-Spring Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      https://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package StarterConfigNacos integrates Nacos as a remote configuration
// center for Go-Spring. Blank-importing this package registers a "nacos"
// config provider that can be consumed via spring.config.import, together with
// the bridge that wires remote config changes into the application-wide
// property refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery
// (Nacos naming) is a separate concern and is not provided here.
package StarterConfigNacos

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/netutil"
)

func init() {
	// Register "nacos" as a remote configuration provider. The controller
	// itself is registered (not just its Load method): the runtime holds the
	// registered provider so it can stop the listeners at shutdown (see
	// provider.Provider). Watch-triggered refreshes go through the
	// gs.RefreshProperties package-level facade, so the controller needs no
	// bean wiring at all.
	conf.RegisterProvider("nacos", newNacosCtrl())
}

var starterTag = log.RegisterAppTag("config_nacos", "")

// nacosCtrl is the single object that owns the full lifecycle of nacos
// configuration: loading configs, listening for changes, and triggering
// property refresh. Its clients are bootstrap infrastructure: they exist
// before the container does and therefore deliberately opt out of
// observability and governance wiring — don't "fix" that here; consumers
// that need instrumented clients (e.g. the governance source) build their
// own.
type nacosCtrl struct {
	mu       sync.Mutex
	clients  map[string]config_client.IConfigClient
	listened map[string]struct{}

	// ctx is the listener generation: Close cancels it and closes every client,
	// which stops the change callbacks; the next Load starts a fresh one (see
	// rearm). The context itself is carried by the callbacks, which hand it to
	// TriggerRefresh.
	ctx context.Context
	// cancel cancels ctx.
	cancel context.CancelFunc
	// stopped is true between Close and the next Load.
	stopped bool
}

// newNacosCtrl creates a controller with its caches ready, so the lazy
// nil-checks are kept out of the hot paths.
func newNacosCtrl() *nacosCtrl {
	ctx, cancel := context.WithCancel(context.Background())
	return &nacosCtrl{
		clients:  map[string]config_client.IConfigClient{},
		listened: map[string]struct{}{},
		ctx:      ctx,
		cancel:   cancel,
	}
}

// rearm restarts the listener generation after a Close, so the first Load of a
// new application instance can install listeners again.
func (c *nacosCtrl) rearm() {
	c.mu.Lock()
	if c.stopped {
		c.ctx, c.cancel = context.WithCancel(context.Background())
		c.stopped = false
	}
	c.mu.Unlock()
}

// Close closes every client — which stops its listeners and polling — and drops
// the caches. It implements provider.Provider. Closing is final only for this
// application instance: the next Load builds fresh clients and re-listens.
func (c *nacosCtrl) Close(ctx context.Context) error {
	c.mu.Lock()
	c.cancel()
	c.stopped = true
	clients := c.clients
	c.clients = map[string]config_client.IConfigClient{}
	c.listened = map[string]struct{}{}
	c.mu.Unlock()

	for _, cli := range clients {
		cli.CloseClient()
	}
	return nil
}

// TriggerRefresh is called by the config listener when a watched data id
// changes. Before the app has started, gs.RefreshProperties returns an
// error and the change is dropped — the initial config load already
// captured the state.
func (c *nacosCtrl) TriggerRefresh(ctx context.Context) {
	// The refresh outcome (status, duration, error) is logged and metered
	// centrally by observability.RefreshConf; this layer only records backend events.
	_ = observability.RefreshConf(ctx, gs.RefreshProperties)
}

// configSource holds the parsed components of a nacos provider source string.
type configSource struct {
	server    string
	dataID    string
	group     string
	namespace string
	username  string
	password  string
	timeoutMs uint64
	format    string
}

// parseSource parses a provider source of the form
// <host>:<port>/<dataId>?group=..&namespace=..&format=..&username=..&password=..&timeout-ms=..
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("nacos://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid nacos source %q", source)
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing nacos server address in %q", source)
	}
	dataID := strings.TrimPrefix(u.Path, "/")
	if dataID == "" {
		return configSource{}, errutil.Explain(nil, "missing data id in %q", source)
	}

	q := u.Query()
	cs := configSource{
		server:    u.Host,
		dataID:    dataID,
		group:     q.Get("group"),
		namespace: q.Get("namespace"),
		username:  q.Get("username"),
		password:  q.Get("password"),
		format:    q.Get("format"),
	}
	if cs.group == "" {
		cs.group = "DEFAULT_GROUP"
	}
	if cs.format == "" {
		if ext := strings.TrimPrefix(filepath.Ext(dataID), "."); ext != "" {
			cs.format = ext
		} else {
			cs.format = "properties"
		}
	}
	cs.timeoutMs = 5000
	if v := q.Get("timeout-ms"); v != "" {
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return configSource{}, errutil.Explain(err, "invalid timeout-ms in %q", source)
		}
		cs.timeoutMs = n
	}
	return cs, nil
}

// clientKey builds a cache key for a client.
func clientKey(cs configSource) string {
	return cs.server + "|" + cs.namespace + "|" + cs.username + "|" + cs.password
}

// sourceFields returns the fields identifying a watched data id. They are
// attached to a context with log.WithFields rather than repeated at each call
// site, and the keys match the ones the change listener stamps so a line and
// the refresh it concerns join on the same names.
func sourceFields(cs configSource) []log.Field {
	return []log.Field{
		log.String("server", cs.server),
		log.String("data_id", cs.dataID),
		log.String("group", cs.group),
	}
}

// clientFor returns a cached client for the source, creating one if necessary.
func (c *nacosCtrl) clientFor(cs configSource) (config_client.IConfigClient, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	host, port, err := netutil.SplitHostPort(cs.server)
	if err != nil {
		return nil, err
	}
	sc := []constant.ServerConfig{*constant.NewServerConfig(host, port)}
	cc := constant.NewClientConfig(
		constant.WithNamespaceId(cs.namespace),
		constant.WithTimeoutMs(cs.timeoutMs),
		constant.WithUsername(cs.username),
		constant.WithPassword(cs.password),
		constant.WithNotLoadCacheAtStart(true),
	)
	cli, err := clients.NewConfigClient(vo.NacosClientParam{ClientConfig: cc, ServerConfigs: sc})
	if err != nil {
		return nil, errutil.Explain(err, "create nacos config client for %s failed", cs.server)
	}
	c.clients[key] = cli
	return cli, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from Nacos, parses it according to the declared format, and installs a
// change listener that triggers an application property refresh.
func (c *nacosCtrl) Load(optional bool, source string) (map[string]string, error) {
	c.rearm()

	// Load is the top of this chain and Provider.Load takes no context, so the
	// one ctx the whole load shares is minted here — the helpers below receive
	// it instead of each minting their own.
	ctx := context.Background()

	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag, err, log.String("source", source), log.Msg("parse nacos source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx, sourceFields(cs)...)

	log.Debug(ctx, starterTag, func() []log.Field {
		return []log.Field{
			log.String("format", cs.format),
			log.Msg("loading nacos config"),
		}
	})

	cli, err := c.clientFor(cs)
	if err != nil {
		log.Error(ctx, starterTag, err, log.Msg("create nacos client failed"))
		return nil, err
	}

	c.registerListener(ctx, cli, cs)

	content, err := cli.GetConfig(vo.ConfigParam{DataId: cs.dataID, Group: cs.group})
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("skip optional config get failed"))
			return nil, nil
		}
		log.Error(ctx, starterTag, err, log.Msg("get nacos config failed"))
		return nil, errutil.Explain(err, "get nacos config %s/%s failed", cs.group, cs.dataID)
	}
	if content == "" {
		if optional {
			log.Warn(ctx, starterTag,
				log.Msg("skip optional config is empty"))
			return nil, nil
		}
		err := errutil.Explain(nil, "nacos config %s/%s is empty", cs.group, cs.dataID)
		log.Error(ctx, starterTag, err, log.Msg("nacos config is empty"))
		return nil, err
	}

	m, err := reader.Read(cs.format, []byte(content))
	if err != nil {
		log.Error(ctx, starterTag, err, log.String("format", cs.format), log.Msg("parse nacos config failed"))
		return nil, errutil.Explain(err, "parse nacos config %s/%s as %s failed", cs.group, cs.dataID, cs.format)
	}

	log.Info(ctx, starterTag,
		log.Int("keys", len(m)),
		log.Msg("load nacos config success"))
	return flatten.Flatten(m), nil
}

// registerListener installs a Nacos change listener for the given data id,
// deduplicated across repeated Load calls.
func (c *nacosCtrl) registerListener(ctx context.Context, cli config_client.IConfigClient, cs configSource) {
	lk := clientKey(cs) + "|" + cs.group + "|" + cs.dataID

	c.mu.Lock()
	if _, ok := c.listened[lk]; ok {
		c.mu.Unlock()
		return
	}
	// A Close may have run between this Load fetched its client and got here:
	// the client is already closed and the caches dropped. Marking listened
	// now would make the next generation's Load skip registration on its
	// fresh client, silently killing hot-reload — bail out instead.
	if c.stopped || c.clients[clientKey(cs)] != cli {
		c.mu.Unlock()
		return
	}
	c.listened[lk] = struct{}{}
	// The listener outlives this load, so it runs on the controller's
	// generation context, not on the load's. It carries the same fields, which
	// is what lets the trigger below name the data id without listing it again.
	wctx := log.WithFields(c.ctx, sourceFields(cs)...)
	c.mu.Unlock()

	err := cli.ListenConfig(vo.ConfigParam{
		DataId: cs.dataID,
		Group:  cs.group,
		OnChange: func(namespace, group, dataId, data string) {
			// Stamp the trigger with what this round adds to the identity the
			// context already carries (server/data_id/group, stamped by
			// registerListener) so the refresh records logged and metered by
			// observability.RefreshConf carry what this round is about. Nacos
			// hands the listener no change revision, so the namespace plus the
			// new content's length are all the identity left to add.
			c.TriggerRefresh(log.WithFields(wctx,
				log.String("source", "nacos"),
				log.String("namespace", namespace),
				log.Int("data_len", len(data)),
			))
		},
	})
	if err != nil {
		// Un-mark so the next Load retries, and surface the failure: a
		// listener that never installs means this data id silently stops
		// hot-reloading.
		log.Error(ctx, starterTag, err, log.Msg("nacos listen config failed; it will not hot-reload until the listen succeeds"))
		c.mu.Lock()
		delete(c.listened, lk)
		c.mu.Unlock()
	}
}
