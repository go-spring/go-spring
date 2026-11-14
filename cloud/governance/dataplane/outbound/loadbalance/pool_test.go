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
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	balancer "go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

func TestPoolHealthFilter(t *testing.T) {
	src := staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true},
		discovery.Endpoint{Addr: "b", Healthy: false},
		discovery.Endpoint{Addr: "c", Healthy: true},
	)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}), mustTracker(t, TrackerConfig{}), Selection{})
	m := map[string]int{}
	for range 20 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		m[r.Endpoint.Addr]++
		r.Complete(context.Background(), nil)
	}
	// The unhealthy instance is never picked.
	assert.Number(t, m["b"]).Equal(0)
	assert.Number(t, m["a"]+m["c"]).Equal(20)
}

func TestPoolEvictionViaComplete(t *testing.T) {
	src := staticSource(eps("a", "b")...)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}), mustTracker(t, TrackerConfig{RequestThreshold: 2, SuspendDuration: time.Minute, MaxEjectionPercent: 100}), Selection{})

	// Drive "a" to failure through the pool's Complete wiring, twice, to suspend it.
	for range 5 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		if r.Endpoint.Addr == "a" {
			r.Complete(context.Background(), errutil.Explain(nil, "boom"))
		} else {
			r.Complete(context.Background(), nil)
		}
	}
	assert.That(t, suspended(p.tracker, "a")).True()

	// Subsequent picks avoid the evicted instance.
	for range 10 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		assert.String(t, r.Endpoint.Addr).Equal("b")
		r.Complete(context.Background(), nil)
	}
}

func TestPoolEmpty(t *testing.T) {
	p := mustPool(t, staticSource(), newBalancer(t, Selection{Balancer: balancer.RoundRobin}), mustTracker(t, TrackerConfig{}), Selection{})
	_, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
	assert.Error(t, err).Is(balancer.ErrNoEndpoints)
}

func TestPoolWithDisabledTracker(t *testing.T) {
	// Negative thresholds disable both arms: Complete with failures must not
	// suspend anything.
	src := staticSource(eps("a", "b")...)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}),
		mustTracker(t, TrackerConfig{DialThreshold: -1, RequestThreshold: -1}), Selection{})
	for range 10 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		r.Complete(context.Background(), errutil.Explain(nil, "boom")) // failures, both arms disabled
	}
	// Both endpoints keep receiving traffic.
	m := map[string]int{}
	for range 10 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		m[r.Endpoint.Addr]++
		r.Complete(context.Background(), nil)
	}
	assert.Number(t, m["a"]).Equal(5)
	assert.Number(t, m["b"]).Equal(5)
}

func TestPoolZeroWeightDrains(t *testing.T) {
	// A zero weight (the drain signal) removes the instance from picking,
	// across every strategy — here round-robin, which otherwise ignores weight.
	src := staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true, Weight: 1},
		discovery.Endpoint{Addr: "b", Healthy: true, Weight: 0},
	)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}), mustTracker(t, TrackerConfig{}), Selection{})
	for range 10 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		assert.String(t, r.Endpoint.Addr).Equal("a")
		r.Complete(context.Background(), nil)
	}
}

func TestPoolAllZeroWeightFallsBack(t *testing.T) {
	// Every endpoint zero-weighted (an unnormalized snapshot from registrants
	// predating the weight contract): fall back to an even split rather than
	// blackholing the pool.
	src := staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true},
		discovery.Endpoint{Addr: "b", Healthy: true},
	)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}), mustTracker(t, TrackerConfig{}), Selection{})
	m := map[string]int{}
	for range 20 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		m[r.Endpoint.Addr]++
		r.Complete(context.Background(), nil)
	}
	assert.Number(t, m["a"]+m["b"]).Equal(20)
	assert.Number(t, m["a"]).Equal(10)
	assert.Number(t, m["b"]).Equal(10)
}

func TestPoolNegativeWeightKept(t *testing.T) {
	// Negative weight is misconfiguration, not a drain signal: the instance
	// stays in rotation (the weighted balancer treats it as the default).
	src := staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true, Weight: -1},
		discovery.Endpoint{Addr: "b", Healthy: true, Weight: 0},
	)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}), mustTracker(t, TrackerConfig{}), Selection{})
	for range 10 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		assert.String(t, r.Endpoint.Addr).Equal("a")
		r.Complete(context.Background(), nil)
	}
}

// TestPoolAppliesItsSelectionsMetadataMatch pins that the metadata match belongs
// to the pool's construction: a pool built with a narrowed selection narrows its
// own candidate set, so what [Pool.Selection] reports and what [Pool.Pick] does
// cannot diverge — including for a standalone caller with no manager behind it.
func TestPoolAppliesItsSelectionsMetadataMatch(t *testing.T) {
	p := mustPool(t, versionedSource(), newBalancer(t, Selection{Balancer: balancer.RoundRobin}), mustTracker(t, TrackerConfig{}),
		Selection{Balancer: balancer.RoundRobin, MetadataMatch: versionPrefix("v2.")})

	for range 5 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		assert.String(t, r.Endpoint.Addr).Equal("a")
	}
}

// mustTracker builds a tracker, failing the test if its instruments cannot be
// registered.
func mustTracker(t *testing.T, cfg TrackerConfig) *Tracker {
	t.Helper()
	tr, err := NewTracker("svc", cfg, emptyObserver{})
	if err != nil {
		t.Fatal(err)
	}
	return tr
}

// mustPool builds a pool from caller-supplied parts.
func mustPool(t *testing.T, src discovery.Resolver, bal balancer.Balancer, tracker *Tracker, sel Selection) *Pool {
	t.Helper()
	return NewPool(src, bal, tracker, sel)
}

// TestMinHealthyPercentIgnoresDrainedEndpoints pins the ordering: drain runs
// before suspension, so a zero-weight endpoint is not counted as selectable
// capacity by the floor. With one live and one drained endpoint and a 50% floor,
// suspending the live one would leave 0/1 and is refused.
func TestMinHealthyPercentIgnoresDrainedEndpoints(t *testing.T) {
	src := staticSource(
		discovery.Endpoint{Addr: "a", Healthy: true, Weight: 1},
		discovery.Endpoint{Addr: "b", Healthy: true, Weight: 0}, // drained
	)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}),
		mustTracker(t, TrackerConfig{RequestThreshold: 1, SuspendDuration: time.Minute, MinHealthyPercent: 50}),
		Selection{Balancer: balancer.RoundRobin})

	peer, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
	assert.Error(t, err).Nil()
	assert.String(t, peer.Endpoint.Addr).Equal("a")

	peer.Complete(context.Background(), errors.New("boom"))
	assert.That(t, suspendedState(p.tracker, "a")).False()
}

// TestPoolPanicNeverPicksDisabled pins that the panic floor widens to the
// eligible pool, never past the Disabled exclusion: a health-driven panic makes
// unhealthy peers servable, but an administratively removed one stays out.
func TestPoolPanicNeverPicksDisabled(t *testing.T) {
	src := staticSource(
		discovery.Endpoint{Addr: "h", Healthy: true, Weight: 1},
		discovery.Endpoint{Addr: "u1", Weight: 1},
		discovery.Endpoint{Addr: "u2", Weight: 1},
		discovery.Endpoint{Addr: "d", Weight: 1, Disabled: true},
	)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}),
		mustTracker(t, TrackerConfig{MinHealthyPercent: 50}), Selection{Balancer: balancer.RoundRobin})

	for range 30 {
		r, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
		assert.Error(t, err).Nil()
		assert.That(t, r.Endpoint.Addr != "d").True()
		r.Complete(context.Background(), nil)
	}
}

// TestPoolPrefersUndrainedUnhealthyOverDrainedHealthy pins the drain/health
// order: drain is an operator decree, so an undrained unhealthy instance is a
// better candidate than a drained healthy one. The all-drained fallback belongs
// to a set where nothing is undrained, not to the healthy subset.
func TestPoolPrefersUndrainedUnhealthyOverDrainedHealthy(t *testing.T) {
	src := staticSource(
		discovery.Endpoint{Addr: "drained", Healthy: true, Weight: 0},    // drained
		discovery.Endpoint{Addr: "unhealthy", Healthy: false, Weight: 1}, // undrained
	)
	p := mustPool(t, src, newBalancer(t, Selection{Balancer: balancer.RoundRobin}),
		mustTracker(t, TrackerConfig{}), Selection{Balancer: balancer.RoundRobin})

	peer, err := p.Pick(context.Background(), balancer.PickInfo{}, nil)
	assert.Error(t, err).Nil()
	assert.String(t, peer.Endpoint.Addr).Equal("unhealthy")
}
