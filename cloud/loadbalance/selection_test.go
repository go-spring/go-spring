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

	"go-spring.org/stdlib/testing/assert"
)

// fakeProvider stands in for the governance-backed provider: it records the
// labels it was asked about, arms each subscriber immediately with a current
// Selection (as the real provider must), lets a test push a later change, and
// counts detaches so a stop path can be asserted.
type fakeProvider struct {
	mu       sync.Mutex
	cur      Selection
	labels   []string
	subs     []func(Selection)
	detached int
}

func (f *fakeProvider) provider() SelectionProvider {
	return func(label string, apply func(Selection)) func() {
		f.mu.Lock()
		f.labels = append(f.labels, label)
		i := len(f.subs)
		f.subs = append(f.subs, apply)
		cur := f.cur
		f.mu.Unlock()
		apply(cur)
		return func() {
			f.mu.Lock()
			defer f.mu.Unlock()
			f.subs[i] = nil
			f.detached++
		}
	}
}

func (f *fakeProvider) push(s Selection) {
	f.mu.Lock()
	subs := append([]func(Selection){}, f.subs...)
	f.mu.Unlock()
	for _, apply := range subs {
		if apply != nil {
			apply(s)
		}
	}
}

// installFake registers f as the process-wide provider and disarms it again when
// the test ends, so the seam never leaks into another test.
func installFake(t *testing.T, f *fakeProvider) {
	t.Helper()
	RegisterSelectionProvider(f.provider())
	t.Cleanup(func() { RegisterSelectionProvider(nil) })
}

// TestBindSelectionWithoutProvider pins the transparent pass-through: with no
// provider registered (governance absent from the process) a pool keeps the
// strategy it was built with, the returned stop is a no-op, and a double stop is
// harmless.
func TestBindSelectionWithoutProvider(t *testing.T) {
	RegisterSelectionProvider(nil)
	t.Cleanup(func() { RegisterSelectionProvider(nil) })

	p := NewPool(staticSource(eps("a", "b")), NewRoundRobin())
	stop := p.BindSelection("demo:resource")
	stop()
	stop()

	assert.That(t, p.Selection().Balancer).Equal("")
	ep, err := p.Pick(PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr == "a" || ep.Addr == "b").True()
}

// TestBindSelectionAppliesCurrentAndLaterChanges covers the provider contract
// end to end: the current policy lands before the first Pick (not only on the
// next push), a later push lands in place, and both the strategy name and the
// suspension thresholds are readable back off the pool.
func TestBindSelectionAppliesCurrentAndLaterChanges(t *testing.T) {
	f := &fakeProvider{cur: Selection{Balancer: LeastConn, OutlierThreshold: 2, OutlierSuspendFor: 30}}
	installFake(t, f)

	p := NewPool(staticSource(eps("a", "b")), NewRoundRobin(),
		WithTracker(NewTracker(TrackerConfig{})))
	stop := p.BindSelection("demo:resource")
	defer stop()

	assert.That(t, f.labels).Equal([]string{"demo:resource"})
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)
	assert.Number(t, p.Selection().OutlierThreshold).Equal(2)
	assert.Number(t, p.Tracker().Config().Threshold).Equal(2)

	f.push(Selection{Balancer: Weighted, OutlierThreshold: 5, OutlierSuspendFor: 60})
	assert.That(t, p.Selection().Balancer).Equal(Weighted)
	assert.Number(t, p.Tracker().Config().Threshold).Equal(5)
}

// TestBindSelectionStrategyTakesEffect proves the swap is real and not just
// bookkeeping: round-robin splits the picks evenly, while a pushed weighted
// policy makes the pool follow the 9:1 weights instead.
func TestBindSelectionStrategyTakesEffect(t *testing.T) {
	f := &fakeProvider{}
	installFake(t, f)

	src := staticSource{
		{Addr: "a", Healthy: true, Weight: 9},
		{Addr: "b", Healthy: true, Weight: 1},
	}
	p := NewPool(src, NewRoundRobin())
	stop := p.BindSelection("demo:resource")
	defer stop()

	counts := map[string]int{}
	for range 40 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		counts[ep.Addr]++
	}
	assert.Number(t, counts["a"]).Equal(20) // even split: the strategy is still round_robin

	f.push(Selection{Balancer: Weighted})
	counts = map[string]int{}
	for range 100 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		counts[ep.Addr]++
	}
	// The weighted strategy strongly favors "a"; an even split would be 50.
	assert.That(t, counts["a"] > 80).True()
}

// TestBindSelectionUnknownNameKeepsLastGood pins the degrade-don't-fail rule:
// the governance Source contract has no error channel, so a rule naming a
// strategy that does not exist leaves the last accepted one in force.
func TestBindSelectionUnknownNameKeepsLastGood(t *testing.T) {
	f := &fakeProvider{}
	installFake(t, f)

	p := NewPool(staticSource(eps("a")), NewRoundRobin())
	stop := p.BindSelection("demo:resource")
	defer stop()

	f.push(Selection{Balancer: LeastConn})
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)

	f.push(Selection{Balancer: "no_such_strategy"})
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)

	// An empty name leaves the strategy alone too, but still applies thresholds.
	f.push(Selection{OutlierThreshold: 3})
	assert.That(t, p.Selection().Balancer).Equal(LeastConn)
	assert.Number(t, p.Selection().OutlierThreshold).Equal(3)
}

// TestBindSelectionIsPerPoolNotPerLabel pins the non-memoized contract: one
// label may legitimately back several pools (two entries sharing a service
// name, a rebuilt client), so each bind subscribes independently and a change
// reaches every pool.
func TestBindSelectionIsPerPoolNotPerLabel(t *testing.T) {
	f := &fakeProvider{}
	installFake(t, f)

	p1 := NewPool(staticSource(eps("a")), NewRoundRobin())
	p2 := NewPool(staticSource(eps("a")), NewRoundRobin())
	stop1 := p1.BindSelection("demo:resource")
	stop2 := p2.BindSelection("demo:resource")
	defer stop2()

	assert.That(t, f.labels).Equal([]string{"demo:resource", "demo:resource"})

	f.push(Selection{Balancer: P2C})
	assert.That(t, p1.Selection().Balancer).Equal(P2C)
	assert.That(t, p2.Selection().Balancer).Equal(P2C)

	// Detaching one leaves the other live — a torn-down client must not silence
	// its still-running neighbour.
	stop1()
	assert.Number(t, f.detached).Equal(1)
	f.push(Selection{Balancer: LeastConn})
	assert.That(t, p1.Selection().Balancer).Equal(P2C)
	assert.That(t, p2.Selection().Balancer).Equal(LeastConn)
}

// TestRegisterSelectionProviderNilDisarms pins the testability escape hatch: a
// nil provider turns the seam back off, so a later bind is a no-op that does not
// even reach a provider.
func TestRegisterSelectionProviderNilDisarms(t *testing.T) {
	f := &fakeProvider{cur: Selection{Balancer: LeastConn}}
	installFake(t, f)

	p := NewPool(staticSource(eps("a")), NewRoundRobin())
	assert.That(t, p.BindSelection("demo:resource")).NotNil()

	RegisterSelectionProvider(nil)
	p2 := NewPool(staticSource(eps("a")), NewRoundRobin())
	p2.BindSelection("demo:resource")
	assert.Number(t, len(f.labels)).Equal(1)
	assert.That(t, p2.Selection().Balancer).Equal("")
}
