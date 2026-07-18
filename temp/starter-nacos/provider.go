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

package StarterNacos

import (
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register "nacos" as a remote configuration provider so that a
	// spring.app.imports entry such as
	//
	//	optional:nacos:127.0.0.1:8848/my-data-id?group=DEFAULT_GROUP&format=properties
	//
	// pulls configuration from a Nacos config server at startup and on
	// every RefreshProperties call.
	conf.RegisterProvider("nacos", loadNacosConfig)
}

// contentReader parses raw configuration bytes into a nested map based on the
// declared format. It mirrors the readers registered in conf/reader but is
// keyed by format name rather than file extension, since remote content has no
// file name.
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

// refreshHook holds the callback used to reload application properties when a
// watched remote configuration changes. It is populated by the refresh bridge
// bean during container wiring (see starter.go). A remote change that arrives
// before the bridge is wired is safely ignored; the value is picked up on the
// next refresh.
var refreshHook atomic.Pointer[func() error]

// setRefreshHook installs the callback that reloads application properties.
func setRefreshHook(fn func() error) {
	refreshHook.Store(&fn)
}

// triggerRefresh invokes the installed refresh callback, if any.
func triggerRefresh() {
	if p := refreshHook.Load(); p != nil {
		_ = (*p)()
	}
}

// configSource holds the parsed components of a nacos provider source string.
type configSource struct {
	server    string // host:port of the Nacos server
	dataID    string // config data id
	group     string // config group, defaults to DEFAULT_GROUP
	namespace string // namespace id, defaults to public
	username  string
	password  string
	timeoutMs uint64
	format    string // content format: properties/yaml/toml/json
}

// parseSource parses a provider source of the form
//
//	<host>:<port>/<dataId>?group=..&namespace=..&format=..&username=..&password=..&timeout-ms=..
//
// The leading "nacos:" prefix has already been stripped by conf/provider.Load.
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
		// Fall back to the data id extension, otherwise properties.
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

// clientCache reuses one config client per (server, namespace, credentials)
// tuple. loadNacosConfig runs at startup and again on every RefreshProperties
// call, so caching avoids leaking a client and its background goroutines on
// each refresh.
var (
	clientMu    sync.Mutex
	clientCache = map[string]config_client.IConfigClient{}
	// listened tracks (client-key, group, dataId) tuples that already have a
	// change listener, so repeated Load calls do not register duplicates.
	listened = map[string]struct{}{}
)

// clientFor returns a cached config client for the source, creating one if
// necessary.
func clientFor(cs configSource) (config_client.IConfigClient, string, error) {
	key := cs.server + "|" + cs.namespace + "|" + cs.username + "|" + cs.password

	clientMu.Lock()
	defer clientMu.Unlock()

	if cli, ok := clientCache[key]; ok {
		return cli, key, nil
	}

	host, port, err := splitHostPort(cs.server)
	if err != nil {
		return nil, "", err
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
		return nil, "", errutil.Explain(err, "create nacos config client for %s failed", cs.server)
	}
	clientCache[key] = cli
	return cli, key, nil
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

// loadNacosConfig implements conf/provider.Provider. It fetches configuration
// content from Nacos, parses it according to the declared format, and installs
// a change listener that triggers an application property refresh.
func loadNacosConfig(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	cli, key, err := clientFor(cs)
	if err != nil {
		return nil, err
	}

	// Register the change listener before reading so that hot-reload works even
	// when the data id does not exist yet: a later publish will trigger a
	// refresh that re-runs this provider and picks up the new value.
	registerListener(cli, key, cs)

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
// deduplicated across repeated Load calls. On change it triggers a full
// application property refresh, which re-runs this provider and propagates new
// values to bound gs.Dync fields.
func registerListener(cli config_client.IConfigClient, key string, cs configSource) {
	lk := key + "|" + cs.group + "|" + cs.dataID

	clientMu.Lock()
	if _, ok := listened[lk]; ok {
		clientMu.Unlock()
		return
	}
	listened[lk] = struct{}{}
	clientMu.Unlock()

	err := cli.ListenConfig(vo.ConfigParam{
		DataId: cs.dataID,
		Group:  cs.group,
		OnChange: func(namespace, group, dataId, data string) {
			triggerRefresh()
		},
	})
	if err != nil {
		// Listener registration is best-effort: startup still succeeds with a
		// static snapshot, only losing hot-reload for this data id.
		clientMu.Lock()
		delete(listened, lk)
		clientMu.Unlock()
	}
}
