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

package StarterElasticsearch

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"go-spring.org/cloud/discovery"
)

// errNoConnection is what the fallback pool reports when no seed connection
// could be built at all — the same "nothing to use" the transport's own pool
// would report.
var errNoConnection = errors.New("elasticsearch: no connection available")

// refreshInterval bounds how often the live pool re-reads the discovery
// snapshot. It is the propagation budget for a membership change: the check runs
// on the request path, so it is deliberately not per-request. A var so tests can
// shrink it.
var refreshInterval = time.Second

// livePool is the elastic-transport [elastictransport.ConnectionPool] that keeps
// the node set in step with the naming service. Without it the client is built
// over a boot-time snapshot and an instance joining or leaving the cluster is
// invisible until the process restarts — the documented one-shot limitation of
// relying on the transport's static `Addresses`.
//
// It does NOT select: it keeps a library-built inner pool (the transport's own
// selector and its live/dead bookkeeping) and rebuilds it only when the node set
// actually changes, so ES keeps choosing among nodes its own way. Rebuilding
// resets that bookkeeping — acceptable, because the node set it described just
// changed. The pool is address-sorted for the same reason the transport gets
// deterministic URLs: an unchanged cluster must not look like a change because
// the naming service reordered a snapshot.
//
// A read error or an empty snapshot keeps the last good set: for a search
// cluster, a registry hiccup should not black-hole traffic that a moment ago
// worked. (Contrast the memcached selector, which surfaces it — there the key
// location genuinely may have moved.)
type livePool struct {
	resolve  discovery.Resolver
	scheme   string
	selector elastictransport.Selector

	mu     sync.Mutex
	pool   elastictransport.ConnectionPool
	key    string
	last   time.Time
	closed bool
}

// ConcurrentSafe marks the pool as internally synchronized, so the transport
// calls it directly instead of double-locking through its synchronizedPool
// wrapper.
func (p *livePool) ConcurrentSafe() {}

// newLivePool builds the live pool over resolve, seeded with the connections the
// transport derived from the (already resolved) configured addresses, so a
// failure to read the snapshot later still leaves a working pool behind.
func newLivePool(resolve discovery.Resolver, scheme string, selector elastictransport.Selector, seed []*elastictransport.Connection) *livePool {
	inner, err := elastictransport.NewConnectionPool(seed, selector)
	if err != nil {
		inner = &emptyPool{}
	}
	p := &livePool{resolve: resolve, scheme: scheme, selector: selector, pool: inner}
	p.mu.Lock()
	p.reconcileLocked()
	p.mu.Unlock()
	return p
}

// Next returns the next connection, first bringing the node set up to date.
func (p *livePool) Next() (*elastictransport.Connection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.reconcileLocked()
	return p.pool.Next()
}

// OnSuccess reports a successful use of c to the inner pool, which owns the
// live/dead bookkeeping.
func (p *livePool) OnSuccess(c *elastictransport.Connection) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pool.OnSuccess(c)
}

// OnFailure reports a failed use of c to the inner pool.
func (p *livePool) OnFailure(c *elastictransport.Connection) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pool.OnFailure(c)
}

// URLs returns the URLs of the nodes currently in the pool.
func (p *livePool) URLs() []*url.URL {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pool.URLs()
}

// Close closes the inner pool. It is the [elastictransport.CloseableConnectionPool]
// half the transport calls when the client is closed.
func (p *livePool) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if cl, ok := p.pool.(elastictransport.CloseableConnectionPool); ok {
		return cl.Close(ctx)
	}
	return nil
}

// reconcileLocked brings the inner pool in step with the current endpoint
// snapshot, at most once per refreshInterval. Caller holds p.mu.
func (p *livePool) reconcileLocked() {
	if p.closed {
		return
	}
	if !p.last.IsZero() && time.Since(p.last) < refreshInterval {
		return
	}
	p.last = time.Now()

	eps, err := p.resolve()
	if err != nil || len(eps) == 0 {
		return // keep the last good set
	}
	addrs := make([]string, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, p.scheme+"://"+ep.Addr)
	}
	sort.Strings(addrs)
	key := strings.Join(addrs, ",")
	if key == p.key {
		return
	}

	conns := make([]*elastictransport.Connection, 0, len(addrs))
	for _, a := range addrs {
		u, err := url.Parse(a)
		if err != nil {
			continue
		}
		conns = append(conns, &elastictransport.Connection{URL: u})
	}
	if len(conns) == 0 {
		return
	}
	inner, err := elastictransport.NewConnectionPool(conns, p.selector)
	if err != nil {
		return
	}
	if cl, ok := p.pool.(elastictransport.CloseableConnectionPool); ok {
		_ = cl.Close(context.Background())
	}
	p.pool, p.key = inner, key
}

// emptyPool is the fallback when the seed connections cannot form a pool; it
// reports the same "nothing to use" the transport would, instead of panicking.
type emptyPool struct{}

func (emptyPool) Next() (*elastictransport.Connection, error) {
	return nil, errNoConnection
}

func (emptyPool) OnSuccess(*elastictransport.Connection) error { return nil }
func (emptyPool) OnFailure(*elastictransport.Connection) error { return nil }
func (emptyPool) URLs() []*url.URL                             { return nil }
