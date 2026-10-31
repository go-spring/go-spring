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
	"sync/atomic"

	"go-spring.org/cloud/governance/capability/discovery"
)

// roundRobinFactory is the [Factory] behind the [RoundRobin] name. The strategy
// has no parameters of its own.
func init() { Register(RoundRobin, newRoundRobin) }

func newRoundRobin(Selection) (Balancer, error) {
	return &roundRobin{}, nil
}

// roundRobin is a stateless round-robin [Balancer]: it cycles evenly through the
// candidate set, ignoring weight. It is the simplest strategy and the base
// strategy a selection that names nothing runs under.
//
// It holds only a monotonically increasing cursor; the candidate set is supplied
// on every Pick, so no per-endpoint bookkeeping is needed.
type roundRobin struct {
	next atomic.Uint64
}

func (b *roundRobin) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	i := b.next.Add(1) - 1
	return eps[int(i%uint64(len(eps)))], nil
}

// Complete is a no-op: round-robin keeps no per-request state.
func (b *roundRobin) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}
