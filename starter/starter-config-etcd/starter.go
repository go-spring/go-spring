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

// Package StarterConfigEtcd integrates etcd as a remote configuration
// center for Go-Spring. Blank-importing this package registers an "etcd"
// config provider that can be consumed via spring.app.imports, together with
// the bridge that wires remote config changes into the application-wide
// property refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery
// (etcd naming) is a separate concern and is not provided here.
package StarterConfigEtcd

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	// Register the etcd controller as both a root bean (so the IoC container
	// injects its PropertiesRefresher via autowire) and the "etcd" config
	// provider (so Load calls go through its method). Before wiring,
	// TriggerRefresh is a harmless no-op — the startup load already captured
	// the initial config.
	gs.Provide(etcdController).Export(gs.As[gs.Rooter]())

	// Register "etcd" as a remote configuration provider. The provider is
	// the global controller's Load method, so the same object that holds the
	// PropertiesRefresher (injected via autowire) also serves config loads.
	conf.RegisterProvider("etcd", etcdController.Load)
}

// etcdController is the global singleton. It is ONLY referenced in init
// functions. All other code operates on the
// receiver without touching this global.
var etcdController = &etcdCtrl{}

// etcdCtrl is the single object that owns the full lifecycle of etcd
// configuration: loading keys, watching for changes, and triggering
// property refresh.
type etcdCtrl struct {
	Refresher *gs.PropertiesRefresher `autowire:""`

	mu       sync.Mutex
	clients  map[string]*clientv3.Client
	listened map[string]struct{}
}

// TriggerRefresh is called by the watch goroutines when a watched key
// changes. Before the IoC container wires the controller, this is a no-op —
// the initial config load already captured the state.
func (c *etcdCtrl) TriggerRefresh() {
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

// configSource holds the parsed components of an etcd provider source string.
type configSource struct {
	endpoint    string
	key         string
	username    string
	password    string
	dialTimeout time.Duration
	format      string
}

// parseSource parses a provider source of the form
// <host>:<port>/<key>?format=..&username=..&password=..&dial-timeout=..
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("etcd://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid etcd source %q", source)
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing etcd server address in %q", source)
	}
	key := strings.TrimPrefix(u.Path, "/")
	if key == "" {
		return configSource{}, errutil.Explain(nil, "missing etcd key in %q", source)
	}

	q := u.Query()
	cs := configSource{
		endpoint: u.Host,
		key:      key,
		username: q.Get("username"),
		password: q.Get("password"),
		format:   q.Get("format"),
	}
	if cs.format == "" {
		if ext := strings.TrimPrefix(filepath.Ext(key), "."); ext != "" {
			cs.format = ext
		} else {
			cs.format = "properties"
		}
	}
	cs.dialTimeout = 5 * time.Second
	if v := q.Get("dial-timeout"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return configSource{}, errutil.Explain(err, "invalid dial-timeout in %q", source)
		}
		cs.dialTimeout = d
	}
	return cs, nil
}

// clientKey builds a cache key for a client.
func clientKey(cs configSource) string {
	return cs.endpoint + "|" + cs.username + "|" + cs.password
}

// clientFor returns a cached client for the source, creating one if necessary.
func (c *etcdCtrl) clientFor(cs configSource) (*clientv3.Client, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.clients == nil {
		c.clients = map[string]*clientv3.Client{}
	}
	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cs.endpoint},
		Username:    cs.username,
		Password:    cs.password,
		DialTimeout: cs.dialTimeout,
	})
	if err != nil {
		return nil, errutil.Explain(err, "create etcd client for %s failed", cs.endpoint)
	}
	c.clients[key] = cli
	return cli, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from etcd, parses it according to the declared format, and installs a
// change watcher that triggers an application property refresh.
func (c *etcdCtrl) Load(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	cli, err := c.clientFor(cs)
	if err != nil {
		return nil, err
	}

	c.registerWatcher(cli, cs)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := cli.Get(ctx, cs.key)
	if err != nil {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(err, "get etcd key %s failed", cs.key)
	}
	if len(resp.Kvs) == 0 {
		if optional {
			return nil, nil
		}
		return nil, errutil.Explain(nil, "etcd key %s is empty", cs.key)
	}

	content := resp.Kvs[0].Value
	r, ok := contentReaders[cs.format]
	if !ok {
		return nil, errutil.Explain(nil, "unsupported etcd config format %q", cs.format)
	}
	m, err := r(content)
	if err != nil {
		return nil, errutil.Explain(err, "parse etcd key %s as %s failed", cs.key, cs.format)
	}
	return flatten.Flatten(m), nil
}

// registerWatcher installs an etcd change watcher for the given key,
// deduplicated across repeated Load calls.
func (c *etcdCtrl) registerWatcher(cli *clientv3.Client, cs configSource) {
	lk := clientKey(cs) + "|" + cs.key

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

	ch := cli.Watch(context.Background(), cs.key)
	go func() {
		for wr := range ch {
			if len(wr.Events) > 0 {
				c.TriggerRefresh()
			}
		}
	}()
}
