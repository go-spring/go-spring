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

	"go-spring.org/stdlib/testing/assert"
)

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
