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
	"errors"
	"strconv"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// eps builds a plain endpoint slice from addresses.
func eps(addrs ...string) []discovery.Endpoint {
	out := make([]discovery.Endpoint, len(addrs))
	for i, a := range addrs {
		out[i] = discovery.Endpoint{Addr: a}
	}
	return out
}

// addrs extracts the addresses from an endpoint slice (Endpoint is not
// comparable — it holds a map — so slice assertions run over addresses).
func addrs(eps []discovery.Endpoint) []string {
	out := make([]string, len(eps))
	for i, ep := range eps {
		out[i] = ep.Addr
	}
	return out
}

// counts tallies how many picks landed on each address over n calls.
func counts(t *testing.T, b Balancer, set []discovery.Endpoint, info PickInfo, n int) map[string]int {
	t.Helper()
	m := map[string]int{}
	for range n {
		ep, err := b.Pick(set, info)
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		b.Complete(ep, nil)
	}
	return m
}

func TestRoundRobin(t *testing.T) {
	b := NewRoundRobin()
	m := counts(t, b, eps("a", "b", "c"), PickInfo{}, 9)
	assert.Number(t, m["a"]).Equal(3)
	assert.Number(t, m["b"]).Equal(3)
	assert.Number(t, m["c"]).Equal(3)

	_, err := b.Pick(nil, PickInfo{})
	assert.Error(t, err).Is(ErrNoAvailable)
}

func TestWeighted(t *testing.T) {
	set := []discovery.Endpoint{
		{Addr: "a", Weight: 5},
		{Addr: "b", Weight: 1},
		{Addr: "c", Weight: 1},
	}
	b := NewWeighted()
	m := counts(t, b, set, PickInfo{}, 7)
	// Over one full cycle (sum of weights = 7) each endpoint gets exactly its
	// weight.
	assert.Number(t, m["a"]).Equal(5)
	assert.Number(t, m["b"]).Equal(1)
	assert.Number(t, m["c"]).Equal(1)

	// Smoothness: the 5-weight endpoint must not take all 5 slots up front —
	// b and c are interleaved within the cycle, so a never runs 5 in a row.
	b2 := NewWeighted()
	var seq []string
	for range 7 {
		ep, _ := b2.Pick(set, PickInfo{})
		seq = append(seq, ep.Addr)
	}
	run := 0
	maxRun := 0
	for i, s := range seq {
		if i > 0 && s == seq[i-1] {
			run++
		} else {
			run = 0
		}
		if run > maxRun {
			maxRun = run
		}
	}
	assert.Number(t, maxRun).LessThan(2) // no 3-in-a-row for the same endpoint

	// Zero weight is treated as weight 1.
	b3 := NewWeighted()
	m3 := counts(t, b3, eps("x", "y"), PickInfo{}, 4)
	assert.Number(t, m3["x"]).Equal(2)
	assert.Number(t, m3["y"]).Equal(2)
}

func TestLeastConn(t *testing.T) {
	b := NewLeastConn()
	set := eps("a", "b")

	// Two picks without releasing: each must land on a different endpoint since
	// the first one is now at in-flight 1.
	e1, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	e2, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e1.Addr).NotEqual(e2.Addr)

	// Release e1; it now has the fewest in-flight and must be chosen.
	b.Complete(e1, nil)
	e3, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e3.Addr).Equal(e1.Addr)

	// A repeated Complete for the same request is a harmless no-op: the count
	// deletes at zero instead of going negative (interface contract).
	b.Complete(e1, nil)
	b.Complete(e1, nil)
	e4, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e4.Addr).Equal(e1.Addr) // e1's count was not driven negative
}

func TestConsistentHash(t *testing.T) {
	b := NewConsistentHash(50)
	set := eps("a", "b", "c")

	// Same key is stable across repeated picks.
	first, err := b.Pick(set, PickInfo{HashKey: "user-42"})
	assert.Error(t, err).Nil()
	for range 20 {
		ep, err := b.Pick(set, PickInfo{HashKey: "user-42"})
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal(first.Addr)
	}

	// Empty key falls back to round-robin spread rather than one instance.
	m := counts(t, b, set, PickInfo{}, 9)
	assert.Number(t, len(m)).Equal(3)
}

func TestZoneAware(t *testing.T) {
	set := []discovery.Endpoint{
		{Addr: "a", Metadata: map[string]string{"zone": "z1"}},
		{Addr: "b", Metadata: map[string]string{"zone": "z1"}},
		{Addr: "c", Metadata: map[string]string{"zone": "z2"}},
	}
	b := NewZoneAware("zone", NewRoundRobin())

	// Requests from z1 stay in z1.
	m := counts(t, b, set, PickInfo{Zone: "z1"}, 20)
	assert.Number(t, m["c"]).Equal(0)
	assert.Number(t, m["a"]+m["b"]).Equal(20)

	// A zone with no local instances spills over to the whole set.
	m2 := counts(t, b, set, PickInfo{Zone: "z9"}, 3)
	assert.Number(t, m2["a"]+m2["b"]+m2["c"]).Equal(3)

	// No zone hint delegates over everything.
	m3 := counts(t, b, set, PickInfo{}, 3)
	assert.Number(t, m3["a"]+m3["b"]+m3["c"]).Equal(3)
}

func TestZoneAwareCompleteForwards(t *testing.T) {
	// Complete must reach the delegate so a stateful strategy composed under
	// zone_aware (least-conn here) keeps its per-request accounting.
	b := NewZoneAware("zone", NewLeastConn())
	set := []discovery.Endpoint{
		{Addr: "a", Metadata: map[string]string{"zone": "z1"}},
		{Addr: "b", Metadata: map[string]string{"zone": "z1"}},
	}
	e1, err := b.Pick(set, PickInfo{Zone: "z1"})
	assert.Error(t, err).Nil()
	e2, err := b.Pick(set, PickInfo{Zone: "z1"})
	assert.Error(t, err).Nil()
	assert.String(t, e1.Addr).NotEqual(e2.Addr) // in-flight count survived the wrap

	b.Complete(e1, nil)
	e3, err := b.Pick(set, PickInfo{Zone: "z1"})
	assert.Error(t, err).Nil()
	assert.String(t, e3.Addr).Equal(e1.Addr) // decrement reached the delegate
}

func TestZoneAwareLevels(t *testing.T) {
	set := []discovery.Endpoint{
		{Addr: "rack", Metadata: map[string]string{"zone": "cn-north-1a"}},
		{Addr: "az", Metadata: map[string]string{"zone": "cn-north-1b"}},
		{Addr: "region", Metadata: map[string]string{"zone": "cn-north-2"}},
	}
	b := NewZoneAware("zone", NewRoundRobin())

	// Exact first level wins: only the rack-local instance is picked.
	m := counts(t, b, set, PickInfo{Zone: "cn-north-1a"}, 5)
	assert.Number(t, m["rack"]).Equal(5)

	// Hierarchical fallback: the az-level hint matches same-AZ endpoints
	// (1a, 1b) but never the sibling region cn-north-2.
	m = counts(t, b, set, PickInfo{Zone: "cn-north-1"}, 6)
	assert.Number(t, m["rack"]+m["az"]).Equal(6)
	assert.Number(t, m["region"]).Equal(0)

	// Ordered multi-level: rack first; with the rack instance gone the az
	// level takes over; only when both are empty does it spill everywhere.
	subset := set[1:]
	m = counts(t, b, subset, PickInfo{Zone: "cn-north-1a,cn-north-1"}, 4)
	assert.Number(t, m["az"]).Equal(4)
	m = counts(t, b, subset, PickInfo{Zone: "cn-north-1a,cn-north-2"}, 4)
	assert.Number(t, m["region"]).Equal(4)
	m = counts(t, b, []discovery.Endpoint{set[2]}, PickInfo{Zone: "cn-north-1a,cn-north-1"}, 3)
	assert.Number(t, m["region"]).Equal(3)

	// All levels empty: full spill-over, never a black hole.
	m = counts(t, b, set, PickInfo{Zone: "cn-south-1,cn-south-2"}, 3)
	assert.Number(t, len(m)).Equal(3)

	// The classic single-zone usage is unchanged by the list support.
	m = counts(t, b, set, PickInfo{Zone: "cn-north-2"}, 3)
	assert.Number(t, m["region"]).Equal(3)
}

func TestZoneLevels(t *testing.T) {
	assert.That(t, len(parseZoneLevels(""))).Equal(0)
	assert.Slice(t, parseZoneLevels("cn-north-1a")).Length(1)
	assert.Slice(t, parseZoneLevels(" cn-north-1a , cn-north-1 ")).Length(2)
	assert.Slice(t, parseZoneLevels("a,,b")).Length(2)
	assert.That(t, zoneMatch("cn-north-1a", "cn-north-1a")).True()
	assert.That(t, zoneMatch("cn-north-1a", "cn-north-1")).True()
	assert.That(t, zoneMatch("cn-north-2", "cn-north-1")).False()
	assert.That(t, zoneMatch("cn-north-12", "cn-north-1")).True() // documented hierarchy edge
	assert.That(t, zoneMatch("", "cn-north-1")).False()
}

func TestRegistry(t *testing.T) {
	for _, name := range []string{RoundRobin, LeastConn, ConsistentHash, Weighted, ZoneAware, Random, P2C} {
		b, err := New(name)
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}

	_, err := New("does-not-exist")
	assert.Error(t, err).Matches("no strategy registered")

	assert.Panic(t, func() { Register("", func() Balancer { return nil }) }, "empty name")
	assert.Panic(t, func() { Register("x", nil) }, "nil factory")
	assert.Panic(t, func() { Register(RoundRobin, NewRoundRobin) }, "already registered")
}

func TestTrackerSuspendAndRecover(t *testing.T) {
	now := time.Unix(0, 0)
	tr := NewTracker(TrackerConfig{Threshold: 2, SuspendFor: time.Second})
	tr.now = func() time.Time { return now }

	// One failure is below threshold: still eligible.
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).False()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)

	// Second consecutive failure trips suspension.
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).True()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"b"})

	// Still cooling down before SuspendFor elapses.
	now = now.Add(500 * time.Millisecond)
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"b"})

	// Cool-down elapsed: a half-open trial admits "a" again.
	now = now.Add(600 * time.Millisecond)
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)

	// A successful trial fully restores it.
	tr.Record("a", true)
	assert.That(t, tr.Suspended("a")).False()

	// If the trial fails instead, it re-suspends for another window.
	tr.Record("a", false)
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).True()
	now = now.Add(1100 * time.Millisecond)
	tr.Allows(eps("a"))   // admit trial -> half-open
	tr.Record("a", false) // trial fails
	assert.That(t, tr.Suspended("a")).True()
}

func TestTrackerDisabled(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 0})
	tr.Record("a", false)
	tr.Record("a", false)
	tr.Record("a", false)
	assert.That(t, tr.Suspended("a")).False()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Length(2)
}

func TestTrackerAllSuspendedFallsBack(t *testing.T) {
	// Black-holing all traffic is worse than probing a degraded instance: when
	// every endpoint is suspended, Allows returns its input unchanged.
	now := time.Unix(0, 0)
	tr := NewTracker(TrackerConfig{Threshold: 1, SuspendFor: time.Minute})
	tr.now = func() time.Time { return now }
	tr.Record("a", false)
	tr.Record("b", false)
	assert.That(t, tr.Suspended("a")).True()
	assert.That(t, tr.Suspended("b")).True()
	assert.Slice(t, addrs(tr.Allows(eps("a", "b")))).Equal([]string{"a", "b"})
}

// staticSource is a fixed EndpointSource for pool tests.
type staticSource []discovery.Endpoint

func (s staticSource) Endpoints() []discovery.Endpoint { return s }

func TestPoolHealthFilter(t *testing.T) {
	src := staticSource{
		{Addr: "a", Healthy: true},
		{Addr: "b", Healthy: false},
		{Addr: "c", Healthy: true},
	}
	p := NewPool(src, NewRoundRobin())
	m := map[string]int{}
	for range 20 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		p.Complete(ep, nil)
	}
	// The unhealthy instance is never picked.
	assert.Number(t, m["b"]).Equal(0)
	assert.Number(t, m["a"]+m["c"]).Equal(20)
}

func TestPoolEvictionViaComplete(t *testing.T) {
	src := staticSource(eps("a", "b"))
	tr := NewTracker(TrackerConfig{Threshold: 2, SuspendFor: time.Minute})
	p := NewPool(src, NewRoundRobin(), WithTracker(tr))

	// Drive "a" to failure through the pool's Complete wiring, twice, to suspend it.
	for range 5 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		if ep.Addr == "a" {
			p.Complete(ep, errors.New("boom"))
		} else {
			p.Complete(ep, nil)
		}
	}
	assert.That(t, tr.Suspended("a")).True()

	// Subsequent picks avoid the evicted instance.
	for range 10 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal("b")
		p.Complete(ep, nil)
	}
}

func TestPoolEmpty(t *testing.T) {
	p := NewPool(staticSource(nil), NewRoundRobin())
	_, err := p.Pick(PickInfo{})
	assert.Error(t, err).Is(ErrNoAvailable)
}

func TestPoolWithoutTracker(t *testing.T) {
	// No WithTracker: the nil tracker is a transparent pass-through (nil-receiver
	// methods), and Complete with failures must not suspend anything.
	src := staticSource(eps("a", "b"))
	p := NewPool(src, NewRoundRobin())
	for range 10 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		p.Complete(ep, errors.New("boom")) // failures, but no tracker attached
	}
	// Both endpoints keep receiving traffic.
	m := map[string]int{}
	for range 10 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		p.Complete(ep, nil)
	}
	assert.Number(t, m["a"]).Equal(5)
	assert.Number(t, m["b"]).Equal(5)
}

func TestPoolZeroWeightDrains(t *testing.T) {
	// A zero weight (the drain signal) removes the instance from picking,
	// across every strategy — here round-robin, which otherwise ignores weight.
	src := staticSource{
		{Addr: "a", Healthy: true, Weight: 1},
		{Addr: "b", Healthy: true, Weight: 0},
	}
	p := NewPool(src, NewRoundRobin())
	for range 10 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal("a")
		p.Complete(ep, nil)
	}
}

func TestPoolAllZeroWeightFallsBack(t *testing.T) {
	// Every endpoint zero-weighted (an unnormalized snapshot from registrants
	// predating the weight contract): fall back to an even split rather than
	// blackholing the pool.
	src := staticSource{
		{Addr: "a", Healthy: true},
		{Addr: "b", Healthy: true},
	}
	p := NewPool(src, NewRoundRobin())
	m := map[string]int{}
	for range 20 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		p.Complete(ep, nil)
	}
	assert.Number(t, m["a"]+m["b"]).Equal(20)
	assert.Number(t, m["a"]).Equal(10)
	assert.Number(t, m["b"]).Equal(10)
}

func TestPoolNegativeWeightKept(t *testing.T) {
	// Negative weight is misconfiguration, not a drain signal: the instance
	// stays in rotation (the weighted balancer treats it as the default).
	src := staticSource{
		{Addr: "a", Healthy: true, Weight: -1},
		{Addr: "b", Healthy: true, Weight: 0},
	}
	p := NewPool(src, NewRoundRobin())
	for range 10 {
		ep, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal("a")
		p.Complete(ep, nil)
	}
}

func TestConsistentHashTopologyChange(t *testing.T) {
	// Adding an endpoint must move only a small fraction of keys (the whole
	// point of consistent hashing): with 4 endpoints joining a 3-endpoint set
	// (~4/7 of traffic should shift in expectation), the vast majority of
	// 200 keys stay put.
	b := NewConsistentHash(100)
	before := map[string]string{}
	keys := make([]string, 0, 200)
	for i := range 200 {
		k := "key-" + strconv.Itoa(i)
		keys = append(keys, k)
		ep, err := b.Pick(eps("a", "b", "c"), PickInfo{HashKey: k})
		assert.Error(t, err).Nil()
		before[k] = ep.Addr
	}
	moved := 0
	for _, k := range keys {
		ep, err := b.Pick(eps("a", "b", "c", "d", "e", "f", "g"), PickInfo{HashKey: k})
		assert.Error(t, err).Nil()
		if ep.Addr != before[k] {
			moved++
		}
	}
	// Expected movement is 3/7 (~86 of 200) with some slack; a modulo-based
	// scheme would move ~100%.
	assert.Number(t, moved).LessThan(120)
	assert.Number(t, moved).GreaterThan(40)
}

func TestFingerprintOrderIndependent(t *testing.T) {
	a := eps("a", "b", "c")
	b := eps("c", "a", "b")
	assert.String(t, fingerprint(a)).Equal(fingerprint(b))
	assert.String(t, fingerprint(a)).NotEqual(fingerprint(eps("a", "b")))
}
