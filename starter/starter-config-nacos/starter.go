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
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register "nacos" as a remote configuration provider. The provider is
	// the controller's Load method — the controller itself lives in this
	// closure only (no package-level variable), so its state is reachable
	// solely through the registered provider. Watch-triggered refreshes go
	// through the gs.RefreshProperties package-level facade, so the
	// controller needs no bean wiring at all.
	conf.RegisterProvider("nacos", (&nacosCtrl{}).Load)
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
}

// TriggerRefresh is called by the config listener when a watched data id
// changes. Before the app has started, gs.RefreshProperties returns an
// error and the change is dropped — the initial config load already
// captured the state.
func (c *nacosCtrl) TriggerRefresh() {
	_ = gs.RefreshProperties()
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
		if err != nil {
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

// clientFor returns a cached client for the source, creating one if necessary.
func (c *nacosCtrl) clientFor(cs configSource) (config_client.IConfigClient, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.clients == nil {
		c.clients = map[string]config_client.IConfigClient{}
	}
	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	host, port, err := splitHostPort(cs.server)
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

// splitHostPort splits "host:port" into its parts.
func splitHostPort(server string) (string, uint64, error) {
	host, portStr, ok := strings.Cut(server, ":")
	if !ok || host == "" || portStr == "" {
		return "", 0, errutil.Explain(nil, "nacos server address must be host:port, got %q", server)
	}
	port, err := strconv.ParseUint(portStr, 10, 64)
	if err != nil {
		return "", 0, errutil.Explain(err, "invalid nacos server port in %q", server)
	}
	return host, port, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from Nacos, parses it according to the declared format, and installs a
// change listener that triggers an application property refresh.
func (c *nacosCtrl) Load(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "parse source %q failed: %v", source, err)
		return nil, err
	}

	log.Debugf(context.Background(), starterTag, "loading nacos config from server=%s group=%s dataId=%s format=%s", cs.server, cs.group, cs.dataID, cs.format)

	cli, err := c.clientFor(cs)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create nacos client for server=%s failed: %v", cs.server, err)
		return nil, err
	}

	c.registerListener(cli, cs)

	content, err := cli.GetConfig(vo.ConfigParam{DataId: cs.dataID, Group: cs.group})
	if err != nil {
		if optional {
			log.Warnf(context.Background(), starterTag, "optional config get %s/%s failed (skipped): %v", cs.group, cs.dataID, err)
			return nil, nil
		}
		log.Errorf(context.Background(), starterTag, "get nacos config %s/%s failed: %v", cs.group, cs.dataID, err)
		return nil, errutil.Explain(err, "get nacos config %s/%s failed", cs.group, cs.dataID)
	}
	if content == "" {
		if optional {
			log.Warnf(context.Background(), starterTag, "optional config %s/%s is empty (skipped)", cs.group, cs.dataID)
			return nil, nil
		}
		log.Errorf(context.Background(), starterTag, "nacos config %s/%s is empty", cs.group, cs.dataID)
		return nil, errutil.Explain(nil, "nacos config %s/%s is empty", cs.group, cs.dataID)
	}

	m, err := reader.Read(cs.format, []byte(content))
	if err != nil {
		log.Errorf(context.Background(), starterTag, "parse nacos config %s/%s as %s failed: %v", cs.group, cs.dataID, cs.format, err)
		return nil, errutil.Explain(err, "parse nacos config %s/%s as %s failed", cs.group, cs.dataID, cs.format)
	}

	log.Infof(context.Background(), starterTag, "loaded nacos config from %s/%s keys=%d", cs.group, cs.dataID, len(m))
	return flatten.Flatten(m), nil
}

// registerListener installs a Nacos change listener for the given data id,
// deduplicated across repeated Load calls.
func (c *nacosCtrl) registerListener(cli config_client.IConfigClient, cs configSource) {
	lk := clientKey(cs) + "|" + cs.group + "|" + cs.dataID

	c.mu.Lock()
	if c.listened == nil {
		c.listened = map[string]struct{}{}
	}
	if _, ok := c.listened[lk]; ok {
		c.mu.Unlock()
		return
	}
	c.listened[lk] = struct{}{}
	c.mu.Unlock()

	err := cli.ListenConfig(vo.ConfigParam{
		DataId: cs.dataID,
		Group:  cs.group,
		OnChange: func(namespace, group, dataId, data string) {
			c.TriggerRefresh()
		},
	})
	if err != nil {
		c.mu.Lock()
		delete(c.listened, lk)
		c.mu.Unlock()
	}
}
