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
	"sync/atomic"

	"go-spring.org/spring/discovery"
)

func init() { Register(RoundRobin, func() Balancer { return &roundRobin{} }) }

// NewRoundRobin returns a stateless round-robin [Balancer] that cycles evenly
// through the candidate set, ignoring weight. It is the simplest strategy and a
// good default when instances are homogeneous.
func NewRoundRobin() Balancer { return &roundRobin{} }

// roundRobin holds only a monotonically increasing cursor; the candidate set is
// supplied on every Pick, so no per-endpoint bookkeeping is needed.
type roundRobin struct {
	next atomic.Uint64
}

func (b *roundRobin) Pick(eps []discovery.Endpoint, _ PickInfo) (Result, error) {
	if len(eps) == 0 {
		return Result{}, ErrNoAvailable
	}
	i := b.next.Add(1) - 1
	return Result{Endpoint: eps[int(i%uint64(len(eps)))]}, nil
}
