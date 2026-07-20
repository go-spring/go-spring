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
// The unified stdlib/discovery abstraction is exactly the seam meant to close
// that gap, so the reference app supplies its own Consul-backed Discovery here
// and registers it once via discovery.Register — the gateway's lb://order route
// and the order service's order->inventory call then resolve through it with no
// per-caller Consul code.
//
// This lives in the sample (not in a starter) on purpose: a real deployment
// would either use starter-discovery-k8s or contribute a company Consul resolver.
// Keeping it here proves the abstraction is enough to bridge a register-only
// backend to full client-side discovery.
package consuldisc

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/spring/cloud/discovery"
)

// Backend resolves service names against a Consul agent.
type Backend struct {
	client *api.Client
}

// Register builds a Consul-backed Discovery for the agent at addr (e.g.
// "127.0.0.1:8500") and publishes it in the stdlib/discovery registry under
// name, so discovery.MustGet(name) (used by the gateway) and any LiveDialer find
// it. It is meant to be called once at process start.
func Register(name, addr string) error {
	b, err := New(addr)
	if err != nil {
		return err
	}
	discovery.Register(name, b)
	return nil
}

// New returns a Consul-backed Discovery for the agent at addr.
func New(addr string) (*Backend, error) {
	client, err := api.NewClient(&api.Config{Address: addr})
	if err != nil {
		return nil, fmt.Errorf("consuldisc: new client: %w", err)
	}
	return &Backend{client: client}, nil
}

// Resolve returns the current healthy endpoints for name.
func (b *Backend) Resolve(ctx context.Context, name string) ([]discovery.Endpoint, error) {
	entries, _, err := b.client.Health().Service(name, "", true, (&api.QueryOptions{}).WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("consuldisc: resolve %q: %w", name, err)
	}
	return toEndpoints(entries), nil
}

// Watch subscribes to name via Consul blocking queries. Each catalog change
// yields a fresh snapshot from Watcher.Next.
func (b *Backend) Watch(ctx context.Context, name string) (discovery.Watcher, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &watcher{backend: b, name: name, ctx: ctx, cancel: cancel}, nil
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

// watcher streams snapshots for one service using Consul blocking queries: each
// Next call blocks until the catalog's modify index advances past the last one
// it saw, then returns the new endpoint set. A WaitTime bounds the block so a
// stopped watcher (context cancelled) unblocks promptly.
type watcher struct {
	backend   *Backend
	name      string
	lastIndex uint64
	ctx       context.Context
	cancel    context.CancelFunc
}

// Next blocks for the next snapshot. It loops past block timeouts (index
// unchanged) so callers only ever see real changes, and returns an error once
// the watcher is stopped.
func (w *watcher) Next() ([]discovery.Endpoint, error) {
	for {
		if err := w.ctx.Err(); err != nil {
			return nil, err
		}
		q := (&api.QueryOptions{WaitIndex: w.lastIndex, WaitTime: 30 * time.Second}).WithContext(w.ctx)
		entries, meta, err := w.backend.client.Health().Service(w.name, "", true, q)
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

// Stop cancels the blocking query and releases the watch. Safe to call twice.
func (w *watcher) Stop() error {
	w.cancel()
	return nil
}
