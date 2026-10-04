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
	// Register "consul" as a remote configuration provider. The controller
	// itself is registered (not just its Load method): the runtime holds the
	// registered provider so it can stop the watchers at shutdown (see
	// provider.Provider). Watch-triggered refreshes go through the
	// gs.RefreshProperties package-level facade, so the controller needs no
	// bean wiring at all.
	conf.RegisterProvider("consul", newConsulCtrl())
}

var starterTag = log.RegisterAppTag("config_consul", "")

// consulCtrl is the single object that owns the full lifecycle of consul
// configuration: loading KV entries, watching for changes, and triggering
// property refresh.
type consulCtrl struct {
	mu       sync.Mutex
	clients  map[string]kvAPI
	listened map[string]struct{}

	// ctx is the watch generation: Close cancels it, which aborts the blocking
	// queries and stops every watch goroutine; the next Load starts a fresh one
	// (see rearm).
	ctx context.Context
	// cancel cancels ctx.
	cancel context.CancelFunc
	// stopped is true between Close and the next Load.
	stopped bool
}

// rearm restarts the watch generation after a Close, so the first Load of a new
// application instance can install watchers again.
func (c *consulCtrl) rearm() {
	c.mu.Lock()
	if c.stopped {
		c.ctx, c.cancel = context.WithCancel(context.Background())
		c.stopped = false
	}
	c.mu.Unlock()
}

// Close stops every watch goroutine and clears the dedup set, so the next Load
// re-watches. It implements provider.Provider. The cached KV handles are
// deliberately kept: a Consul handle holds no goroutine and nothing to release
// (its HTTP transport is what we want to reuse), and closing is therefore final
// only for this application instance, not for the handles.
func (c *consulCtrl) Close(ctx context.Context) error {
	c.mu.Lock()
	c.cancel()
	c.stopped = true
	n := len(c.listened)
	c.listened = map[string]struct{}{}
	c.mu.Unlock()
	if n > 0 {
		log.Infof(ctx, starterTag, "stopped %d consul watcher(s)", n)
	}
	return nil
}

// kvAPI is the slice of the Consul API surface this starter consumes. It
// exists so tests can fake the KV backend without a live Consul agent.
type kvAPI interface {
	Get(key string, q *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error)
}

// newConsulCtrl creates a controller with its caches ready, so the lazy
// nil-checks are kept out of the hot paths.
func newConsulCtrl() *consulCtrl {
	ctx, cancel := context.WithCancel(context.Background())
	return &consulCtrl{
		clients:  map[string]kvAPI{},
		listened: map[string]struct{}{},
		ctx:      ctx,
		cancel:   cancel,
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

// sourceFields returns the fields identifying a watched KV path. They are
// attached to a context with log.WithFields rather than repeated at each call
// site, and the keys match the ones the watch trigger stamps so a line and the
// refresh it concerns join on the same names.
func sourceFields(cs configSource) []log.Field {
	return []log.Field{
		log.String("address", cs.address),
		log.String("kv_path", cs.kvPath),
		log.String("datacenter", cs.datacenter),
	}
}

// clientFor returns a cached KV handle for the source, creating one if
// necessary. Its log line takes the source fields from ctx, which Load has
// already stamped.
func (c *consulCtrl) clientFor(ctx context.Context, cs configSource) (kvAPI, error) {
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
	log.Info(ctx, starterTag, log.Msg("created consul client"))
	kv := raw.KV()
	c.clients[key] = kv
	return kv, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from a Consul KV path, parses it according to the declared format, and
// installs a blocking-query watcher that triggers an application property
// refresh on change.
func (c *consulCtrl) Load(optional bool, source string) (map[string]string, error) {
	c.rearm()

	// Load is the top of this chain and Provider.Load takes no context, so the
	// one ctx the whole load shares is minted here — the helpers below receive
	// it instead of each minting their own.
	ctx := context.Background()

	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag,
			log.String("source", source),
			log.Err(err),
			log.Msg("parse consul source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx, sourceFields(cs)...)

	log.Debug(ctx, starterTag, func() []log.Field {
		return []log.Field{
			log.String("format", cs.format),
			log.Msg("loading consul config"),
		}
	})

	cli, err := c.clientFor(ctx, cs)
	if err != nil {
		log.Error(ctx, starterTag,
			log.Err(err),
			log.Msg("create consul client failed"))
		return nil, err
	}

	c.registerWatch(ctx, cli, cs, optional)

	pair, _, err := cli.Get(cs.kvPath, &api.QueryOptions{Datacenter: cs.datacenter})
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("optional config get kv failed, skipped"))
			return nil, nil
		}
		log.Error(ctx, starterTag,
			log.Err(err),
			log.Msg("get consul kv failed"))
		return nil, errutil.Explain(err, "get consul kv %s failed", cs.kvPath)
	}
	if pair == nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Msg("optional config kv not found, skipped"))
			return nil, nil
		}
		log.Error(ctx, starterTag, log.Msg("consul kv not found"))
		return nil, errutil.Explain(nil, "consul kv %s not found", cs.kvPath)
	}
	if len(pair.Value) == 0 {
		if optional {
			log.Warn(ctx, starterTag,
				log.Msg("optional config kv is empty, skipped"))
			return nil, nil
		}
		log.Error(ctx, starterTag, log.Msg("consul kv is empty"))
		return nil, errutil.Explain(nil, "consul kv %s is empty", cs.kvPath)
	}

	m, err := reader.Read(cs.format, pair.Value)
	if err != nil {
		log.Error(ctx, starterTag,
			log.String("format", cs.format),
			log.Err(err),
			log.Msg("parse consul kv failed"))
		return nil, errutil.Explain(err, "parse consul kv %s as %s failed", cs.kvPath, cs.format)
	}

	log.Info(ctx, starterTag,
		log.Int("keys", len(m)),
		log.Msg("loaded consul config"))
	return flatten.Flatten(m), nil
}

// registerWatch spawns a background goroutine that runs a Consul blocking
// query against the given KV path. Deduplicated across repeated Load calls.
// optional records whether the import declared the path optional: deleting an
// optional path is an expected transition (its properties simply disappear),
// while deleting a required one leaves the last snapshot in place, which the
// watcher surfaces as a warning.
func (c *consulCtrl) registerWatch(ctx context.Context, cli kvAPI, cs configSource, optional bool) {
	lk := clientKey(cs) + "|" + cs.kvPath

	c.mu.Lock()
	if _, ok := c.listened[lk]; ok {
		c.mu.Unlock()
		return
	}
	c.listened[lk] = struct{}{}
	// The goroutine outlives this load, so it runs on the controller's
	// generation context, not on the load's. It carries the same fields, which
	// is what lets the loop's own lines name the KV path without interpolating
	// it.
	wctx := log.WithFields(c.ctx, sourceFields(cs)...)
	c.mu.Unlock()

	log.Info(ctx, starterTag, log.Msg("watching consul kv for changes"))
	go c.watchLoop(wctx, cli, cs, optional)
}

// unwatch drops the dedup entry for a source's watch. Called when a watch
// goroutine exits because its generation was canceled: the entry would
// otherwise block re-watching after the next rearm (a Load that raced Close
// registers on an already-dead generation).
func (c *consulCtrl) unwatch(cs configSource) {
	lk := clientKey(cs) + "|" + cs.kvPath
	c.mu.Lock()
	delete(c.listened, lk)
	c.mu.Unlock()
}

// watchLoop runs the blocking-query loop for a single KV path. Errors retry
// silently-but-visibly: the first failure logs a warning, subsequent ones only
// debug (the loop keeps retrying every 2s, so per-failure warnings would flood),
// and recovery logs once at info.
func (c *consulCtrl) watchLoop(ctx context.Context, cli kvAPI, cs configSource, optional bool) {
	var lastIndex uint64
	initialized := false
	failing := false
	for {
		// The blocking query carries the watch generation's context, so Close
		// aborts an in-flight wait instead of leaving it parked for WaitTime.
		q := &api.QueryOptions{
			Datacenter: cs.datacenter,
			WaitIndex:  lastIndex,
			WaitTime:   5 * time.Minute,
		}
		pair, meta, err := cli.Get(cs.kvPath, q.WithContext(ctx))
		if err == nil && meta == nil {
			err = errutil.Explain(nil, "nil query meta")
		}
		if err != nil {
			if ctx.Err() != nil {
				c.unwatch(cs)
				return // shutting down
			}
			if !failing {
				log.Warn(ctx, starterTag,
					log.Err(err),
					log.Msg("consul watch failing, retrying every 2s (changes are missed until it recovers)"))
				failing = true
			} else {
				log.Debug(ctx, starterTag, func() []log.Field {
					return []log.Field{
						log.Err(err),
						log.Msg("consul watch still failing"),
					}
				})
			}
			select {
			case <-ctx.Done():
				c.unwatch(cs)
				return
			case <-time.After(2 * time.Second):
			}
			continue
		}
		if failing {
			log.Info(ctx, starterTag, log.Msg("consul watch recovered"))
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
				log.Warn(ctx, starterTag,
					log.Msg("consul kv deleted; stale snapshot retained until the key is restored"))
			}
			// Stamp the trigger with the change's identity (which KV entry, which
			// consul revision) so the refresh records logged and metered by
			// observability.RefreshConf carry what this round is about. The LastIndex
			// doubles as the refresh identifier: it is unique per KV change within
			// consul, so two refreshes from the same path are distinguishable.
			c.TriggerRefresh(log.WithFields(context.Background(),
				log.String("source", "consul"),
				log.String("kv_path", cs.kvPath),
				log.Int("last_index", int64(meta.LastIndex)),
			))
		}
	}
}
