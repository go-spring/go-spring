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
	"sync"

	"go-spring.org/cloud/discovery"
)

// NewWeighted builds the [Weighted] strategy. It has no parameters of its own.
func NewWeighted() *WeightedBalancer {
	return &WeightedBalancer{currentWeights: map[string]int64{}}
}

// WeightedBalancer is a smooth weighted round-robin [Balancer]. It honours
// [discovery.Endpoint.Weight] (a weight <= 0 is treated as 1), distributing
// requests in proportion to weight while interleaving them evenly rather than
// sending w consecutive requests to the same instance.
//
// The algorithm is the classic nginx smooth weighted round-robin: for endpoints
// {5,1,1} it yields the sequence a,a,b,a,c,a,a instead of a,a,a,a,a,b,c.
//
// It keeps a "current weight" per endpoint address across picks, which is the
// state the algorithm advances. Keying by Addr lets the endpoint set change
// without discarding accumulated fairness for surviving addresses. The
// accumulator is int64 so the round's total stays far from overflow on 32-bit
// platforms as well.
//
// Weights are capped at [maxWeight] before they reach the accumulator, so a
// wildly misconfigured weight cannot overflow the total and starve the rest. A
// weight CHANGE applies from the next pick, but the accumulated credit is still
// in the old unit, so a demoted endpoint drains it over a bounded run (the same
// property nginx's smooth WRR has); weights are expected to be stable.
//
// Build it with [NewWeighted]: the accumulator table is not created lazily.
type WeightedBalancer struct {
	// mu guards currentWeights.
	mu sync.Mutex

	// currentWeights is the SWRR accumulator: one running score per address,
	// advanced by every pick.
	currentWeights map[string]int64
}

// maxWeight bounds a single endpoint's weight. Weights arrive from discovery
// backends without an upper bound and the SWRR accumulator sums them; two
// extreme values would overflow the total and let one endpoint starve the others
// indefinitely. A ceiling this high preserves any realistic ratio while keeping
// the sum far from overflow.
const maxWeight = 1 << 20

// effectiveWeight returns the weight the SWRR algorithm runs on: the declared
// weight, bounded by [maxWeight]; an unset or invalid (<= 0) value falls back to
// 1 — the "all instances equal" default. Note weight 0 as a drain signal is
// handled earlier by [Pool.Pick], which filters drained endpoints for every
// strategy; what reaches here as 0 is an unnormalized snapshot, treated as
// default.
func effectiveWeight(ep discovery.Endpoint) int {
	if ep.Weight <= 0 {
		return 1
	}
	if ep.Weight > maxWeight {
		return maxWeight
	}
	return ep.Weight
}

// Pick advances one step of smooth weighted round-robin. Each address carries a
// running score in [WeightedBalancer.currentWeights]; every pick adds each
// endpoint's effective weight to its score, picks the highest score, then
// subtracts the total weight from the winner — the winner pays back what
// everyone just earned. Over one full cycle (total picks) each endpoint wins
// exactly its weight, and the wins interleave: for weights {5,1,1} the sequence
// is a,a,b,a,c,a,a rather than a,a,a,a,a,b,c, so a heavy instance never receives
// a burst of consecutive hits. Example trace (total = 7):
//
//	pick 1: current 5,1,1 -> a wins, a -= 7 -> -2,1,1
//	pick 2: current 3,2,2 -> a wins, a -= 7 -> -4,2,2
//	pick 3: current 1,3,3 -> b wins, b -= 7 -> 1,-4,3
//	...after 7 picks each endpoint has won its weight times.
func (b *WeightedBalancer) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	var total int64
	bestIndex := -1
	var bestCurrent int64
	for i, ep := range eps {
		weight := int64(effectiveWeight(ep))
		total += weight
		accrued := b.currentWeights[ep.Addr] + weight // each endpoint accrues at its own weight's pace
		b.currentWeights[ep.Addr] = accrued
		if bestIndex < 0 || accrued > bestCurrent {
			bestIndex = i
			bestCurrent = accrued
		}
	}
	// The loop above has touched every picked address, so currentWeights now holds
	// the picked set plus whatever stale addresses survive: a map larger than eps
	// means exactly that some address departed (an arrival alone keeps the sizes
	// equal). Rebuilding from eps drops the stale keys in one pass; the common
	// pick, with an unchanged address set, allocates nothing.
	if len(b.currentWeights) > len(eps) {
		keptCurrents := make(map[string]int64, len(eps))
		for _, ep := range eps {
			keptCurrents[ep.Addr] = b.currentWeights[ep.Addr]
		}
		b.currentWeights = keptCurrents
	}
	b.currentWeights[eps[bestIndex].Addr] = bestCurrent - total // the winner pays back the round's total
	return eps[bestIndex], nil
}

// Complete is a no-op: smooth WRR advances all of its state inside Pick.
func (b *WeightedBalancer) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}
