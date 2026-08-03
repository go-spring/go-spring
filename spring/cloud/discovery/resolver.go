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

package discovery

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
)

// Resolver is the stateful counterpart of [Discovery.Resolve]: it resolves a
// service name to its live endpoints once, then keeps that snapshot fresh via a
// background watch — so [Resolver.Pick] always draws from a current set.
//
// It is the shared piece every infrastructure-client starter reuses (Redis,
// MySQL, MongoDB, ...): the starter builds one Resolver per logical service
// name and calls Pick when it needs to open a connection, then dials the
// returned [Endpoint.Addr] itself. Resolver deliberately does NOT dial — it
// only decides "which live endpoint", leaving the socket (and the connection
// pool, retries, dead-connection eviction) to the client. That keeps it a pure
// decision layer over discovery, mirroring how loadbalance.Pick sits above
// discovery for per-request RPC selection.
type Resolver struct {
	q        Query   // materialized lookup, for error messages
	opts     []Option // replayed into Resolve/Watch on (re)open
	eps      atomic.Pointer[[]Endpoint]
	next     atomic.Uint64
	cancel   context.CancelFunc
	stopOnce sync.Once
}

// NewResolver returns a Resolver for name, narrowed by opts (e.g.
// [WithScheme]). It seeds the snapshot with one explicit [Discovery.Resolve] —
// a synchronous read of the current state that also fails fast when the service
// is unknown — then opens a [Discovery.Watch] to keep that snapshot fresh. The
// caller owns the lifecycle and must call Stop to release the background watch;
// Stop cancels the derived context, which closes the watch channel and ends the
// loop.
//
// name is required; opts are the same [Option] values Resolve/Watch take, so a
// starter wires scheme as discovery.NewResolver(ctx, d, c.ServiceName,
// discovery.WithScheme(c.Scheme)) and a starter that does not care about scheme
// passes none.
func NewResolver(ctx context.Context, d Discovery, name string, opts ...Option) (*Resolver, error) {
	ctx, cancel := context.WithCancel(ctx)

	eps, err := d.Resolve(ctx, name, opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("discovery: resolve %q: %w", name, err)
	}
	ch, err := d.Watch(ctx, name, opts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("discovery: watch %q: %w", name, err)
	}

	r := &Resolver{q: NewQuery(name, opts...), opts: opts, cancel: cancel}
	r.eps.Store(&eps)
	go r.loop(ch)
	return r, nil
}

// loop drains the watch channel, replacing the snapshot on every change until
// the channel closes (ctx cancelled) or a terminal error arrives. On either it
// stops and the Resolver keeps serving from the last snapshot it received.
func (r *Resolver) loop(ch <-chan WatchResult) {
	for res := range ch {
		if res.Err != nil {
			return
		}
		eps := res.Endpoints
		r.eps.Store(&eps)
	}
}

// Endpoints returns the current endpoint snapshot. It is safe for concurrent
// use; callers must not mutate the returned slice.
func (r *Resolver) Endpoints() []Endpoint {
	return *r.eps.Load()
}

// Pick selects one eligible endpoint by round-robin. Weight is ignored —
// weighted selection belongs in loadbalance. Eligibility follows the
// [Endpoint] contract: prefer !Disabled && Healthy, degrade to !Disabled, never
// Disabled. It errors when the service has no endpoints, or every endpoint is
// disabled.
func (r *Resolver) Pick() (Endpoint, error) {
	eps := *r.eps.Load()
	if len(eps) == 0 {
		return Endpoint{}, fmt.Errorf("discovery: no endpoints for %q", r.q)
	}

	eligible := eps[:0:0]
	for _, ep := range eps {
		if !ep.Disabled && ep.Healthy {
			eligible = append(eligible, ep)
		}
	}
	if len(eligible) == 0 {
		for _, ep := range eps {
			if !ep.Disabled {
				eligible = append(eligible, ep)
			}
		}
	}
	if len(eligible) == 0 {
		return Endpoint{}, fmt.Errorf("discovery: no eligible (non-disabled) endpoints for %q", r.q)
	}

	i := r.next.Add(1) - 1
	return eligible[int(i%uint64(len(eligible)))], nil
}

// Stop ends the background watch by cancelling its context. It is safe to call
// more than once. The returned error is always nil (cancellation cannot fail);
// the error signature is kept so Stop plugs directly into a bean destructor
// (gs Destroy expects func(T) error).
func (r *Resolver) Stop() error {
	r.stopOnce.Do(r.cancel)
	return nil
}
