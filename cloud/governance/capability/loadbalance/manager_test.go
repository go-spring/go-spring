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

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// TestUnarmedResolvesBase pins the matcher's read path on an unarmed manager:
// before any document arrives every service resolves the base document, whose
// strategy is stated (round-robin) rather than left empty.
func TestUnarmedResolvesBase(t *testing.T) {
	m := newManager(t)
	assert.That(t, selectionFor(m, "demo:service")).Equal(Selection{Balancer: RoundRobin})
}

// TestSettingsMatch pins the matching this package owns: the first rule whose
// Service equals the service wins, a repeated service is resolved by list order, an
// empty Service never matches, and an unmatched service falls to the default.
func TestSettingsMatch(t *testing.T) {
	rule := Selection{Balancer: LeastConn, OutlierThreshold: 2}
	shadowed := Selection{Balancer: Weighted}
	def := Selection{Balancer: P2C}

	m := newManager(t)
	assert.Error(t, m.Apply(context.Background(), Settings{
		Selection: def,
		Rules: []Rule{
			{Service: "", Selection: shadowed},             // never matches
			{Service: "demo:service", Selection: rule},     // first wins
			{Service: "demo:service", Selection: shadowed}, // shadowed by order
		},
	})).Nil()

	assert.That(t, selectionFor(m, "demo:service")).Equal(rule)
	assert.That(t, selectionFor(m, "other:service")).Equal(def)
}

// TestUnservedRuleIsJustData pins the lazy reading: a rule for a service no chain
// serves is not built when the document is adopted. It is read when a chain asks
// for that service, and only then can it fail.
func TestUnservedRuleIsJustData(t *testing.T) {
	m := newManager(t)

	err := m.Apply(context.Background(), Settings{
		Selection: Selection{Balancer: RoundRobin},
		Rules:     []Rule{{Service: "unserved", Selection: Selection{Balancer: "no_such_strategy"}}},
	})
	assert.Error(t, err).Nil()

	_, err = m.Chain(context.Background(), staticSource(eps("a")...), "unserved")
	assert.Error(t, err).NotNil()
}

// ---------------------------------------------------------------------------
// Chain: the handle callers hold, and the generations the manager swaps under it
// ---------------------------------------------------------------------------

// errChainBoom is a request failure, for feeding the suspension tracker.
var errChainBoom = errors.New("boom")

// TestChainBuildsFromScratchOnChange covers the contract of the rebuild path: the
// first generation already runs the selection in force, and a push that moves
// this service's selection replaces the whole chain — a new pool, not the old one
// mutated.
func TestChainBuildsFromScratchOnChange(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: LeastConn, OutlierThreshold: 2, OutlierSuspendFor: 30}})

	c := mustChain(t, m, context.Background(), staticSource(eps("a", "b")...), "demo:service")

	// Built before any push, but with the selection already in force.
	base := c.cur.Load()
	assert.That(t, c.Selection().Balancer).Equal(LeastConn)
	assert.Number(t, c.Selection().OutlierThreshold).Equal(2)

	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: Weighted, OutlierThreshold: 5, OutlierSuspendFor: 60}})

	assert.That(t, c.cur.Load() != base).True() // a new pool, not the old one mutated
	assert.That(t, c.Selection().Balancer).Equal(Weighted)
	assert.Number(t, c.Selection().OutlierThreshold).Equal(5)
}

// TestChainRebuildDropsObservedFailures is what separates a Chain from a bound
// pool, stated as an observable fact: the old pool suspends an endpoint, the
// rebuild throws that observation away, and the new generation puts the endpoint
// straight back into rotation. A rebuild shares nothing with the generation it
// replaces — not even what it learned.
func TestChainRebuildDropsObservedFailures(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: RoundRobin, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}})

	c := mustChain(t, m, context.Background(), staticSource(eps("a", "b")...), "demo:service")
	base := c.cur.Load()

	PickResult{Endpoint: discovery.Endpoint{Addr: "a"}, pool: c.cur.Load()}.Complete(context.Background(), errChainBoom)
	assert.That(t, suspended(c.cur.Load().tracker, "a")).True()

	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: Random, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}})

	assert.That(t, c.cur.Load() != base).True()
	assert.That(t, suspended(c.cur.Load().tracker, "a")).False()

	// The endpoint is genuinely back: the new generation picks from the full set.
	pick, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, pick.Endpoint.Addr == "a" || pick.Endpoint.Addr == "b").True()
}

// TestChainRebuildClosesReplacedTracker pins the gauge accounting: the replaced
// generation's tracker is closed, so its contribution to the process-wide
// "currently suspended" UpDownCounter is returned instead of drifting upward
// across pushes.
func TestChainRebuildClosesReplacedTracker(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: RoundRobin, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}})

	c := mustChain(t, m, context.Background(), staticSource(eps("a", "b")...), "demo:service")
	old := c.cur.Load()
	PickResult{Endpoint: discovery.Endpoint{Addr: "a"}, pool: c.cur.Load()}.Complete(context.Background(), errChainBoom)
	assert.Number(t, old.tracker.suspendedCount()).Equal(1)

	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: Random, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}})

	assert.Number(t, old.tracker.suspendedCount()).Equal(0)
}

// TestChainCompleteSettlesOnThePickingGeneration guards the cross-generation
// bug: a rebuild landing between Pick and Complete must not move the outcome to
// the new generation. The retired generation drops it (it is gone by design),
// and the new one must not learn a failure it never saw.
func TestChainCompleteSettlesOnThePickingGeneration(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: RoundRobin, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}})

	c := mustChain(t, m, context.Background(), staticSource(eps("a")...), "demo:service")
	old := c.cur.Load()
	pick, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()

	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: Random, OutlierThreshold: 1, OutlierSuspendFor: time.Hour}})
	assert.That(t, c.cur.Load() != old).True()

	// The in-flight request settles after the swap: it belongs to the retired
	// generation and must not suspend "a" in the one that replaced it.
	pick.Complete(context.Background(), errChainBoom)

	assert.That(t, suspended(c.cur.Load().tracker, "a")).False()
	assert.Number(t, old.tracker.suspendedCount()).Equal(0)
}

// TestChainRebuildIsDrivenByTheChangedService pins that the diff answers "which
// service changed" rather than rebuilding everything: a rule scoped to one
// service moves only that service's chain, and re-applying an unchanged document
// moves nothing at all.
func TestChainRebuildIsDrivenByTheChangedService(t *testing.T) {
	m := newManager(t)

	a := mustChain(t, m, context.Background(), staticSource(eps("a")...), "svc:a")
	b := mustChain(t, m, context.Background(), staticSource(eps("b")...), "svc:b")
	aBase, bBase := a.cur.Load(), b.cur.Load()

	m.Apply(context.Background(), Settings{Rules: []Rule{{Service: "svc:b", Selection: Selection{Balancer: LeastConn}}}})

	assert.That(t, a.cur.Load() == aBase).True() // untouched: still the base generation
	assert.That(t, b.cur.Load() != bBase).True()
	assert.That(t, b.Selection().Balancer).Equal(LeastConn)

	// The same document again is not a change, so neither chain is rebuilt.
	m.Apply(context.Background(), Settings{Rules: []Rule{{Service: "svc:b", Selection: Selection{Balancer: LeastConn}}}})

	assert.That(t, a.cur.Load() == aBase).True()
	assert.That(t, b.cur.Load() != bBase).True()
}

// TestChainUnknownStrategyKeepsLastGood pins the atomic adopt rule: a document
// naming a strategy that does not exist is rejected whole, the last good
// generation stays in force and keeps serving.
func TestChainUnknownStrategyKeepsLastGood(t *testing.T) {
	m := newManager(t)
	c := mustChain(t, m, context.Background(), staticSource(eps("a")...), "demo:service")

	assert.Error(t, m.Apply(context.Background(), Settings{Selection: Selection{Balancer: LeastConn}})).Nil()
	assert.That(t, c.Selection().Balancer).Equal(LeastConn)
	good := c.cur.Load()

	err := m.Apply(context.Background(), Settings{Selection: Selection{Balancer: "no_such_strategy"}})
	assert.Error(t, err).NotNil()

	assert.That(t, c.cur.Load() == good).True() // the rejected document committed nothing
	pick, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, pick.Endpoint.Addr).Equal("a")
}

// TestChainRejectsASecondRegistration pins the one-service-one-client rule: two
// clients on one governance label would fork the selection state and only the
// first source would ever be consulted, so a second registration fails at wiring
// instead of being stored as another entry.
func TestChainRejectsASecondRegistration(t *testing.T) {
	m := newManager(t)

	mustChain(t, m, context.Background(), staticSource(eps("a")...), "demo:service")

	_, err := m.Chain(context.Background(), staticSource(eps("b")...), "demo:service")
	assert.Error(t, err).Matches("already has a chain")
}

// TestChainSwapIsLive pins the egress contract: after a governance change, a
// fresh chain-level Pick/Complete pair runs against the new generation, and a
// rebuild never leaves the chain without one.
func TestChainSwapIsLive(t *testing.T) {
	m := newManager(t)
	c := mustChain(t, m, context.Background(), staticSource(eps("a")...), "demo:service")
	base := c.cur.Load()

	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: LeastConn}})
	assert.That(t, c.cur.Load() != base).True()

	pick, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	pick.Complete(context.Background(), nil)

	assert.That(t, c.Selection().Balancer).Equal(LeastConn)
}

// TestChainConcurrentPickAndRebuild is the race guard for the atomic swap: picks
// run while pushes move the chain underneath them. What must hold is that
// traffic never breaks, that at least one rebuild happened, and that the chain
// ends on the last selection applied.
func TestChainConcurrentPickAndRebuild(t *testing.T) {
	m := newManager(t)
	c := mustChain(t, m, context.Background(), staticSource(eps("a", "b")...), "demo:service")
	base := c.cur.Load()

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
				pick, err := c.Pick(context.Background(), PickInfo{})
				if err != nil {
					errs <- err
					return
				}
				pick.Complete(context.Background(), nil)
			}
		})
	}

	for i := range 50 {
		m.Apply(context.Background(), Settings{Selection: Selection{
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
	assert.That(t, c.cur.Load() != base).True() // at least one rebuild happened
	assert.Number(t, c.Selection().OutlierThreshold).Equal(49)
}

// TestChainRebuildsOnThePushNotTheRequest pins the push-side property of the
// eager model: every push that moves this service's selection rebuilds the chain
// RIGHT THERE — no request needed to observe the change — while a push whose
// content leaves the service where it was rebuilds nothing.
func TestChainRebuildsOnThePushNotTheRequest(t *testing.T) {
	m := newManager(t)
	c := mustChain(t, m, context.Background(), staticSource(eps("a")...), "demo:service")
	base := c.cur.Load()

	for i := range 5 {
		m.Apply(context.Background(), Settings{Selection: Selection{
			Balancer:          LeastConn,
			OutlierThreshold:  i,
			OutlierSuspendFor: time.Minute,
		}})
	}

	assert.That(t, c.cur.Load() != base).True() // the pushes rebuilt the chain on the spot
	moved := c.cur.Load()
	assert.Number(t, c.Selection().OutlierThreshold).Equal(4)

	// A re-push of the same document moves nothing, so nothing is rebuilt.
	m.Apply(context.Background(), Settings{Selection: Selection{
		Balancer:          LeastConn,
		OutlierThreshold:  4,
		OutlierSuspendFor: time.Minute,
	}})
	assert.That(t, c.cur.Load() == moved).True()
}

// TestChainSelectorNarrowsCandidates pins that a selection's selector filters
// the candidate set before the strategy picks: only matching endpoints are ever
// chosen.
func TestChainSelectorNarrowsCandidates(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: RoundRobin, Selector: versionPrefix("v2.")}})

	c := mustChain(t, m, context.Background(), versionedSource(), "svc:x")
	for range 4 {
		pick, err := c.Pick(context.Background(), PickInfo{})
		assert.Error(t, err).Nil()
		assert.That(t, pick.Endpoint.Addr).Equal("a")
	}
	assert.Number(t, len(c.Selection().Selector)).Equal(1)
}

// TestChainSelectorChangeRebuilds pins the dynamic half: a pushed selector is an
// ordinary selection change, so it moves the service's generation and the rebuilt
// pool picks from the newly narrowed set — no redeploy, no in-place mutation.
func TestChainSelectorChangeRebuilds(t *testing.T) {
	m := newManager(t)
	c := mustChain(t, m, context.Background(), versionedSource(), "svc:x")
	base := c.cur.Load()

	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: RoundRobin, Selector: versionPrefix("v2.")}})
	assert.That(t, c.cur.Load() != base).True()
	pick, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, pick.Endpoint.Addr).Equal("a")

	narrowed := c.cur.Load()
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: RoundRobin, Selector: versionPrefix("v1.")}})
	assert.That(t, c.cur.Load() != narrowed).True()
	pick, err = c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, pick.Endpoint.Addr).Equal("b")
}

// TestChainSelectorMatchingNothingFailsLoudly pins the no-fallback decision: a
// rule that matches no endpoint surfaces ErrNoEndpoints instead of silently
// widening to every instance.
func TestChainSelectorMatchingNothingFailsLoudly(t *testing.T) {
	m := newManager(t)
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: RoundRobin, Selector: versionPrefix("v9.")}})

	c := mustChain(t, m, context.Background(), versionedSource(), "svc:x")
	_, err := c.Pick(context.Background(), PickInfo{})
	assert.Error(t, err).Is(ErrNoEndpoints)
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
	c := mustChain(t, m, context.Background(),
		staticSource(discovery.Endpoint{Addr: upstream.Listener.Addr().String(), Healthy: true}),
		"demo:service",
	)
	base := c.cur.Load()

	// The gateway handler, wired exactly as the chain's egress documents: pick
	// through the chain, forward to the picked address, complete through the chain.
	gw := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pick, err := c.Pick(context.Background(), PickInfo{})
		if err != nil {
			http.Error(w, "503 Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		hits.Add(1)
		req, err := http.NewRequestWithContext(r.Context(), r.Method,
			"http://"+pick.Endpoint.Addr+r.URL.Path, nil)
		if err != nil {
			pick.Complete(r.Context(), err)
			http.Error(w, "500", http.StatusInternalServerError)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			pick.Complete(r.Context(), err)
			http.Error(w, "502 Bad Gateway", http.StatusBadGateway)
			return
		}
		defer func() { _ = resp.Body.Close() }()
		_, _ = io.Copy(w, resp.Body)
		pick.Complete(r.Context(), nil)
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

	// A push that moves this service's selection rebuilds the chain underneath the
	// live gateway; the next request must be served by the new generation.
	m.Apply(context.Background(), Settings{Selection: Selection{Balancer: LeastConn, OutlierThreshold: 2}})

	assert.That(t, get()).Equal("from-upstream")
	assert.That(t, c.cur.Load() != base).True()
	assert.Number(t, hits.Load()).Equal(int64(2))
}

// TestApplyRejectsWholeDocument pins the adopt entry point: a document that
// names an unregistered strategy or parameters the strategy refuses is rejected
// whole — for the default and for every rule — and never becomes the settings in
// force. Creation is the check; nothing is committed on failure.
func TestApplyRejectsWholeDocument(t *testing.T) {
	m := newManager(t)
	src := staticSource(eps("a")...)
	ctx := context.Background()
	mustChain(t, m, ctx, src, "redis:cache")
	mustChain(t, m, ctx, src, "gorm:db")
	mustChain(t, m, ctx, src, "x")

	// A well-formed settings: the base strategy stated in the default, plus rules
	// that name real strategies with the parameters those strategies own.
	ok := Settings{
		Selection: Selection{Balancer: RoundRobin},
		Rules: []Rule{
			{Service: "redis:cache", Selection: Selection{Balancer: LeastConn}},
			{Service: "gorm:db", Selection: Selection{
				Balancer:               ConsistentHash,
				ConsistentHashReplicas: 200,
			}},
		},
	}
	assert.Error(t, m.Apply(ctx, ok)).Nil()

	// An unregistered name in the DEFAULT: every service without a rule resolves
	// to it, so a chain that serves one rejects the whole document.
	badDefault := Settings{Selection: Selection{Balancer: "no_such_strategy"}}
	assert.Error(t, m.Apply(ctx, badDefault)).Matches("no strategy registered")

	// An unregistered name in a RULE — the default being fine must not mask it.
	badRule := Settings{
		Selection: Selection{Balancer: RoundRobin},
		Rules:     []Rule{{Service: "redis:cache", Selection: Selection{Balancer: "no_such_strategy"}}},
	}
	assert.Error(t, m.Apply(ctx, badRule)).Matches("no strategy registered")

	// A rule that names no strategy is not an error: resolution falls through
	// to the global base strategy.
	assert.Error(t, m.Apply(ctx, Settings{
		Selection: Selection{Balancer: RoundRobin},
		Rules:     []Rule{{Service: "x"}},
	})).Nil()
}

// TestEmptyDefaultMeansRoundRobin pins the base default: a document that leaves
// selection alone is valid and arms round-robin, rather than failing because no
// strategy was stated.
func TestEmptyDefaultMeansRoundRobin(t *testing.T) {
	m := newManager(t)
	assert.Error(t, m.Apply(context.Background(), Settings{})).Nil()
	assert.That(t, selectionFor(m, "redis:cache").Balancer).Equal(RoundRobin)
}
