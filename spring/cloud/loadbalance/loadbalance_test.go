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
	"testing"
	"time"

	"go-spring.org/spring/cloud/discovery"
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
		r, err := b.Pick(set, info)
		assert.Error(t, err).Nil()
		m[r.Endpoint.Addr]++
		if r.Done != nil {
			r.Done(DoneInfo{})
		}
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
		r, _ := b2.Pick(set, PickInfo{})
		seq = append(seq, r.Endpoint.Addr)
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
	r1, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	r2, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, r1.Endpoint.Addr).NotEqual(r2.Endpoint.Addr)

	// Release r1's endpoint; it now has the fewest in-flight and must be chosen.
	r1.Done(DoneInfo{})
	r3, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, r3.Endpoint.Addr).Equal(r1.Endpoint.Addr)
}

func TestConsistentHash(t *testing.T) {
	b := NewConsistentHash(50)
	set := eps("a", "b", "c")

	// Same key is stable across repeated picks.
	first, err := b.Pick(set, PickInfo{HashKey: "user-42"})
	assert.Error(t, err).Nil()
	for range 20 {
		r, err := b.Pick(set, PickInfo{HashKey: "user-42"})
		assert.Error(t, err).Nil()
		assert.String(t, r.Endpoint.Addr).Equal(first.Endpoint.Addr)
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

func TestRegistry(t *testing.T) {
	for _, name := range []string{RoundRobin, LeastConn, ConsistentHash, Weighted, ZoneAware} {
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

func TestTrackerEjectAndRecover(t *testing.T) {
	now := time.Unix(0, 0)
	tr := NewTracker(TrackerConfig{Threshold: 2, EjectFor: time.Second})
	tr.now = func() time.Time { return now }

	// One failure is below threshold: still eligible.
	tr.Record("a", false)
	assert.That(t, tr.Ejected("a")).False()
	assert.Slice(t, addrs(tr.Eligible(eps("a", "b")))).Length(2)

	// Second consecutive failure trips ejection.
	tr.Record("a", false)
	assert.That(t, tr.Ejected("a")).True()
	assert.Slice(t, addrs(tr.Eligible(eps("a", "b")))).Equal([]string{"b"})

	// Still cooling down before EjectFor elapses.
	now = now.Add(500 * time.Millisecond)
	assert.Slice(t, addrs(tr.Eligible(eps("a", "b")))).Equal([]string{"b"})

	// Cool-down elapsed: a half-open trial admits "a" again.
	now = now.Add(600 * time.Millisecond)
	assert.Slice(t, addrs(tr.Eligible(eps("a", "b")))).Length(2)

	// A successful trial fully restores it.
	tr.Record("a", true)
	assert.That(t, tr.Ejected("a")).False()

	// If the trial fails instead, it re-ejects for another window.
	tr.Record("a", false)
	tr.Record("a", false)
	assert.That(t, tr.Ejected("a")).True()
	now = now.Add(1100 * time.Millisecond)
	tr.Eligible(eps("a")) // admit trial -> half-open
	tr.Record("a", false) // trial fails
	assert.That(t, tr.Ejected("a")).True()
}

func TestTrackerDisabled(t *testing.T) {
	tr := NewTracker(TrackerConfig{Threshold: 0})
	tr.Record("a", false)
	tr.Record("a", false)
	tr.Record("a", false)
	assert.That(t, tr.Ejected("a")).False()
	assert.Slice(t, addrs(tr.Eligible(eps("a", "b")))).Length(2)
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
		r, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		m[r.Endpoint.Addr]++
		r.Done(DoneInfo{})
	}
	// The unhealthy instance is never picked.
	assert.Number(t, m["b"]).Equal(0)
	assert.Number(t, m["a"]+m["c"]).Equal(20)
}

func TestPoolEvictionViaDone(t *testing.T) {
	src := staticSource(eps("a", "b"))
	tr := NewTracker(TrackerConfig{Threshold: 2, EjectFor: time.Minute})
	p := NewPool(src, NewRoundRobin(), WithTracker(tr))

	// Drive "a" to failure through the pool's Done wiring, twice, to eject it.
	for range 5 {
		r, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		if r.Endpoint.Addr == "a" {
			r.Done(DoneInfo{Err: errors.New("boom")})
		} else {
			r.Done(DoneInfo{})
		}
	}
	assert.That(t, tr.Ejected("a")).True()

	// Subsequent picks avoid the evicted instance.
	for range 10 {
		r, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		assert.String(t, r.Endpoint.Addr).Equal("b")
		r.Done(DoneInfo{})
	}
}

func TestPoolEmpty(t *testing.T) {
	p := NewPool(staticSource(nil), NewRoundRobin())
	_, err := p.Pick(PickInfo{})
	assert.Error(t, err).Is(ErrNoAvailable)
}

func TestPoolMeshPassThrough(t *testing.T) {
	t.Cleanup(func() { discovery.SetMeshMode(false) })

	// A tracker that would eject "svc" after a single failure. In mesh mode it
	// must be bypassed entirely so the lone endpoint is never black-holed.
	src := staticSource{{Addr: "svc", Healthy: true}}
	tr := NewTracker(TrackerConfig{Threshold: 1, EjectFor: time.Minute})
	p := NewPool(src, NewRoundRobin(), WithTracker(tr))

	discovery.SetMeshMode(true)
	for range 5 {
		r, err := p.Pick(PickInfo{})
		assert.Error(t, err).Nil()
		assert.String(t, r.Endpoint.Addr).Equal("svc")
		// Report failures: without the mesh short-circuit these would eject "svc".
		r.Done(DoneInfo{Err: errors.New("boom")})
	}
	// The tracker never saw the failures — degradation skipped it.
	assert.That(t, tr.Ejected("svc")).False()
}
