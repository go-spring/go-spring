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

package loadbalance

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// errChainBoom is a request failure, for feeding the suspension tracker.
var errChainBoom = errors.New("boom")

// TestChainBuildsFromScratchOnChange covers the contract of the rebuild path: the
// first generation already runs the selection in force, and a push that moves
// this label's selection replaces the whole chain — a new pool, not the old one
// mutated.
func TestChainBuildsFromScratchOnChange(t *testing.T) {
	f := &fakeSource{cur: Selection{Balancer: LeastConn, OutlierThreshold: 2, OutlierSuspendFor: 30}}
	m := f.manager(t)

	c := m.Chain(context.Background(), staticSource(eps("a", "b")...), "demo:service")

	// Built before any push, but from the selection already in force.
	assert.Number(t, c.Generation()).Equal(uint64(1))
	assert.That(t, c.Selection().Balancer).Equal(LeastConn)
	assert.Number(t, c.Tracker().Config().Threshold).Equal(2)

	f.push(m, Selection{Balancer: Weighted, OutlierThreshold: 5, OutlierSuspendFor: 60})

	assert.Number(t, c.Generation()).Equal(uint64(2))
	assert.That(t, c.Selection().Balancer).Equal(Weighted)
	assert.Number(t, c.Tracker().Config().Threshold).Equal(5)
}

// TestChainRebuildDropsObservedFailures is what separates a Chain from a bound
// pool, stated as an observable fact: the old pool suspends an endpoint, the
// rebuild throws that observation away, and the new generation puts the endpoint
// straight back into rotation. A rebuild shares nothing with the generation it
// replaces — not even what it learned.
func TestChainRebuildDropsObservedFailures(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Default: Selection{Balancer: RoundRobin, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}})

	c := m.Chain(context.Background(), staticSource(eps("a", "b")...), "demo:service")

	c.Complete(context.Background(), discovery.Endpoint{Addr: "a"}, errChainBoom)
	assert.That(t, c.Tracker().Suspended("a")).True()

	f := &fakeSource{cur: Selection{Balancer: Random, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}}
	f.push(m, f.cur)

	assert.Number(t, c.Generation()).Equal(uint64(2))
	assert.That(t, c.Tracker().Suspended("a")).False()

	// The endpoint is genuinely back: the new generation picks from the full set.
	ep, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr == "a" || ep.Addr == "b").True()
}

// TestChainRebuildIsDrivenByTheChangedLabel pins that the diff answers "which
// service changed" rather than rebuilding everything: a rule scoped to one label
// moves only that label's chain, and re-applying an unchanged document moves
// nothing at all.
func TestChainRebuildIsDrivenByTheChangedLabel(t *testing.T) {
	m := newManager(t)

	a := m.Chain(context.Background(), staticSource(eps("a")...), "svc:a")
	b := m.Chain(context.Background(), staticSource(eps("b")...), "svc:b")

	m.Apply(context.Background(), Settings{Rules: []Rule{{Service: "svc:b", Selection: Selection{Balancer: LeastConn}}}})

	assert.Number(t, a.Generation()).Equal(uint64(1)) // untouched: still the base generation
	assert.Number(t, b.Generation()).Equal(uint64(2))
	assert.That(t, b.Selection().Balancer).Equal(LeastConn)

	// The same document again is not a change, so neither chain is rebuilt.
	m.Apply(context.Background(), Settings{Rules: []Rule{{Service: "svc:b", Selection: Selection{Balancer: LeastConn}}}})

	assert.Number(t, a.Generation()).Equal(uint64(1))
	assert.Number(t, b.Generation()).Equal(uint64(2))
}

// TestChainUnknownStrategyKeepsLastGood pins the degrade-don't-fail rule on the
// rebuild path: a rule naming a strategy that does not exist is refused, the
// last good generation stays in force and keeps serving.
func TestChainUnknownStrategyKeepsLastGood(t *testing.T) {
	m := newManager(t)
	c := m.Chain(context.Background(), staticSource(eps("a")...), "demo:service")

	m.Apply(context.Background(), Settings{Default: Selection{Balancer: LeastConn}})
	assert.That(t, c.Selection().Balancer).Equal(LeastConn)

	m.Apply(context.Background(), Settings{Default: Selection{Balancer: "no_such_strategy"}})

	assert.Number(t, c.Generation()).Equal(uint64(2)) // the refused rebuild built nothing
	ep, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr).Equal("a")
}

// TestChainPerChainNotPerLabel pins that the label is matched, not used as a key:
// one label legitimately backs more than one chain (two clients sharing a
// service name), so a change to that label rebuilds every chain carrying it, and
// each keeps its own endpoint source.
func TestChainPerChainNotPerLabel(t *testing.T) {
	m := newManager(t)

	c1 := m.Chain(context.Background(), staticSource(eps("a")...), "demo:service")
	c2 := m.Chain(context.Background(), staticSource(eps("b")...), "demo:service")

	m.Apply(context.Background(), Settings{Default: Selection{Balancer: LeastConn}})

	assert.Number(t, c1.Generation()).Equal(uint64(2))
	assert.Number(t, c2.Generation()).Equal(uint64(2))

	ep1, err := c1.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep1.Addr).Equal("a")
	ep2, err := c2.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep2.Addr).Equal("b")
}

// TestChainSwapIsLive pins the egress contract: after a governance change, a
// fresh chain-level Pick/Complete pair runs against the new generation, and a
// rebuild never leaves the chain without one.
func TestChainSwapIsLive(t *testing.T) {
	m := newManager(t)
	c := m.Chain(context.Background(), staticSource(eps("a")...), "demo:service")

	m.Apply(context.Background(), Settings{Default: Selection{Balancer: LeastConn}})
	assert.Number(t, c.Generation()).Equal(uint64(2))

	ep, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	c.Complete(context.Background(), ep, nil)

	assert.That(t, c.Selection().Balancer).Equal(LeastConn)
}

// TestChainConcurrentPickAndRebuild is the race guard for the atomic swap: picks
// run while pushes move the chain underneath them. An exact rebuild count is NOT
// asserted, and deliberately so: a chain follows the config on the request path,
// so a generation no request ever observes is coalesced away — the chain moves
// straight to the newest one. What must hold is that traffic never breaks and
// that the chain ends on the last selection applied.
func TestChainConcurrentPickAndRebuild(t *testing.T) {
	m := newManager(t)
	c := m.Chain(context.Background(), staticSource(eps("a", "b")...), "demo:service")

	stop := make(chan struct{})
	errs := make(chan error, 4)
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				ep, err := c.Pick(context.Background(), PickInfo{})
				if err != nil {
					errs <- err
					return
				}
				c.Complete(context.Background(), ep, nil)
			}
		})
	}

	for i := range 50 {
		m.Apply(context.Background(), Settings{Default: Selection{
			Balancer:          LeastConn,
			OutlierThreshold:  i,
			OutlierSuspendFor: time.Minute,
		}})
	}
	close(stop)
	wg.Wait()
	close(errs)

	for err := range errs {
		assert.Error(t, err).Nil()
	}
	assert.That(t, c.Generation() >= 2).True() // at least one rebuild happened
	assert.Number(t, c.Selection().OutlierThreshold).Equal(49)
}

// TestChainCoalescesUnobservedGenerations pins the property the pull design makes
// explicit: pushes landing between requests cost ONE rebuild, not one each — the
// chain moves straight to the generation in force, and the ones it never ran
// under are never built.
func TestChainCoalescesUnobservedGenerations(t *testing.T) {
	m := newManager(t)
	c := m.Chain(context.Background(), staticSource(eps("a")...), "demo:service")
	assert.Number(t, c.Generation()).Equal(uint64(1))

	for i := range 5 {
		m.Apply(context.Background(), Settings{Default: Selection{
			Balancer:          LeastConn,
			OutlierThreshold:  i,
			OutlierSuspendFor: time.Minute,
		}})
	}

	assert.Number(t, c.Generation()).Equal(uint64(2)) // five pushes, one rebuild
	assert.Number(t, c.Selection().OutlierThreshold).Equal(4)
}

// TestChainServesRealTrafficAcrossARebuild is the end-to-end proof that the
// rebuild path can actually carry requests: a real HTTP upstream behind a real
// HTTP gateway whose handler follows the documented request-path contract
// (snapshot once, pick, forward to the picked address, complete). It proves the
// three things a caller depends on — a request is served before any governance
// arrives, the same request path keeps serving on the other side of a rebuild,
// and the rebuild really did replace the generation.
func TestChainServesRealTrafficAcrossARebuild(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "from-upstream")
	}))
	defer upstream.Close()

	var hits atomic.Int64
	m := newManager(t)
	c := m.Chain(context.Background(),
		staticSource(discovery.Endpoint{Addr: upstream.Listener.Addr().String(), Healthy: true}),
		"demo:service",
	)

	// The gateway handler, wired exactly as the chain's egress documents: pick
	// through the chain, forward to the picked address, complete through the chain.
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ep, err := c.Pick(context.Background(), PickInfo{})
		if err != nil {
			c.Complete(r.Context(), ep, err)
			http.Error(w, "503 Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		hits.Add(1)
		req, err := http.NewRequestWithContext(r.Context(), r.Method,
			"http://"+ep.Addr+r.URL.Path, nil)
		if err != nil {
			c.Complete(r.Context(), ep, err)
			http.Error(w, "500", http.StatusInternalServerError)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			c.Complete(r.Context(), ep, err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(w, resp.Body)
		c.Complete(r.Context(), ep, nil)
	}))
	defer gw.Close()

	get := func() string {
		t.Helper()
		resp, err := http.Get(gw.URL + "/api/orders")
		assert.Error(t, err).Nil()
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		assert.Error(t, err).Nil()
		assert.Number(t, resp.StatusCode).Equal(200)
		return string(body)
	}

	// Before any governance arrives: the base chain already serves.
	assert.That(t, get()).Equal("from-upstream")

	// A push that moves this label's selection rebuilds the chain underneath the
	// live gateway; the next request must be served by the new generation.
	m.Apply(context.Background(), Settings{Default: Selection{Balancer: LeastConn, OutlierThreshold: 2}})

	assert.That(t, get()).Equal("from-upstream")
	assert.Number(t, c.Generation()).Equal(uint64(2))
	assert.Number(t, hits.Load()).Equal(int64(2))
}
