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
// config provider that can be consumed via spring.config.import, together with
// the bridge that wires remote KV changes into the application-wide property
// refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery via
// Consul catalog is a separate concern and is not provided here.
package StarterConfigConsul

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register "consul" as a remote configuration provider. The provider is
	// the global controller's Load method. Watch-triggered refreshes go
	// through the gs.RefreshProperties package-level facade, so the
	// controller needs no bean wiring at all.
	conf.RegisterProvider("consul", newConsulCtrl().Load)
}

var starterTag = log.RegisterAppTag("config_consul", "")

// consulCtrl is the single object that owns the full lifecycle of consul
// configuration: loading KV entries, watching for changes, and triggering
// property refresh.
type consulCtrl struct {
	mu       sync.Mutex
	clients  map[string]kvAPI
	listened map[string]struct{}
}

// kvAPI is the slice of the Consul API surface this starter consumes. It
// exists so tests can fake the KV backend without a live Consul agent.
type kvAPI interface {
	Get(key string, q *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error)
}

// newConsulCtrl creates a controller with its caches ready, so the lazy
// nil-checks are kept out of the hot paths.
func newConsulCtrl() *consulCtrl {
	return &consulCtrl{
		clients:  map[string]kvAPI{},
		listened: map[string]struct{}{},
	}
}

// TriggerRefresh is called by the watcher goroutines when a watched KV entry
// changes. Before the app has started, gs.RefreshProperties returns an
// error and the change is dropped — the initial config load already captured
// the state.
func (c *consulCtrl) TriggerRefresh(ctx context.Context) {
	// The refresh outcome (status, duration, error) is logged and metered
	// centrally by observability.RefreshConf; this layer only records backend events.
	_ = observability.RefreshConf(ctx, gs.RefreshProperties)
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

// clientKey builds a cache key for a client from the connection tuple, matching
// the etcd/nacos config-providers so the (client, key) dedup in registerWatch
// composes consistently across the family.
func clientKey(cs configSource) string {
	return cs.address + "|" + cs.scheme + "|" + cs.token + "|" + cs.datacenter
}

// clientFor returns a cached KV handle for the source, creating one if
// necessary.
func (c *consulCtrl) clientFor(cs configSource) (kvAPI, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

	if cli, ok := c.clients[key]; ok {
		return cli, nil
	}

	raw, err := api.NewClient(&api.Config{
		Address:    cs.address,
		Scheme:     cs.scheme,
		Token:      cs.token,
		Datacenter: cs.datacenter,
	})
	if err != nil {
		return nil, errutil.Explain(err, "create consul client for %s failed", cs.address)
	}
	kv := raw.KV()
	c.clients[key] = kv
	return kv, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from a Consul KV path, parses it according to the declared format, and
// installs a blocking-query watcher that triggers an application property
// refresh on change.
func (c *consulCtrl) Load(optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "parse source %q failed: %v", source, err)
		return nil, err
	}

	log.Debugf(context.Background(), starterTag, "loading config from address=%s kvPath=%s format=%s", cs.address, cs.kvPath, cs.format)

	cli, err := c.clientFor(cs)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "create client for address=%s failed: %v", cs.address, err)
		return nil, err
	}

	c.registerWatch(cli, cs, optional)

	pair, _, err := cli.Get(cs.kvPath, &api.QueryOptions{Datacenter: cs.datacenter})
	if err != nil {
		if optional {
			log.Warnf(context.Background(), starterTag, "optional config get kv %s failed (skipped): %v", cs.kvPath, err)
			return nil, nil
		}
		log.Errorf(context.Background(), starterTag, "get consul kv %s failed: %v", cs.kvPath, err)
		return nil, errutil.Explain(err, "get consul kv %s failed", cs.kvPath)
	}
	if pair == nil {
		if optional {
			log.Warnf(context.Background(), starterTag, "optional config kv %s not found (skipped)", cs.kvPath)
			return nil, nil
		}
		log.Errorf(context.Background(), starterTag, "consul kv %s not found", cs.kvPath)
		return nil, errutil.Explain(nil, "consul kv %s not found", cs.kvPath)
	}
	if len(pair.Value) == 0 {
		if optional {
			log.Warnf(context.Background(), starterTag, "optional config kv %s is empty (skipped)", cs.kvPath)
			return nil, nil
		}
		log.Errorf(context.Background(), starterTag, "consul kv %s is empty", cs.kvPath)
		return nil, errutil.Explain(nil, "consul kv %s is empty", cs.kvPath)
	}

	m, err := reader.Read(cs.format, pair.Value)
	if err != nil {
		log.Errorf(context.Background(), starterTag, "parse consul kv %s as %s failed: %v", cs.kvPath, cs.format, err)
		return nil, errutil.Explain(err, "parse consul kv %s as %s failed", cs.kvPath, cs.format)
	}

	log.Infof(context.Background(), starterTag, "loaded consul config from kvPath=%s keys=%d", cs.kvPath, len(m))
	return flatten.Flatten(m), nil
}

// registerWatch spawns a background goroutine that runs a Consul blocking
// query against the given KV path. Deduplicated across repeated Load calls.
// optional records whether the import declared the path optional: deleting an
// optional path is an expected transition (its properties simply disappear),
// while deleting a required one leaves the last snapshot in place, which the
// watcher surfaces as a warning.
func (c *consulCtrl) registerWatch(cli kvAPI, cs configSource, optional bool) {
	lk := clientKey(cs) + "|" + cs.kvPath

	c.mu.Lock()
	if _, ok := c.listened[lk]; ok {
		c.mu.Unlock()
		return
	}
	c.listened[lk] = struct{}{}
	c.mu.Unlock()

	go c.watchLoop(cli, cs, optional)
}

// watchLoop runs the blocking-query loop for a single KV path. Errors retry
// silently-but-visibly: the first failure logs a warning, subsequent ones only
// debug (the loop keeps retrying every 2s, so per-failure warnings would flood),
// and recovery logs once at info.
func (c *consulCtrl) watchLoop(cli kvAPI, cs configSource, optional bool) {
	var lastIndex uint64
	initialized := false
	failing := false
	for {
		pair, meta, err := cli.Get(cs.kvPath, &api.QueryOptions{
			Datacenter: cs.datacenter,
			WaitIndex:  lastIndex,
			WaitTime:   5 * time.Minute,
		})
		if err == nil && meta == nil {
			err = errutil.Explain(nil, "nil query meta")
		}
		if err != nil {
			if !failing {
				log.Warnf(context.Background(), starterTag,
					"consul watch on %s failing, retrying every 2s (changes are missed until it recovers): %v", cs.kvPath, err)
				failing = true
			} else {
				log.Debugf(context.Background(), starterTag,
					"consul watch on %s still failing: %v", cs.kvPath, err)
			}
			time.Sleep(2 * time.Second)
			continue
		}
		if failing {
			log.Infof(context.Background(), starterTag,
				"consul watch on %s recovered", cs.kvPath)
			failing = false
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
			if pair == nil && !optional {
				log.Warnf(context.Background(), starterTag,
					"consul kv %s deleted; stale snapshot retained until the key is restored", cs.kvPath)
			}
			c.TriggerRefresh(context.Background())
		}
	}
}
