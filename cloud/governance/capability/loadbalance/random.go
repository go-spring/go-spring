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

	"go-spring.org/cloud/governance/capability/discovery"
)

// randomFactory is the [Factory] behind the [Random] name. The strategy has no
// parameters: a rule that sets any is rejected by [Validate].
func init() { Register(Random, newRandom) }

func newRandom(Selection) (Balancer, error) {
	return &randomBalancer{}, nil
}

// randomBalancer is a random [Balancer]: every pick draws a uniformly random
// candidate. Unlike round-robin there is no shared cursor, so concurrent picks
// never contend on one atomic — the law of large numbers keeps the spread even,
// which makes this a good baseline and a lock-free choice for very wide fan-out
// callers. It ignores weight and routing hints.
//
// It is fully stateless: no cursor, no lock, no per-address map — the only
// strategy here that can be shared across pools for free.
type randomBalancer struct{}

func (b *randomBalancer) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}
	return eps[rand.IntN(len(eps))], nil
}

// Complete is a no-op: random keeps no per-request state.
func (b *randomBalancer) Complete(_ context.Context, _ discovery.Endpoint, _ error) {}
