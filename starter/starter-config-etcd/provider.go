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

package StarterConfigEtcd

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader/json"
	"go-spring.org/spring/conf/reader/prop"
	"go-spring.org/spring/conf/reader/toml"
	"go-spring.org/spring/conf/reader/yaml"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	// Register "etcd" as a remote configuration provider so that a
	// spring.app.imports entry such as
	//
	//	optional:etcd:127.0.0.1:2379/my-key?format=properties
	//
	// pulls configuration from an etcd cluster at startup and on every
	// RefreshProperties call.
	conf.RegisterProvider("etcd", loadEtcdConfig)
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

// configSource holds the parsed components of an etcd provider source string.
type configSource struct {
	endpoint    string // host:port of the etcd server
	key         string // the etcd key holding the configuration content
	username    string
	password    string
	dialTimeout time.Duration
	format      string // content format: properties/yaml/toml/json
}

// parseSource parses a provider source of the form
//
//	<host>:<port>/<key>?format=..&username=..&password=..&dial-timeout=..
//
// The leading "etcd:" prefix has already been stripped by conf/provider.Load.
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
		// Fall back to the key extension, otherwise properties.
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

// clientCache reuses one etcd client per (endpoint, credentials) tuple.
// loadEtcdConfig runs at startup and again on every RefreshProperties call,
// so caching avoids leaking a client and its background goroutines on each
// refresh.
var (
	clientMu    sync.Mutex
	clientCache = map[string]*clientv3.Client{}
	// listened tracks (client-key, etcd-key) tuples that already have a change
	// watcher, so repeated Load calls do not register duplicates.
	listened = map[string]struct{}{}
)

// clientFor returns a cached etcd client for the source, creating one if
// necessary.
func clientFor(cs configSource) (*clientv3.Client, string, error) {
	key := cs.endpoint + "|" + cs.username + "|" + cs.password

	clientMu.Lock()
	defer clientMu.Unlock()

	if cli, ok := clientCache[key]; ok {
		return cli, key, nil
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{cs.endpoint},
		Username:    cs.username,
		Password:    cs.password,
		DialTimeout: cs.dialTimeout,
	})
	if err != nil {
		return nil, "", errutil.Explain(err, "create etcd client for %s failed", cs.endpoint)
	}
	clientCache[key] = cli
	return cli, key, nil
}

// loadEtcdConfig implements conf/provider.Provider. It fetches configuration
// content from etcd, parses it according to the declared format, and installs
// a change watcher that triggers an application property refresh.
func loadEtcdConfig(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}

	cli, ckey, err := clientFor(cs)
	if err != nil {
		return nil, err
	}

	// Register the change watcher before reading so that hot-reload works even
	// when the key does not exist yet: a later put will trigger a refresh that
	// re-runs this provider and picks up the new value.
	registerWatcher(cli, ckey, cs)

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
// deduplicated across repeated Load calls. On change it triggers a full
// application property refresh, which re-runs this provider and propagates new
// values to bound gs.Dync fields.
func registerWatcher(cli *clientv3.Client, ckey string, cs configSource) {
	lk := ckey + "|" + cs.key

	clientMu.Lock()
	if _, ok := listened[lk]; ok {
		clientMu.Unlock()
		return
	}
	listened[lk] = struct{}{}
	clientMu.Unlock()

	// Watch registration is synchronous and best-effort: the watch channel is
	// created here and consumed in a background goroutine. Any error surfaces
	// as a closed channel with wr.Err() != nil; failing only loses hot-reload
	// for this key and does not block startup.
	ch := cli.Watch(context.Background(), cs.key)
	go func() {
		for wr := range ch {
			if len(wr.Events) > 0 {
				triggerRefresh()
			}
		}
	}()
}
