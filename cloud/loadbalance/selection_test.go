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
	"sync"
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// fakeSource stands in for the governance center's rule document: it records the
// labels it was asked to resolve and lets a test push a later change through
// [Manager.Apply], which is the only entry point the center uses.
type fakeSource struct {
	mu     sync.Mutex
	cur    Selection
	labels []string
}

func (f *fakeSource) resolve(label string) Selection {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.labels = append(f.labels, label)
	return f.cur
}

// manager returns a Manager armed against this source.
func (f *fakeSource) manager() *Manager {
	m := NewManager()
	m.Apply(Settings{Enabled: true, Resolve: f.resolve})
	return m
}

// push adopts s as the source's current selection and re-applies, delivering the
// change to every bound pool.
func (f *fakeSource) push(m *Manager, s Selection) {
	f.mu.Lock()
	f.cur = s
	f.mu.Unlock()
	m.Apply(Settings{Enabled: true, Resolve: f.resolve})
}

func (f *fakeSource) labelCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.labels)
}

// TestBindWithoutManager pins the transparent pass-through: with an unarmed
// manager (governance absent from the process) a pool keeps the strategy it was
// built with, the returned stop is a no-op, and a double stop is harmless.
func TestBindWithoutManager(t *testing.T) {
	m := NewManager()

	p := NewPool(staticSource(eps("a", "b")...), NewRoundRobin())
	stop := m.Bind(p, "demo:service")
	stop()
	stop()

	assert.That(t, p.Selection().Balancer).Equal("")
	ep, err := p.Pick(PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr == "a" || ep.Addr == "b").True()
}

// TestBindAppliesCurrentAndLaterChanges covers the contract end to end: the
// current selection lands before the first Pick (not only on the next push), a
// later push lands in place, and both the strategy name and the suspension
// thresholds are readable back off the pool.
func TestBindAppliesCurrentAndLaterChanges(t *testing.T) {
	f := &fakeSource{cur: Selection{Balancer: LeastConn, OutlierThreshold: 2, OutlierSuspendFor: 30}}
	m := f.manager()

	p := NewPool(staticSource(eps("a", "b")...), NewRoundRobin())
	stop := m.Bind(p, "demo:service")
	defer stop()

	assert.That(t, p.Selection().Balancer).Equal(LeastConn)
	assert.Number(t, p.Selection().OutlierThreshold).Equal(2)
	assert.Number(t, p.Tracker().Config().Threshold).Equal(2)

	f.push(m, Selection{Balancer: Weighted, OutlierThreshold: 5, OutlierSuspendFor: 60})
	assert.That(t, p.Selection().Balancer).Equal(Weighted)
	assert.Number(t, p.Tracker().Config().Threshold).Equal(5)
}

// TestBindStrategyTakesEffect proves the swap is real and not just bookkeeping:
// round-robin splits the picks evenly, while a pushed weighted selection makes
// the pool follow the 9:1 weights instead.
func TestBindStrategyTakesEffect(t *testing.T) {
	f := &fakeSource{}
	m := f.manager()

	src := staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true, Weight: 9},
		discovery.Endpoint{Addr: "b", Healthy: true, Weight: 1},
	)
	p := NewPool(src, NewRoundRobin())
	stop := m.Bind(p, "demo:service")
	defer stop()

	counts := map[string]int{}
	for range 40 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		counts[ep.Addr]++
	}
	assert.Number(t, counts["a"]).Equal(20) // even split: the strategy is still round_robin

	f.push(m, Selection{Balancer: Weighted})
	counts = map[string]int{}
	for range 100 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		counts[ep.Addr]++
	}
	// The weighted strategy strongly favors "a"; an even split would be 50.
	assert.That(t, counts["a"] > 80).True()
}

// TestBindUnknownNameKeepsLastGood pins the degrade-don't-fail rule: the
// governance Source contract has no error channel, so a rule naming a strategy
// that does not exist leaves the last accepted one in force.
func TestBindUnknownNameKeepsLastGood(t *testing.T) {
	f := &fakeSource{}
	m := f.manager()

	p := NewPool(staticSource(eps("a")...), NewRoundRobin())
	stop := m.Bind(p, "demo:service")
	defer stop()

	f.push(m, Selection{Balancer: LeastConn})
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)

	f.push(m, Selection{Balancer: "no_such_strategy"})
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)

	// An empty name leaves the strategy alone too, but still applies thresholds.
	f.push(m, Selection{OutlierThreshold: 3})
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)
	assert.Number(t, p.Selection().OutlierThreshold).Equal(3)
}

// TestBindIsPerPoolNotPerLabel pins the non-memoized contract: one label may
// legitimately back several pools (two entries sharing a service name, a rebuilt
// client), so each bind subscribes independently and a change reaches every pool.
func TestBindIsPerPoolNotPerLabel(t *testing.T) {
	f := &fakeSource{}
	m := f.manager()

	p1 := NewPool(staticSource(eps("a")...), NewRoundRobin())
	p2 := NewPool(staticSource(eps("a")...), NewRoundRobin())
	stop1 := m.Bind(p1, "demo:service")
	stop2 := m.Bind(p2, "demo:service")
	defer stop2()

	f.push(m, Selection{Balancer: P2C})
	assert.That(t, p1.Selection().Balancer).Equal(P2C)
	assert.That(t, p2.Selection().Balancer).Equal(P2C)

	// Detaching one leaves the other live — a torn-down client must not silence
	// its still-running neighbour.
	stop1()
	f.push(m, Selection{Balancer: LeastConn})
	assert.That(t, p1.Selection().Balancer).Equal(P2C)
	assert.That(t, p2.Selection().Balancer).Equal(LeastConn)
}

// TestApplyDisabledDisarms pins that a disabled settings switches the manager
// back off, so a later bind is a no-op that does not even consult the resolver.
func TestApplyDisabledDisarms(t *testing.T) {
	f := &fakeSource{cur: Selection{Balancer: LeastConn}}
	m := f.manager()

	p := NewPool(staticSource(eps("a")...), NewRoundRobin())
	stop := m.Bind(p, "demo:service")
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)
	stop()
	before := f.labelCount()

	m.Apply(Settings{Enabled: false, Resolve: f.resolve})
	p2 := NewPool(staticSource(eps("a")...), NewRoundRobin())
	p2stop := m.Bind(p2, "demo:service")
	p2stop()

	assert.Number(t, f.labelCount()).Equal(before) // the resolver was never consulted
	assert.That(t, p2.Selection().Balancer).Equal("")
	assert.That(t, m.Enabled()).False()
}

// TestSelectionForUnarmedIsZero pins the read path on an unarmed manager.
func TestSelectionForUnarmedIsZero(t *testing.T) {
	m := NewManager()
	assert.That(t, m.SelectionFor("demo:service")).Equal(Selection{})
	assert.That(t, m.Enabled()).False()
}

// TestBindBeforeArmingSurvives is the regression guard for a silent loss of the
// binding: a pool is built during bean CONSTRUCTION, which runs before the
// wiring bean arms the center (Centre.GoLive is an Init hook). A subscription
// taken at that moment must be REMEMBERED and armed once the manager is armed —
// dropping it would leave the pool unmanaged for the life of the process, with
// nothing to indicate it.
func TestBindBeforeArmingSurvives(t *testing.T) {
	m := NewManager() // unarmed: this is the state at construction time
	pool := NewPool(staticSource(eps("a")...), NewRoundRobin())

	stop := m.Bind(pool, "demo:service")
	defer stop()
	// Unarmed: the pool keeps the strategy it was built with.
	assert.That(t, pool.Selection().Balancer).Equal("")

	// The center goes live and arms the manager.
	f := &fakeSource{cur: Selection{Balancer: LeastConn, OutlierThreshold: 2, OutlierSuspendFor: 30}}
	m.Apply(Settings{Enabled: true, Resolve: f.resolve})

	// The binding taken before arming must now be live.
	assert.That(t, pool.Selection().Balancer).Equal(LeastConn)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(2)

	// ...and must keep following later changes.
	f.push(m, Selection{Balancer: Weighted, OutlierThreshold: 5, OutlierSuspendFor: 60})
	assert.That(t, pool.Selection().Balancer).Equal(Weighted)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(5)
}

// TestBindWhileDisabledArmsOnEnable is the same guard across a disable/enable
// cycle: a manager that is explicitly disabled still remembers subscriptions, so
// switching governance back on reaches the pools that bound while it was off.
func TestBindWhileDisabledArmsOnEnable(t *testing.T) {
	m := NewManager()
	m.Apply(Settings{Enabled: false})

	pool := NewPool(staticSource(eps("a")...), NewRoundRobin())
	stop := m.Bind(pool, "demo:service")
	defer stop()
	assert.That(t, pool.Selection().Balancer).Equal("")

	f := &fakeSource{cur: Selection{Balancer: P2C}}
	m.Apply(Settings{Enabled: true, Resolve: f.resolve})
	assert.That(t, pool.Selection().Balancer).Equal(P2C)
}
