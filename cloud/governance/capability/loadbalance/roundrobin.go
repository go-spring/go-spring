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
	"math/rand/v2"
	"sync/atomic"

	"go-spring.org/cloud/discovery"
)

// NewRoundRobin builds the [RoundRobin] strategy. It has no parameters of its
// own. The counter starts at a random offset so a fleet of clients that comes up
// together does not send its first picks to the same instance; the cycle is
// uniform from there.
func NewRoundRobin(Selection) *RoundRobinBalancer {
	b := &RoundRobinBalancer{}
	b.seq.Store(rand.Uint64())
	return b
}

// RoundRobinBalancer is a round-robin [Balancer]: it cycles through the
// candidate set in order, ignoring weight. It is the simplest strategy and the
// base strategy a selection that names nothing runs under.
//
// It keeps no per-endpoint or per-request state — only a monotonically
// increasing counter. The candidate set is supplied on every Pick, so no
// bookkeeping per endpoint is needed.
type RoundRobinBalancer struct {
	// seq is a monotonically increasing pick counter; its value modulo the
	// candidate count is the cycle position. It is atomic so concurrent picks each
	// take one slot without a lock.
	seq atomic.Uint64
}

// Pick returns the next endpoint in the cycle, or [ErrNoEndpoints] when eps is
// empty. While the candidate set is stable, any len(eps) consecutive picks cover
// it exactly once. eps is read, never retained.
func (b *RoundRobinBalancer) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	seq := b.seq.Add(1) - 1
	return eps[int(seq%uint64(len(eps)))], nil
}

// Complete is a no-op: round-robin keeps no per-request state.
func (b *RoundRobinBalancer) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}
