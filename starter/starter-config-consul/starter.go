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
package StarterConfigConsul

import (
	"context"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/hashicorp/consul/api"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/conf/reader"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var starterTag = log.RegisterAppTag("config", "consul")

func init() {
	conf.RegisterProvider("consul", newConsulCtrl())
}

// kvAPI is the slice of the Consul API surface this starter consumes. It
// exists so tests can fake the KV backend without a live Consul agent.
type kvAPI interface {
	Get(key string, q *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error)
}

// consulCtrl owns the full lifecycle of consul configuration: it loads KV
// entries through a client cache and watches for changes through its
// watchCore. The two sides keep separate mutexes, so the client cache never
// contends with watch registration.
type consulCtrl struct {
	watch    watchCore
	clientMu sync.Mutex
	clients  map[string]kvAPI
}

func newConsulCtrl() *consulCtrl {
	return &consulCtrl{
		clients: map[string]kvAPI{},
		watch:   newWatchCore(),
	}
}

// Close stops the watch side; it satisfies conf/provider.Provider.
func (c *consulCtrl) Close() {
	c.watch.Close()
}

// configSource holds the parsed components of a consul provider source string.
type configSource struct {
	address    string
	scheme     string
	kvPath     string
	token      string
	datacenter string
	format     string
	retryMs    int
}

// redactSource masks the token query parameter of a source string so the
// remainder is safe to print. Everything user-visible — parse errors, log
// fields — must carry the redacted form; the raw form never reaches output.
func redactSource(source string) string {
	i := strings.Index(source, "?")
	if i < 0 {
		return source
	}
	q := strings.Split(source[i+1:], "&")
	kept := q[:0]
	for _, kv := range q {
		if strings.HasPrefix(kv, "token=") {
			kv = "token=***"
		}
		kept = append(kept, kv)
	}
	return source[:i+1] + strings.Join(kept, "&")
}

// parseSource parses a provider source of the form
// <host>:<port>/<kv-path>?format=..&token=..&datacenter=..&scheme=..&retry-ms=..
func parseSource(source string) (configSource, error) {
	u, err := url.Parse("consul://" + source)
	if err != nil {
		return configSource{}, errutil.Explain(err, "invalid consul source %q", redactSource(source))
	}
	if u.Host == "" {
		return configSource{}, errutil.Explain(nil, "missing consul server address in %q", redactSource(source))
	}
	kvPath := strings.TrimPrefix(u.Path, "/")
	if kvPath == "" {
		return configSource{}, errutil.Explain(nil, "missing kv path in %q", redactSource(source))
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
	cs.retryMs = 2000
	if v := q.Get("retry-ms"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return configSource{}, errutil.Explain(nil, "invalid retry-ms in %q", redactSource(source))
		}
		cs.retryMs = n
	}
	return cs, nil
}

// clientKey builds a cache key for a client: one Consul KV handle per
// (address, scheme, token, datacenter).
func clientKey(cs configSource) string {
	return cs.address + "|" + cs.scheme + "|" + cs.token + "|" + cs.datacenter
}

// clientFor returns a cached KV handle for the source, creating one if
// necessary.
func (c *consulCtrl) clientFor(ctx context.Context, cs configSource) (kvAPI, error) {
	key := clientKey(cs)

	c.clientMu.Lock()
	defer c.clientMu.Unlock()

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
	log.Infof(ctx, starterTag, "create consul client success")
	kv := raw.KV()
	c.clients[key] = kv
	return kv, nil
}

// Load implements conf/provider.Provider. It fetches configuration content
// from a Consul KV path, parses it according to the declared format, and
// installs a blocking-query watcher that triggers an application property
// refresh on change.
func (c *consulCtrl) Load(ctx context.Context, optional bool, source string) (map[string]string, error) {
	cs, err := parseSource(source)
	if err != nil {
		log.Error(ctx, starterTag, err,
			log.String("source", redactSource(source)),
			log.Msg("parse consul source failed"))
		return nil, err
	}

	// The source's identity rides on the context from here on: every event this
	// load and its helpers print carries it without repeating it at each call
	// site.
	ctx = log.WithFields(ctx,
		log.String("address", cs.address),
		log.String("kv_path", cs.kvPath),
		log.String("format", cs.format),
		log.String("datacenter", cs.datacenter),
	)

	log.Debugf(ctx, starterTag, "loading consul config")

	cli, err := c.clientFor(ctx, cs)
	if err != nil {
		log.Errorf(ctx, starterTag, err, "create consul client failed")
		return nil, err
	}

	// Read first to learn the KV index, then install the watcher seeded with
	// it; the watcher's first query then resumes from that index and reports
	// any change since — including one that landed mid-read. Registering
	// regardless of the read's outcome keeps the "missing optional key is
	// still watched" behaviour.
	m, since, err := loadFromClient(ctx, cli, cs, optional)
	c.watch.registerWatch(cli, cs, optional, since)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// loadFromClient reads the KV pair once, applies the optional/unset/empty rules,
// and parses it into flattened properties. It also returns the KV index the read
// observed (0 when unknown), which the caller seeds the watcher with so the
// watch resumes from exactly there.
func loadFromClient(ctx context.Context, cli kvAPI, cs configSource, optional bool) (map[string]string, uint64, error) {
	pair, meta, err := cli.Get(cs.kvPath, &api.QueryOptions{Datacenter: cs.datacenter})
	var since uint64
	if meta != nil {
		since = meta.LastIndex
	}
	if err != nil {
		if optional {
			log.Warn(ctx, starterTag,
				log.Err(err),
				log.Msg("skip optional config get kv failed"))
			return nil, since, nil
		}
		log.Errorf(ctx, starterTag, err, "get consul kv failed")
		return nil, since, errutil.Explain(err, "get consul kv %s failed", cs.kvPath)
	}
	if pair == nil {
		if optional {
			log.Warnf(ctx, starterTag, "skip optional config kv not found")
			return nil, since, nil
		}
		err := errutil.Explain(nil, "consul kv %s not found", cs.kvPath)
		log.Errorf(ctx, starterTag, err, "consul kv not found")
		return nil, since, err
	}
	if len(pair.Value) == 0 {
		if optional {
			log.Warnf(ctx, starterTag, "skip optional config kv is empty")
			return nil, since, nil
		}
		err := errutil.Explain(nil, "consul kv %s is empty", cs.kvPath)
		log.Errorf(ctx, starterTag, err, "consul kv is empty")
		return nil, since, err
	}

	m, err := reader.Read(cs.format, pair.Value)
	if err != nil {
		log.Errorf(ctx, starterTag, err, "parse consul kv failed")
		return nil, since, errutil.Explain(err, "parse consul kv %s as %s failed", cs.kvPath, cs.format)
	}

	log.Infof(ctx, starterTag, "load consul config success")
	return flatten.Flatten(m), since, nil
}
