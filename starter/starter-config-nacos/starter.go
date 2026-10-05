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
package StarterConfigNacos

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/netutil"
)

var starterTag = log.RegisterAppTag("config", "nacos")

func init() {
	conf.RegisterProvider("nacos", newNacosCtrl())
}

// nacosClient is the slice of the Nacos config client this starter consumes,
// narrowed from the SDK's IConfigClient so the dependency surface is explicit
// and the test fake stays self-contained.
type nacosClient interface {
	GetConfig(param vo.ConfigParam) (string, error)
	ListenConfig(param vo.ConfigParam) error
	CloseClient()
}

// nacosCtrl owns the full lifecycle of nacos configuration: loading configs
// through its client cache, and listening for changes through the embedded
// watchCore. Its clients are bootstrap infrastructure: they exist
// before the container does and therefore deliberately opt out of
// observability and governance wiring — don't "fix" that here; consumers
// that need instrumented clients (e.g. the governance source) build their
// own.
type nacosCtrl struct {
	watch    watchCore
	clientMu sync.Mutex
	clients  map[string]nacosClient
}

func newNacosCtrl() *nacosCtrl {
	return &nacosCtrl{
		clients: map[string]nacosClient{},
		watch:   newWatchCore(),
	}
}

// Close closes every client — which stops its listeners and polling.
func (c *nacosCtrl) Close() {
	c.watch.Close()
	c.clientMu.Lock()
	clients := c.clients
	c.clients = map[string]nacosClient{}
	c.clientMu.Unlock()

	for _, cli := range clients {
		cli.CloseClient()
	}
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
	retryMs   int
	format    string
}

// redactSource masks the password query parameter of a source string so
// the remainder is safe to print. Everything user-visible — parse errors, log
// fields — must carry the redacted form; the raw form never reaches output.
func redactSource(source string) string {
	i := strings.Index(source, "?")
	if i < 0 {
		return source
	}
	q := strings.Split(source[i+1:], "&")
	kept := q[:0]
	for _, kv := range q {
		if strings.HasPrefix(kv, "password=") {
			kv = "password=***"
		}
		kept = append(kept, kv)
	}
	return source[:i+1] + strings.Join(kept, "&")
}

// parseSource parses a provider source of the form
// <host>:<port>/<dataId>?group=..&namespace=..&format=..&username=..&password=..&timeout-ms=..&retry-ms=..
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("nacos://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid nacos source %q", redactSource(source))
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing nacos server address in %q", redactSource(source))
	}
	dataID := strings.TrimPrefix(u.Path, "/")
	if dataID == "" {
		return configSource{}, errutil.Explain(nil, "missing data id in %q", redactSource(source))
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
			return configSource{}, errutil.Explain(err, "invalid timeout-ms in %q", redactSource(source))
		}
		cs.timeoutMs = n
	}
	cs.retryMs = 2000
	if v := q.Get("retry-ms"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return configSource{}, errutil.Explain(nil, "invalid retry-ms in %q", redactSource(source))
		}
		cs.retryMs = n
	}
	return cs, nil
}

// clientKey builds a cache key for a client.
func clientKey(cs configSource) string {
	return cs.server + "|" + cs.namespace + "|" + cs.username + "|" + cs.password
}

// clientFor returns a cached client for the source, creating one if necessary.
func (c *nacosCtrl) clientFor(ctx context.Context, cs configSource) (nacosClient, error) {
	key := clientKey(cs)

	c.clientMu.Lock()
	defer c.clientMu.Unlock()

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
	log.Infof(ctx, starterTag, "create nacos client success")
	c.clients[key] = cli
	return cli, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from Nacos, parses it according to the declared format, and installs a
// change listener that triggers an application property refresh.
func (c *nacosCtrl) Load(ctx context.Context, optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag, err,
			log.String("source", redactSource(source)),
			log.Msg("parse nacos source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx,
		log.String("server", cs.server),
		log.String("data_id", cs.dataID),
		log.String("format", cs.format),
		log.String("group", cs.group))

	log.Debug(ctx, starterTag, func() []log.Field {
		return []log.Field{
			log.Msg("loading nacos config"),
		}
	})

	cli, err := c.clientFor(ctx, cs)
	if err != nil {
		log.Errorf(ctx, starterTag, err, "create nacos client failed")
		return nil, err
	}

	c.watch.registerWatch(cli, cs)
	return loadFromClient(ctx, cli, cs, optional)
}

// loadFromClient reads the data id once, applies the optional/empty rules, and
// parses it into flattened properties. The caller installs the listener before
// calling it, so a change landing right after the read is not missed.
func loadFromClient(ctx context.Context, cli nacosClient, cs configSource, optional bool) (map[string]string, error) {
	content, err := cli.GetConfig(vo.ConfigParam{DataId: cs.dataID, Group: cs.group})
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("skip optional config get failed"))
			return nil, nil
		}
		log.Errorf(ctx, starterTag, err, "get nacos config failed")
		return nil, errutil.Explain(err, "get nacos config %s/%s failed", cs.group, cs.dataID)
	}
	if content == "" {
		if optional {
			log.Warnf(ctx, starterTag, "skip optional config is empty")
			return nil, nil
		}
		err := errutil.Explain(nil, "nacos config %s/%s is empty", cs.group, cs.dataID)
		log.Errorf(ctx, starterTag, err, "nacos config is empty")
		return nil, err
	}

	m, err := reader.Read(cs.format, []byte(content))
	if err != nil {
		log.Error(ctx, starterTag, err, log.String("format", cs.format), log.Msg("parse nacos config failed"))
		return nil, errutil.Explain(err, "parse nacos config %s/%s as %s failed", cs.group, cs.dataID, cs.format)
	}

	log.Infof(ctx, starterTag, "load nacos config success")
	return flatten.Flatten(m), nil
}
