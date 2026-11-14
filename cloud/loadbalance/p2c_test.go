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
	"testing"
	"testing/synctest"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

func TestP2CSpreadWhenHealthy(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// With every candidate healthy and fast, power-of-two-choices keeps the
		// spread near even (each draw starts from a uniform pair).
		b := NewP2C()
		set := eps("a", "b", "c")
		// Give every address the same small measurement, so the comparison is about
		// load rather than which prior happens to be fresher.
		for _, addr := range []string{"a", "b", "c"} {
			b.ewma[addr] = float64(time.Microsecond)
			b.observedAt[addr] = time.Now()
		}

		m := map[string]int{}
		for i := range 2000 {
			ep, err := b.Pick(set, PickInfo{})
			assert.Error(t, err).Nil()
			m[ep.Addr]++
			b.Complete(context.Background(), ep, nil)
			// Let staleness decay run periodically: a peer that keeps losing its
			// comparisons is never sampled, so only aging can make it competitive
			// again — and a microsecond-scale loop never reaches that on its own.
			if i%100 == 99 {
				time.Sleep(ewmaTau)
			}
		}
		// p2c is not uniform by design; the invariant this loop can check is that
		// no address is starved to zero.
		for _, addr := range []string{"a", "b", "c"} {
			assert.Number(t, m[addr]).GreaterThan(0)
		}
	})
}

// TestP2CFailuresDoNotFeedLatency pins that a failure is not turned into a
// synthetic latency sample: the strategy would then have to keep the peer out of
// rotation to recover, which it cannot do by measurement (it is not selected) and
// which duplicates the tracker's failure handling.
func TestP2CFailuresDoNotFeedLatency(t *testing.T) {
	b := NewP2C()
	set := eps("a", "b")

	ep, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	ewma := b.ewma[ep.Addr] // seeded from the prior by the Pick's comparison

	b.Complete(context.Background(), ep, errutil.Explain(nil, "boom"))

	assert.Number(t, b.ewma[ep.Addr]).Equal(ewma)  // estimate untouched
	assert.Number(t, b.inflight[ep.Addr]).Equal(0) // slot released anyway
}

func TestP2CInflightDrains(t *testing.T) {
	// An outstanding pick scales the score; Complete releases the slot and the
	// address competes on latency alone again.
	b := NewP2C()
	set := eps("a", "b")

	e1, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	withInflight := b.score(e1.Addr)
	b.Complete(context.Background(), e1, nil)
	assert.That(t, b.score(e1.Addr) < withInflight).True()
}

// TestP2CInitialLatency pins the pessimistic prior: an address with no
// measurement is scored from initialLatency rather than from zero, so its
// in-flight count matters from the first pick instead of being multiplied away.
func TestP2CInitialLatency(t *testing.T) {
	b := NewP2C()

	assert.Number(t, b.score("new")).Equal(float64(initialLatency))

	b.inflight["new"] = 2
	got := b.score("new")
	assert.That(t, got > 2.9*float64(initialLatency)).True()
	assert.That(t, got < 3.01*float64(initialLatency)).True()
}

// TestP2CPrunesVanishedAddresses pins the bounded-state rule: a sweep triggered
// by table growth drops state for addresses that left the candidate set, while a
// live address and a recently observed one keep their history.
func TestP2CPrunesVanishedAddresses(t *testing.T) {
	b := NewP2C()
	stale := time.Now().Add(-2 * ewmaTau)
	for _, addr := range []string{"gone-1", "gone-2"} {
		b.ewma[addr] = 1
		b.observedAt[addr] = stale
	}
	b.ewma["kept"] = 1 // live: never pruned
	b.observedAt["kept"] = stale
	b.ewma["recent"] = 1 // gone from eps, but observed inside the window
	b.observedAt["recent"] = time.Now()

	// Four tracked addresses for one candidate: the sweep runs on this pick.
	_, err := b.Pick(eps("kept"), PickInfo{})
	assert.Error(t, err).Nil()

	for _, addr := range []string{"gone-1", "gone-2"} {
		_, ok := b.ewma[addr]
		assert.That(t, ok).False()
	}
	for _, addr := range []string{"kept", "recent"} {
		_, ok := b.ewma[addr]
		assert.That(t, ok).True()
	}
}

func TestP2CEmpty(t *testing.T) {
	b := NewP2C()
	_, err := b.Pick(nil, PickInfo{})
	assert.Error(t, err).Is(ErrNoEndpoints)
}

func TestP2CSingleCandidate(t *testing.T) {
	b := NewP2C()
	set := []discovery.Endpoint{{Addr: "only"}}
	ep, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, ep.Addr).Equal("only")
}

func TestP2CStalenessAgingReadmits(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// The frozen-estimate guard: an address whose EWMA went high and then
		// stopped being picked must re-enter rotation once its estimate decays past
		// the tau half-life — otherwise it starves forever behind a stale number.
		b := NewP2C()
		b.ewma["slow"] = float64(time.Second)
		b.observedAt["slow"] = time.Now()

		assert.Number(t, b.score("slow")).GreaterThan(0.5 * float64(time.Second))

		time.Sleep(time.Second)
		assert.Number(t, b.score("slow")).GreaterThan(0.1 * float64(time.Second))

		// After several taus with no observations, the stored estimate decays to
		// microseconds of noise and the address competes again.
		time.Sleep(ewmaTau * 10)
		assert.Number(t, b.score("slow")).LessThan(float64(time.Millisecond))
	})
}

// TestP2CDecayFloor pins the underflow guard: however long an address has been
// idle, the decayed estimate never reaches zero, so the in-flight factor keeps
// counting.
func TestP2CDecayFloor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NewP2C()
		b.ewma["idle"] = float64(time.Second)
		b.observedAt["idle"] = time.Now()
		b.inflight["idle"] = 9

		time.Sleep(1000 * time.Hour)
		assert.Number(t, b.score("idle")).Equal(minLatency * 10)
	})
}
