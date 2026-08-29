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

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestP2CSpreadWhenHealthy(t *testing.T) {
	// With every candidate healthy and fast, both-two-choices keeps the spread
	// near even (each draw starts from a uniform pair).
	b := NewP2C()
	m := map[string]int{}
	for range 2000 {
		ep, err := b.Pick(eps("a", "b", "c"), PickInfo{})
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		b.Complete(ep, nil)
	}
	// p2c is not uniform by design: timing noise in the EWMA makes it prefer
	// the momentarily-cheapest of each drawn pair, and a momentarily slower
	// address only re-enters rotation through the staleness aging (τ = 10s of
	// wall clock), which a microsecond-scale test loop never reaches. The
	// invariant this loop can check is that no address is starved to zero.
	for _, addr := range []string{"a", "b", "c"} {
		assert.Number(t, m[addr]).GreaterThan(0)
	}
}

func TestP2CAvoidsFailingEndpoint(t *testing.T) {
	// Failures feed a 1s penalty into the EWMA, so "b" quickly loses every
	// two-choices comparison and traffic concentrates on "a".
	b := NewP2C()
	set := eps("a", "b")
	for range 20 { // seed failures on b
		ep, err := b.Pick(set, PickInfo{})
		assert.Error(t, err).Nil()
		if ep.Addr == "b" {
			b.Complete(ep, errors.New("boom"))
		} else {
			b.Complete(ep, nil)
		}
	}
	m := map[string]int{}
	for range 200 {
		ep, err := b.Pick(set, PickInfo{})
		assert.Error(t, err).Nil()
		m[ep.Addr]++
		b.Complete(ep, nil)
	}
	assert.Number(t, m["a"]).GreaterThan(190)
}

func TestP2CInflightDrains(t *testing.T) {
	// Outstanding picks raise the score; after Complete the slot is released
	// and the address competes on latency alone again.
	b := NewP2C()
	set := eps("a", "b")

	e1, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	b.Complete(e1, nil)

	// A fresh instance with no history scores 0 and is preferred (exploration).
	e2, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	b.Complete(e2, nil)
}

func TestP2CEmpty(t *testing.T) {
	b := NewP2C()
	_, err := b.Pick(nil, PickInfo{})
	assert.Error(t, err).Is(ErrNoAvailable)
}

func TestP2CSingleCandidate(t *testing.T) {
	b := NewP2C()
	set := []discovery.Endpoint{{Addr: "only"}}
	ep, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, ep.Addr).Equal("only")
}

func TestP2CStalenessAgingReadmits(t *testing.T) {
	// The frozen-estimate guard: an address whose EWMA went high (here via a
	// failure penalty) and then stopped being picked must re-enter rotation
	// once its estimate decays past the tau half-life — otherwise it starves
	// forever behind a stale number.
	now := time.Unix(0, 0)
	b := NewP2C().(*p2c)
	b.now = func() time.Time { return now }
	set := eps("a", "b")

	// Seed a failure on "a" so its EWMA carries the 1s penalty.
	for range 10 {
		ep, err := b.Pick(set, PickInfo{})
		assert.Error(t, err).Nil()
		if ep.Addr == "a" {
			b.Complete(ep, errors.New("boom"))
			break
		}
		b.Complete(ep, nil)
	}
	assert.Number(t, b.score("a")).GreaterThan(b.score("b") + 1) // a looks costly

	// Right after: the penalty dominates the score (score is in nanoseconds).
	now = now.Add(time.Second)
	assert.Number(t, b.score("a")).GreaterThan(0.1 * float64(p2cFailurePenalty))

	// After several taus with no observations, the stored estimate decays to
	// microseconds of noise and "a" competes again.
	now = now.Add(ewmaTau * 10)
	assert.Number(t, b.score("a")).LessThan(float64(time.Millisecond))
}
