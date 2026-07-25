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

// Package StarterConfigConsul integrates Consul KV as a remote configuration
// center for Go-Spring. Blank-importing this package registers a "consul"
// config provider that can be consumed via spring.app.imports, together with
// the bridge that wires remote KV changes into the application-wide property
// refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery via
// Consul catalog is a separate concern and is not provided here.
package StarterConfigConsul

import (
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
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
	// Register the consul controller as both a root bean (so the IoC container
	// injects its PropertiesRefresher via autowire) and the "consul" config
	// provider (so Load calls go through its method). Before wiring,
	// TriggerRefresh is a harmless no-op — the startup load already captured
	// the initial config.
	gs.Provide(consulController).Export(gs.As[gs.Rooter]())

	// Register "consul" as a remote configuration provider. The provider is
	// the global controller's Load method, so the same object that holds the
	// PropertiesRefresher (injected via autowire) also serves config loads.
	conf.RegisterProvider("consul", consulController.Load)
}

// consulController is the global singleton. It is ONLY referenced in init
// functions. All other code operates on the
// receiver without touching this global.
var consulController = &consulCtrl{}

// consulCtrl is the single object that owns the full lifecycle of consul
// configuration: loading KV entries, watching for changes, and triggering
// property refresh.
type consulCtrl struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu       sync.Mutex
	clients  map[string]*api.Client
	listened map[string]struct{}
}

// TriggerRefresh is called by the watcher goroutines when a watched KV entry
// changes. Before the IoC container wires the controller, this is a no-op —
// the initial config load already captured the state.
func (c *consulCtrl) TriggerRefresh() {
	if c.Refresher != nil {
		_ = c.Refresher.RefreshProperties()
	}
}

// contentReader parses raw configuration bytes into a nested map based on the
// declared format.
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

// configSource holds the parsed components of a consul provider source string.
type configSource struct {
	address    string
	scheme     string
	kvPath     string
	token      string
	datacenter string
	format     string
}

// parseSource parses a provider source of the form
// <host>:<port>/<kv-path>?format=..&token=..&datacenter=..&scheme=..
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
		if ext := strings.TrimPrefix(filepath.Ext(kvPath), "."); ext != "" {
			cs.format = ext
		} else {
			cs.format = "properties"
		}
	}
	return cs, nil
}

// clientFor returns a cached client for the source, creating one if necessary.
func (c *consulCtrl) clientFor(cs configSource) (*api.Client, error) {
	key := cs.address + "|" + cs.scheme + "|" + cs.token + "|" + cs.datacenter

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.clients == nil {
		c.clients = map[string]*api.Client{}
	}
	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	cli, err := api.NewClient(&api.Config{
		Address:    cs.address,
		Scheme:     cs.scheme,
		Token:      cs.token,
		Datacenter: cs.datacenter,
	})
	if err != nil {
		return nil, errutil.Explain(err, "create consul client for %s failed", cs.address)
	}
	c.clients[key] = cli
	return cli, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from a Consul KV path, parses it according to the declared format, and
// installs a blocking-query watcher that triggers an application property
// refresh on change.
func (c *consulCtrl) Load(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	cli, err := c.clientFor(cs)
	if err != nil {
		return nil, err
	}

	c.registerWatch(cli, cs)

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
// query against the given KV path. Deduplicated across repeated Load calls.
func (c *consulCtrl) registerWatch(cli *api.Client, cs configSource) {
	lk := cs.address + "|" + cs.scheme + "|" + cs.token + "|" + cs.datacenter + "|" + cs.kvPath

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

	go c.watchLoop(cli, cs)
}

// watchLoop runs the blocking-query loop for a single KV path.
func (c *consulCtrl) watchLoop(cli *api.Client, cs configSource) {
	var lastIndex uint64
	initialized := false
	for {
		pair, meta, err := cli.KV().Get(cs.kvPath, &api.QueryOptions{
			Datacenter: cs.datacenter,
			WaitIndex:  lastIndex,
			WaitTime:   5 * time.Minute,
		})
		if err != nil {
			_ = pair
			time.Sleep(2 * time.Second)
			continue
		}
		if meta == nil {
			time.Sleep(2 * time.Second)
			continue
		}
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
			c.TriggerRefresh()
		}
	}
}
