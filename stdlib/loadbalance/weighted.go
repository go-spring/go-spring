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

	"go-spring.org/stdlib/discovery"
)

func init() { Register(Weighted, func() Balancer { return newWeighted() }) }

// NewWeighted returns a smooth weighted round-robin [Balancer]. It honours
// [discovery.Endpoint.Weight] (a weight <= 0 is treated as 1), distributing
// requests in proportion to weight while interleaving them evenly rather than
// sending w consecutive requests to the same instance.
//
// The algorithm is the classic nginx smooth weighted round-robin: for endpoints
// {5,1,1} it yields the sequence a,a,b,a,c,a,a instead of a,a,a,a,a,b,c.
func NewWeighted() Balancer { return newWeighted() }

func newWeighted() *weighted {
	return &weighted{current: map[string]int{}}
}

// weighted keeps a "current weight" per endpoint address across picks, which is
// the state the smooth WRR algorithm advances. Keying by Addr lets the endpoint
// set change without discarding accumulated fairness for surviving addresses.
type weighted struct {
	mu      sync.Mutex
	current map[string]int
}

func effectiveWeight(ep discovery.Endpoint) int {
	if ep.Weight <= 0 {
		return 1
	}
	return ep.Weight
}

func (b *weighted) Pick(eps []discovery.Endpoint, _ PickInfo) (Result, error) {
	if len(eps) == 0 {
		return Result{}, ErrNoAvailable
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
		cur := b.current[ep.Addr] + w
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
	b.current[eps[best].Addr] = bestCur - total
	return Result{Endpoint: eps[best]}, nil
}
