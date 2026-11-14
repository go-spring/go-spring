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

	"go-spring.org/cloud/discovery"
)

// NewRandom builds the [Random] strategy. It has no parameters of its own.
func NewRandom() *RandomBalancer {
	return &RandomBalancer{}
}

// RandomBalancer is a uniform-random [Balancer]: every pick draws one candidate
// at random. It is the simplest strategy and a baseline to compare the smarter
// ones against.
//
// It is fully stateless — no cursor, no lock, no per-address map — so concurrent
// picks never contend (unlike round-robin's shared atomic) and one value can be
// reused across pools without coupling them. The law of large numbers keeps the
// spread even, which makes it a reasonable choice for wide fan-out callers with
// no better signal to go on.
//
// It ignores [discovery.Endpoint.Weight] and every [PickInfo] hint: the choice is
// uniform over the candidates it is given. Weight and locality belong to the
// weighted strategy and the zone-aware filter respectively.
type RandomBalancer struct{}

// Pick returns a uniformly random endpoint from eps, or [ErrNoEndpoints] when eps
// is empty. It never blocks, allocates nothing, and does not retain eps.
func (b *RandomBalancer) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	return eps[rand.IntN(len(eps))], nil
}

// Complete is a no-op: random keeps no per-request state.
func (b *RandomBalancer) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}
