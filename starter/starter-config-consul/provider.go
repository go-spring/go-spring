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

package StarterConfigConsul

import (
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register "consul" as a remote configuration provider so that a
	// spring.app.imports entry such as
	//
	//	optional:consul:127.0.0.1:8500/my-kv-path?format=properties
	//
	// pulls configuration from a Consul KV endpoint at startup and on
	// every RefreshProperties call.
	conf.RegisterProvider("consul", loadConsulConfig)
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

// configSource holds the parsed components of a consul provider source string.
type configSource struct {
	address    string // host:port of the Consul agent
	scheme     string // http or https, defaults to http
	kvPath     string // KV path (key), leading '/' already stripped
	token      string // ACL token, empty for anonymous
	datacenter string // datacenter override, empty uses agent default
	format     string // content format: properties/yaml/toml/json
}

// parseSource parses a provider source of the form
//
//	<host>:<port>/<kv-path>?format=..&token=..&datacenter=..&scheme=..
//
// The leading "consul:" prefix has already been stripped by conf/provider.Load.
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("consul://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid consul source %q", source)
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing consul server address in %q", source)
	}
	kvPath := strings.TrimPrefix(u.Path, "/")
	if kvPath == "" {
		return configSource{}, errutil.Explain(nil, "missing kv path in %q", source)
	}

	q := u.Query()
	cs := configSource{
		address:    u.Host,
		scheme:     q.Get("scheme"),
		kvPath:     kvPath,
		token:      q.Get("token"),
		datacenter: q.Get("datacenter"),
		format:     q.Get("format"),
	}
	if cs.scheme == "" {
		cs.scheme = "http"
	}
	if cs.format == "" {
		// Fall back to the KV path extension, otherwise properties.
		if ext := strings.TrimPrefix(filepath.Ext(kvPath), "."); ext != "" {
			cs.format = ext
		} else {
			cs.format = "properties"
		}
	}
	return cs, nil
}

// clientCache reuses one Consul API client per (address, scheme, token,
// datacenter) tuple. loadConsulConfig runs at startup and again on every
// RefreshProperties call, so caching avoids leaking a client on each refresh.
var (
	clientMu    sync.Mutex
	clientCache = map[string]*api.Client{}
	// listened tracks (client-key, kv-path) tuples that already have a
	// blocking-query watcher, so repeated Load calls do not start duplicates.
	listened = map[string]struct{}{}
)

// clientFor returns a cached client for the source, creating one if necessary.
// It also returns the cache key so listener registration can dedupe.
func clientFor(cs configSource) (*api.Client, string, error) {
	key := cs.address + "|" + cs.scheme + "|" + cs.token + "|" + cs.datacenter

	clientMu.Lock()
	defer clientMu.Unlock()

	if cli, ok := clientCache[key]; ok {
		return cli, key, nil
	}

	cli, err := api.NewClient(&api.Config{
		Address:    cs.address,
		Scheme:     cs.scheme,
		Token:      cs.token,
		Datacenter: cs.datacenter,
	})
	if err != nil {
		return nil, "", errutil.Explain(err, "create consul client for %s failed", cs.address)
	}
	clientCache[key] = cli
	return cli, key, nil
}

// loadConsulConfig implements conf/provider.Provider. It fetches configuration
// content from a Consul KV path, parses it according to the declared format,
// and installs a blocking-query watcher that triggers an application property
// refresh on change.
func loadConsulConfig(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	cli, key, err := clientFor(cs)
	if err != nil {
		return nil, err
	}

	// Register the watcher before reading so that hot-reload works even when
	// the key does not exist yet: a later Put will trigger a refresh that
	// re-runs this provider and picks up the new value.
	registerWatch(cli, key, cs)

	pair, _, err := cli.KV().Get(cs.kvPath, &api.QueryOptions{Datacenter: cs.datacenter})
	if err != nil {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(err, "get consul kv %s failed", cs.kvPath)
	}
	if pair == nil {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "consul kv %s not found", cs.kvPath)
	}
	if len(pair.Value) == 0 {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "consul kv %s is empty", cs.kvPath)
	}

	r, ok := contentReaders[cs.format]
	if !ok {
		return nil, errutil.Explain(nil, "unsupported consul config format %q", cs.format)
	}
	m, err := r(pair.Value)
	if err != nil {
		return nil, errutil.Explain(err, "parse consul kv %s as %s failed", cs.kvPath, cs.format)
	}
	return flatten.Flatten(m), nil
}

// registerWatch spawns a background goroutine that runs a Consul blocking
// query against the given KV path. On every observed index bump it invokes
// triggerRefresh, which re-runs this provider and propagates new values to
// bound gs.Dync fields. Deduplicated across repeated Load calls.
func registerWatch(cli *api.Client, key string, cs configSource) {
	lk := key + "|" + cs.kvPath

	clientMu.Lock()
	if _, ok := listened[lk]; ok {
		clientMu.Unlock()
		return
	}
	listened[lk] = struct{}{}
	clientMu.Unlock()

	go watchLoop(cli, cs)
}

// watchLoop runs the blocking-query loop for a single KV path. lastIndex is
// initialized from the first response so startup itself does not trigger a
// spurious refresh; any subsequent index change fires triggerRefresh.
func watchLoop(cli *api.Client, cs configSource) {
	var lastIndex uint64
	initialized := false
	for {
		pair, meta, err := cli.KV().Get(cs.kvPath, &api.QueryOptions{
			Datacenter: cs.datacenter,
			WaitIndex:  lastIndex,
			WaitTime:   5 * time.Minute,
		})
		if err != nil {
			// Transient errors: back off briefly and retry. Keep lastIndex so
			// we resume where we left off once the agent is reachable again.
			_ = pair
			time.Sleep(2 * time.Second)
			continue
		}
		if meta == nil {
			time.Sleep(2 * time.Second)
			continue
		}
		// Consul recommends resetting the wait index if the server returns a
		// value that has gone backwards, which happens on state resets.
		if meta.LastIndex < lastIndex {
			lastIndex = 0
			continue
		}
		if !initialized {
			lastIndex = meta.LastIndex
			initialized = true
			continue
		}
		if meta.LastIndex > lastIndex {
			lastIndex = meta.LastIndex
			triggerRefresh()
		}
	}
}
