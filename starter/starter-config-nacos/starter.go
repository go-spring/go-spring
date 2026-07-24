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
// config provider that can be consumed via spring.app.imports, together with
// the bridge that wires remote config changes into the application-wide
// property refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery
// (Nacos naming) is a separate concern and is not provided here.
package StarterConfigNacos

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register the nacos controller as both a root bean (so the IoC container
	// injects its PropertiesRefresher via autowire) and the "nacos" config
	// provider (so Load calls go through its method). Before wiring,
	// TriggerRefresh is a harmless no-op — the startup load already captured
	// the initial config.
	gs.Provide(nacosController).Export(gs.As[gs.Rooter]())

	// Register "nacos" as a remote configuration provider. The provider is
	// the global controller's Load method, so the same object that holds the
	// PropertiesRefresher (injected via autowire) also serves config loads.
	conf.RegisterProvider("nacos", nacosController.Load)
}

// nacosController is the global singleton. It is ONLY referenced in init
// functions. All other code operates on the
// receiver without touching this global.
var nacosController = &nacosCtrl{}

// nacosCtrl is the single object that owns the full lifecycle of nacos
// configuration: loading configs, listening for changes, and triggering
// property refresh.
type nacosCtrl struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu       sync.Mutex
	clients  map[string]config_client.IConfigClient
	listened map[string]struct{}
}

// TriggerRefresh is called by the config listener when a watched data id
// changes. Before the IoC container wires the controller, this is a no-op —
// the initial config load already captured the state.
func (c *nacosCtrl) TriggerRefresh() {
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}

// contentReader parses raw configuration bytes into a nested map.
type contentReader func(b []byte) (map[string]any, error)

var contentReaders = map[string]contentReader{
	"properties": prop.Read,
	"props":      prop.Read,
	"yaml":       yaml.Read,
	"yml":        yaml.Read,
	"toml":       toml.Read,
	"tml":        toml.Read,
	"json":       json.Read,
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
		return nil, err
	}

	cli, err := c.clientFor(cs)
	if err != nil {
		return nil, err
	}

	c.registerListener(cli, cs)

	content, err := cli.GetConfig(vo.ConfigParam{DataId: cs.dataID, Group: cs.group})
	if err != nil {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(err, "get nacos config %s/%s failed", cs.group, cs.dataID)
	}
	if content == "" {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "nacos config %s/%s is empty", cs.group, cs.dataID)
	}

	r, ok := contentReaders[cs.format]
	if !ok {
		return nil, errutil.Explain(nil, "unsupported nacos config format %q", cs.format)
	}
	m, err := r([]byte(content))
	if err != nil {
		return nil, errutil.Explain(err, "parse nacos config %s/%s as %s failed", cs.group, cs.dataID, cs.format)
	}
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
