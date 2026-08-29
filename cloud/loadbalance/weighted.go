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

	"go-spring.org/cloud/discovery"
)

func init() {
	Register(Weighted, NewWeighted)
}

// weighted keeps a "current weight" per endpoint address across picks, which is
// the state the smooth WRR algorithm advances. Keying by Addr lets the endpoint
// set change without discarding accumulated fairness for surviving addresses.
type weighted struct {
	mu      sync.Mutex
	current map[string]int
}

// NewWeighted returns a smooth weighted round-robin [Balancer]. It honours
// [discovery.Endpoint.Weight] (a weight <= 0 is treated as 1), distributing
// requests in proportion to weight while interleaving them evenly rather than
// sending w consecutive requests to the same instance.
//
// The algorithm is the classic nginx smooth weighted round-robin: for endpoints
// {5,1,1} it yields the sequence a,a,b,a,c,a,a instead of a,a,a,a,a,b,c.
func NewWeighted() Balancer {
	return &weighted{current: map[string]int{}}
}

// effectiveWeight returns the weight the SWRR algorithm runs on: the declared
// weight, with an unset or invalid (<= 0) value falling back to 1 — the "all
// instances equal" default. Note weight 0 as a drain signal is handled earlier
// by [Pool.Pick], which filters drained endpoints for every strategy; what
// reaches here as 0 is an unnormalized snapshot, treated as default.
func effectiveWeight(ep discovery.Endpoint) int {
	if ep.Weight <= 0 {
		return 1
	}
	return ep.Weight
}

// Pick advances one step of smooth weighted round-robin. Each address carries
// a "current" score in b.current; every pick adds each endpoint's effective
// weight to its score, picks the highest score, then subtracts the total
// weight from the winner — the winner pays back what everyone just earned.
// Over one full cycle (total picks) each endpoint wins exactly its weight,
// and the wins interleave: for weights {5,1,1} the sequence is a,a,b,a,c,a,a
// rather than a,a,a,a,a,b,c, so a heavy instance never receives a burst of
// consecutive hits. Example trace (total = 7):
//
//	pick 1: current 5,1,1 -> a wins, a -= 7 -> -2,1,1
//	pick 2: current 3,2,2 -> a wins, a -= 7 -> -4,2,2
//	pick 3: current 1,3,3 -> b wins, b -= 7 -> 1,-4,3
//	...after 7 picks: a=5, b=1, c=1 wins.
func (b *weighted) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	total := 0
	best := -1
	bestCur := 0
	seen := make(map[string]struct{}, len(eps))
	for i, ep := range eps {
		w := effectiveWeight(ep)
		total += w
		cur := b.current[ep.Addr] + w // each endpoint accrues at its own weight's pace
		b.current[ep.Addr] = cur
		seen[ep.Addr] = struct{}{}
		if best < 0 || cur > bestCur {
			best = i
			bestCur = cur
		}
	}
	// Drop stale addresses so the map does not grow without bound as instances
	// churn.
	for addr := range b.current {
		if _, ok := seen[addr]; !ok {
			delete(b.current, addr)
		}
	}
	b.current[eps[best].Addr] = bestCur - total // the winner pays back the round's total
	return eps[best], nil
}

// Complete is a no-op: smooth WRR advances all of its state inside Pick.
func (b *weighted) Complete(discovery.Endpoint, error) {}
