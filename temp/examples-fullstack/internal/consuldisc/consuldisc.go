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

// Package consuldisc is a small, client-side [discovery.Discovery] backed by the
// Consul catalog. starter-registry-consul is a *register-side* starter (it
// advertises this instance into Consul); it does not ship a client-side resolver.
// The unified cloud/discovery abstraction is exactly the seam meant to close
// that gap, so the reference app supplies its own Consul-backed Discovery here
// and registers it once via discovery.Register — the gateway's lb://order route
// and the order service's order->inventory call then resolve through it with no
// per-caller Consul code.
//
// This lives in the sample (not in a starter) on purpose: a real deployment
// would either use starter-registry-k8s or contribute a company Consul resolver.
// Keeping it here proves the abstraction is enough to bridge a register-only
// backend to full client-side discovery.
package consuldisc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/cloud/discovery"
)

// Backend resolves service names against a Consul agent. The first Resolve of
// a service seeds a per-service cache and starts a blocking-query loop that
// keeps it fresh; later calls are in-memory reads.
type Backend struct {
	client *api.Client

	ctx     context.Context
	mu      sync.Mutex
	entries map[string]*consulEntry
}

// consulEntry is the cached snapshot for one service name+tag.
type consulEntry struct {
	mu     sync.Mutex
	eps    []discovery.Endpoint
	seeded bool
}

// Register builds a Consul-backed Discovery for the agent at addr (e.g.
// "127.0.0.1:8500") and publishes it in the cloud/discovery registry under
// name, so discovery.GetDiscovery(name) (used by the gateway) and any Resolver find
// it. It is meant to be called once at process start.
func Register(name, addr string) error {
	b, err := New(addr)
	if err != nil {
		return err
	}
	discovery.RegisterDiscovery(name, b)
	return nil
}

// New returns a Consul-backed Discovery for the agent at addr.
func New(addr string) (*Backend, error) {
	client, err := api.NewClient(&api.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("consuldisc: new client: %w", err)
	}
	return &Backend{client: client, ctx: context.Background(), entries: map[string]*consulEntry{}}, nil
}

// Resolve returns the current healthy endpoints for name. The first call seeds
// a per-service cache and starts a blocking-query loop that keeps it fresh;
// later calls are in-memory reads. opts narrow the result: [discovery.WithTag]
// is passed to Consul's service query server-side, [discovery.WithScheme]
// filters the returned endpoints by transport scheme.
func (b *Backend) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	q := discovery.NewQuery(name, opts...)
	e := b.entry(name, q.Tag)
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.seeded {
		entries, _, err := b.client.Health().Service(name, q.Tag, true, (&api.QueryOptions{}).WithContext(ctx))
		if err != nil {
			return nil, fmt.Errorf("consuldisc: resolve %q: %w", name, err)
		}
		e.eps = toEndpoints(entries)
		e.seeded = true
		go b.watchLoop(name, q.Tag, e)
	}
	return discovery.FilterByScheme(append([]discovery.Endpoint(nil), e.eps...), q.Scheme), nil
}

// watchLoop keeps one service's cache fresh via Consul blocking queries until
// the backend's context ends. A failed query keeps the stale snapshot — stale
// addresses are safer than none — and retries on the next index advance.
func (b *Backend) watchLoop(name, tag string, e *consulEntry) {
	w := &watcher{backend: b, name: name, tag: tag, ctx: b.ctx}
	for w.ctx.Err() == nil {
		eps, err := w.next()
		if err != nil {
			if w.ctx.Err() != nil {
				return
			}
			continue
		}
		e.mu.Lock()
		e.eps = eps
		e.mu.Unlock()
	}
}

// entry returns (creating if needed) the cache entry for name+tag.
func (b *Backend) entry(name, tag string) *consulEntry {
	b.mu.Lock()
	defer b.mu.Unlock()
	key := name + "\x00" + tag
	e, ok := b.entries[key]
	if !ok {
		e = &consulEntry{}
		b.entries[key] = e
	}
	return e
}

// toEndpoints maps Consul service entries to discovery endpoints. It prefers the
// service-advertised address, falling back to the node address when the service
// left it blank (Consul's own convention).
func toEndpoints(entries []*api.ServiceEntry) []discovery.Endpoint {
	eps := make([]discovery.Endpoint, 0, len(entries))
	for _, e := range entries {
		host := e.Service.Address
		if host == "" {
			host = e.Node.Address
		}
		eps = append(eps, discovery.Endpoint{
			Addr:     fmt.Sprintf("%s:%d", host, e.Service.Port),
			Weight:   e.Service.Weights.Passing,
			Healthy:  true,
			Metadata: e.Service.Meta,
		})
	}
	return eps
}

// watcher drives one service's blocking-query loop. next blocks until the
// catalog's modify index advances past the last one seen, then returns the new
// endpoint set. A WaitTime bounds the block so a cancelled context unblocks
// promptly.
type watcher struct {
	backend   *Backend
	name      string
	tag       string // narrowing from WithTag; "" = any
	lastIndex uint64
	ctx       context.Context
}

// next blocks for the next snapshot. It loops past block timeouts (index
// unchanged) so callers only ever see real changes, and returns ctx.Err() once
// the context is cancelled.
func (w *watcher) next() ([]discovery.Endpoint, error) {
	for {
		if err := w.ctx.Err(); err != nil {
			return nil, err
		}
		q := (&api.QueryOptions{WaitIndex: w.lastIndex, WaitTime: 30 * time.Second}).WithContext(w.ctx)
		entries, meta, err := w.backend.client.Health().Service(w.name, w.tag, true, q)
		if err != nil {
			if w.ctx.Err() != nil {
				return nil, w.ctx.Err()
			}
			return nil, fmt.Errorf("consuldisc: watch %q: %w", w.name, err)
		}
		// A blocking query that times out with no change returns the same index;
		// keep waiting instead of emitting a duplicate snapshot.
		if meta.LastIndex == w.lastIndex {
			continue
		}
		w.lastIndex = meta.LastIndex
		return toEndpoints(entries), nil
	}
}
