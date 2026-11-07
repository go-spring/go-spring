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
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestWeighted(t *testing.T) {
	set := []discovery.Endpoint{
		{Addr: "a", Weight: 5},
		{Addr: "b", Weight: 1},
		{Addr: "c", Weight: 1},
	}
	b := newBalancer(t, Selection{Balancer: Weighted})
	m := counts(t, b, set, PickInfo{}, 7)
	// Over one full cycle (sum of weights = 7) each endpoint gets exactly its
	// weight.
	assert.Number(t, m["a"]).Equal(5)
	assert.Number(t, m["b"]).Equal(1)
	assert.Number(t, m["c"]).Equal(1)

	// Smoothness: the 5-weight endpoint must not take all 5 slots up front —
	// b and c are interleaved within the cycle, so a never runs 5 in a row.
	b2 := newBalancer(t, Selection{Balancer: Weighted})
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
	b3 := newBalancer(t, Selection{Balancer: Weighted})
	m3 := counts(t, b3, eps("x", "y"), PickInfo{}, 4)
	assert.Number(t, m3["x"]).Equal(2)
	assert.Number(t, m3["y"]).Equal(2)
}

// TestWeightedClampsHugeWeights pins the overflow guard: discovery can hand over
// any int as a weight, and two extreme values would overflow the round's total
// and starve one endpoint. Weights are clamped before they reach the accumulator.
func TestWeightedClampsHugeWeights(t *testing.T) {
	huge := int(^uint(0) >> 1)

	assert.Number(t, effectiveWeight(discovery.Endpoint{Weight: huge})).Equal(maxWeight)
	assert.Number(t, effectiveWeight(discovery.Endpoint{Weight: maxWeight + 1})).Equal(maxWeight)
	assert.Number(t, effectiveWeight(discovery.Endpoint{Weight: maxWeight})).Equal(maxWeight)
	assert.Number(t, effectiveWeight(discovery.Endpoint{Weight: 0})).Equal(1)
	assert.Number(t, effectiveWeight(discovery.Endpoint{Weight: -3})).Equal(1)

	// Two extreme weights clamp to the same value, so neither overflows the total
	// and both keep receiving traffic.
	b := newBalancer(t, Selection{Balancer: Weighted})
	set := []discovery.Endpoint{
		{Addr: "huge-1", Weight: huge},
		{Addr: "huge-2", Weight: huge},
	}
	m := counts(t, b, set, PickInfo{}, 10)
	assert.Number(t, m["huge-1"]).Equal(5)
	assert.Number(t, m["huge-2"]).Equal(5)
}

func TestWeightedDropsDepartedAddresses(t *testing.T) {
	b := &WeightedBalancer{currentWeights: map[string]int64{}}
	a := discovery.Endpoint{Addr: "a", Weight: 1}
	bp := discovery.Endpoint{Addr: "b", Weight: 1}
	c := discovery.Endpoint{Addr: "c", Weight: 1}

	pick := func(set ...discovery.Endpoint) {
		t.Helper()
		_, err := b.Pick(set, PickInfo{})
		assert.Error(t, err).Nil()
	}

	pick(a, bp, c)
	assert.Map(t, b.currentWeights).Length(3)

	// A departed address must not linger in the running state.
	pick(a, bp)
	assert.Map(t, b.currentWeights).Length(2)
	if _, ok := b.currentWeights["c"]; ok {
		t.Fatalf("stale address %q kept in current: %v", "c", b.currentWeights)
	}

	// Equal sizes (one leaves, one arrives) must still evict the stale address:
	// the map grows past len(eps) because the newcomer is added before the
	// departed key is rebuilt away.
	pick(a, discovery.Endpoint{Addr: "d", Weight: 1})
	assert.Map(t, b.currentWeights).Length(2)
	if _, ok := b.currentWeights["b"]; ok {
		t.Fatalf("stale address %q kept in current: %v", "b", b.currentWeights)
	}
}
