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

// chain.go is the request-path handle over one immutable endpoint-selection
// generation. The manager owns the registrations and builds the generations;
// this file is the object a caller holds.

package loadbalance

import (
	"context"
	"sync/atomic"

	"go-spring.org/cloud/discovery"
)

// Chain is the OUTER layer of client-side load balancing: the egress a request
// path talks to, and the object that follows the governance selection for its
// service. It owns a whole endpoint-selection chain — the [Balancer], its
// [Tracker] and the inner [Pool] that binds them to a discovery source — as an
// immutable generation, REBUILT FROM SCRATCH by [Manager.Apply] whenever the
// selection its service resolves to moves. Callers hold the chain, not the pool:
// [Chain.Pick], [Chain.Complete] and [Chain.Selection] are the
// outward surface, each delegating to the generation in force.
//
// A changed selection means a new balancer, a new tracker and a new pool,
// sharing nothing with the generation it replaces. After a config change nothing
// from the old configuration is still in force — which is the property a Chain
// exists for, and also its price: in-flight counts, a hash ring and observed
// failures all start over. The alternative would be to retune the pool in place,
// which is exactly what a Chain is built to avoid: a change must not leave any
// of the configuration it replaced still deciding traffic.
//
// # The manager pushes; the chain holds still
//
// A rebuild happens ON [Manager.Apply], on the governance source's single loop —
// never on the request path. A request path's whole contact with refresh is one
// atomic load of the generation in force: no lock, no comparison, no lazy
// first-request build. This is the fault kernel's shape ([fault.Manager.Fire]
// loads its core the same way): the stable shell publishes, the hot path reads.
//
// The service IS the key: several chains may share a service (two clients on one
// service name), and each is rebuilt independently by the same push.
//
// A chain carries no identity of its own beyond the generation it points at.
// The service it follows is its key in the manager's registration table, and the
// source its generations are built over lives inside each [Pool] — the base
// source, unchanged from one generation to the next, because the selection's
// selector is applied in [Pool.Pick] rather than baked into the source. A
// TARGET change (another service, another discovery backend) is therefore a new
// registration and a new chain, which is the caller's decision: only the caller
// knows what its upstream points at.
type Chain struct {
	// cur is the generation in force: the pool a request pays attention to. It
	// is read on every request and stored only by the manager's loop on a
	// rebuild, so it is an atomic pointer — neither side ever blocks on the
	// other. The pool carries the selection it was built from (see
	// [Pool.Selection]), which is also the change-test record: one pointer, one
	// record, no way for the two to disagree.
	cur atomic.Pointer[Pool]
}

// Pick selects one live, healthy, non-suspended endpoint from the generation in
// force. It is the chain-level egress: a request path talks to the chain and
// never to the pool underneath. See [Pool.Pick] for the filtering and fallback
// semantics.
//
// Pick is a pure read: one atomic load, then the pool's own work. Refresh
// happens entirely on [Manager.Apply], so a Pick never triggers, waits on, or
// observes a half-built generation.
//
// ctx is the request's, and is threaded to the pool's endpoint source.
func (c *Chain) Pick(ctx context.Context, info PickInfo) (discovery.Endpoint, error) {
	return c.cur.Load().Pick(ctx, info)
}

// Complete settles the request that Pick issued to ep. Invoke it exactly once
// per picked endpoint, after the request ends. See [Pool.Complete].
func (c *Chain) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	c.cur.Load().Complete(ctx, ep, err)
}

// Selection returns the selection the generation in force was built from. It is
// an inspection and test helper.
func (c *Chain) Selection() Selection { return c.cur.Load().Selection() }
