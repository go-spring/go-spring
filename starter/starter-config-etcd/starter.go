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
// config provider that can be consumed via spring.config.import, together with
// the bridge that wires remote config changes into the application-wide
// property refresh for live hot-reload.
//
// This starter covers the config-center role only. Service discovery
// (etcd naming) is a separate concern and is not provided here.
package StarterConfigEtcd

import (
	"context"
	"errors"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func init() {
	// Register "etcd" as a remote configuration provider. The controller
	// itself is registered (not just its Load method): the runtime holds the
	// registered provider so it can stop the watchers at shutdown (see
	// provider.Provider). Watch-triggered refreshes go through the
	// gs.RefreshProperties package-level facade, so the controller needs no
	// bean wiring at all.
	conf.RegisterProvider("etcd", newEtcdCtrl())
}

var starterTag = log.RegisterAppTag("config_etcd", "")

// etcdCtrl is the single object that owns the full lifecycle of etcd
// configuration: loading keys, watching for changes, and triggering
// property refresh. Its clients are bootstrap infrastructure: they exist
// before the container does and therefore deliberately opt out of
// observability and governance wiring — don't "fix" that here; consumers
// that need instrumented clients (e.g. the governance source) build their
// own.
type etcdCtrl struct {
	mu       sync.Mutex
	clients  map[string]*clientv3.Client
	listened map[string]struct{}

	// ctx is the watch generation: Close cancels it, which stops every watch
	// goroutine; the next Load starts a fresh one (see rearm).
	ctx context.Context
	// cancel cancels ctx.
	cancel context.CancelFunc
	// stopped is true between Close and the next Load.
	stopped bool
}

// newEtcdCtrl creates a controller with its caches ready, so the lazy
// nil-checks are kept out of the hot paths.
func newEtcdCtrl() *etcdCtrl {
	ctx, cancel := context.WithCancel(context.Background())
	return &etcdCtrl{
		clients:  map[string]*clientv3.Client{},
		listened: map[string]struct{}{},
		ctx:      ctx,
		cancel:   cancel,
	}
}

// rearm restarts the watch generation after a Close, so the first Load of a new
// application instance can install watchers again.
func (c *etcdCtrl) rearm() {
	c.mu.Lock()
	if c.stopped {
		c.ctx, c.cancel = context.WithCancel(context.Background())
		c.stopped = false
	}
	c.mu.Unlock()
}

// Close stops every watch goroutine and closes every client. It implements
// provider.Provider. Closing is final only for this application instance: the
// caches are dropped, so the next Load builds fresh clients and re-watches.
func (c *etcdCtrl) Close(ctx context.Context) error {
	c.mu.Lock()
	c.cancel()
	c.stopped = true
	clients := c.clients
	c.clients = map[string]*clientv3.Client{}
	c.listened = map[string]struct{}{}
	c.mu.Unlock()

	var errs []error
	for _, cli := range clients {
		if err := cli.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if n := len(clients); n > 0 {
		log.Info(ctx, starterTag, log.Int("clients", n), log.Msg("closed etcd clients"))
	}
	return errors.Join(errs...)
}

// TriggerRefresh is called by the watch goroutines when a watched key
// changes. Before the app has started, gs.RefreshProperties returns an
// error and the change is dropped — the startup load already captured the
// state.
func (c *etcdCtrl) TriggerRefresh(ctx context.Context) {
	// The refresh outcome (status, duration, error) is logged and metered
	// centrally by observability.RefreshConf; this layer only records backend events.
	_ = observability.RefreshConf(ctx, gs.RefreshProperties)
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

// sourceFields returns the fields identifying a watched key. They are attached
// to a context with log.WithFields rather than repeated at each call site, and
// the keys match the ones the watch trigger stamps so a line and the refresh it
// concerns join on the same names.
func sourceFields(cs configSource) []log.Field {
	return []log.Field{
		log.String("endpoint", cs.endpoint),
		log.String("key", cs.key),
	}
}

// clientFor returns a cached client for the source, creating one if necessary.
// Its log line takes the source fields from ctx, which Load has already stamped.
func (c *etcdCtrl) clientFor(ctx context.Context, cs configSource) (*clientv3.Client, error) {
	key := clientKey(cs)

	c.mu.Lock()
	defer c.mu.Unlock()

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
	log.Info(ctx, starterTag, log.Msg("create etcd client success"))
	c.clients[key] = cli
	return cli, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from etcd, parses it according to the declared format, and installs a
// change watcher that triggers an application property refresh.
func (c *etcdCtrl) Load(optional bool, source string) (map[string]string, error) {
	c.rearm()

	// Load is the top of this chain and Provider.Load takes no context, so the
	// one ctx the whole load shares is minted here — the helpers below receive
	// it instead of each minting their own.
	ctx := context.Background()

	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag, err, log.String("source", source), log.Msg("parse etcd source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx, sourceFields(cs)...)

	log.Debug(ctx, starterTag, func() []log.Field {
		return []log.Field{
			log.String("format", cs.format),
			log.Msg("loading etcd config"),
		}
	})

	cli, err := c.clientFor(ctx, cs)
	if err != nil {
		log.Error(ctx, starterTag, err, log.Msg("create etcd client failed"))
		return nil, err
	}

	c.registerWatcher(ctx, cli, cs, optional)

	getCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	resp, err := cli.Get(getCtx, cs.key)
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("skip optional config get key failed"))
			return nil, nil
		}
		log.Error(ctx, starterTag, err, log.Msg("get etcd key failed"))
		return nil, errutil.Explain(err, "get etcd key %s failed", cs.key)
	}
	if len(resp.Kvs) == 0 {
		if optional {
			log.Warn(ctx, starterTag,
				log.Msg("skip optional config key is empty"))
			return nil, nil
		}
		err := errutil.Explain(nil, "etcd key %s is empty", cs.key)
		log.Error(ctx, starterTag, err, log.Msg("etcd key is empty"))
		return nil, err
	}

	content := resp.Kvs[0].Value
	m, err := reader.Read(cs.format, content)
	if err != nil {
		log.Error(ctx, starterTag, err, log.String("format", cs.format), log.Msg("parse etcd key failed"))
		return nil, errutil.Explain(err, "parse etcd key %s as %s failed", cs.key, cs.format)
	}

	log.Info(ctx, starterTag,
		log.Int("keys", len(m)),
		log.Msg("load etcd config success"))
	return flatten.Flatten(m), nil
}

// registerWatcher installs an etcd change watcher for the given key,
// deduplicated across repeated Load calls. optional records whether the import
// declared the key optional: deleting an optional key is an expected transition
// (its properties simply disappear), while deleting a required one leaves the
// last snapshot in place, which the watcher surfaces as a warning.
func (c *etcdCtrl) registerWatcher(ctx context.Context, cli *clientv3.Client, cs configSource, optional bool) {
	lk := clientKey(cs) + "|" + cs.key

	c.mu.Lock()
	if _, ok := c.listened[lk]; ok {
		c.mu.Unlock()
		return
	}
	c.listened[lk] = struct{}{}
	// The goroutine outlives this load, so it runs on the controller's
	// generation context, not on the load's. It carries the same fields, which
	// is what lets the loop's own lines name the key without interpolating it.
	wctx := log.WithFields(c.ctx, sourceFields(cs)...)
	c.mu.Unlock()

	log.Info(ctx, starterTag, log.Msg("watching etcd key for changes"))

	go func() {
		for {
			ch := cli.Watch(wctx, cs.key)
			for wr := range ch {
				if err := wr.Err(); err != nil {
					log.Warn(wctx, starterTag,
						log.Err(err),
						log.Msg("etcd watch returned an error"))
				}
				deleted := false
				for _, ev := range wr.Events {
					if ev.Type == clientv3.EventTypeDelete {
						deleted = true
					}
				}
				if deleted && !optional {
					log.Warn(wctx, starterTag,
						log.Msg("etcd key deleted; stale snapshot retained until the key is restored"))
				}
				if len(wr.Events) > 0 {
					// Stamp the trigger with the change's identity (which key, which
					// etcd revision) so the refresh records logged and metered by
					// observability.RefreshConf carry what this round is about. The
					// last event's ModRevision doubles as the refresh identifier: it
					// is unique per key change within etcd, so two refreshes from the
					// same key are distinguishable.
					rev := int64(0)
					if ev := wr.Events[len(wr.Events)-1]; ev.Kv != nil {
						rev = ev.Kv.ModRevision
					}
					c.TriggerRefresh(log.WithFields(context.Background(),
						log.String("source", "etcd"),
						log.String("key", cs.key),
						log.Int("mod_revision", rev),
					))
				}
			}
			// The channel closes either because the application is shutting
			// down (Close cancelled the watch generation) or because the
			// watcher is genuinely dead (unrecoverable error). The former
			// exits; the latter would otherwise exit silently and this key
			// would never refresh again, so resubscribe and keep watching.
			select {
			case <-wctx.Done():
				return
			default:
			}
			err := errutil.Explain(nil, "etcd watch channel closed")
			log.Error(wctx, starterTag, err, log.Msg("etcd watch channel closed; resubscribing in 5s"))
			select {
			case <-wctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}()
}
