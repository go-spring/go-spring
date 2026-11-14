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
	"sync"

	"go-spring.org/cloud/discovery"
)

// NewLeastConn builds the [LeastConn] strategy. It has no parameters of its own.
// The tie-break rotation starts at a random offset so a fleet of clients that
// starts together does not break ties toward the same instance.
func NewLeastConn() *LeastConnBalancer {
	return &LeastConnBalancer{
		inflight: map[string]int{},
		rotation: rand.Uint64(),
	}
}

// LeastConnBalancer is a least-connections [Balancer]: each request goes to the
// endpoint currently serving the fewest in-flight requests. This adapts to slow
// instances automatically — a backend that is struggling accumulates in-flight
// requests and stops being picked until it drains.
//
// It tracks in-flight request counts per endpoint address. Counts persist across
// Pick calls (that is the whole point) and are keyed by Addr so the set can
// change underneath without losing accounting for surviving addresses. Entries
// disappear when their count reaches zero, so the table only holds addresses with
// requests actually in flight — a departing endpoint leaves no state behind.
//
// The caller MUST invoke [Balancer.Complete] exactly once per picked endpoint
// when the request finishes; otherwise the in-flight count leaks and that
// endpoint is starved.
//
// [discovery.Endpoint.Weight] is not taken into account: endpoints are compared
// by in-flight count alone. Weight belongs to the weighted strategy.
//
// Build it with [NewLeastConn]: the in-flight table is not created lazily.
type LeastConnBalancer struct {
	// mu guards inflight and rotation.
	mu sync.Mutex

	// inflight counts outstanding requests per endpoint address. An entry exists
	// only while its count is non-zero.
	inflight map[string]int

	// rotation advances the scan start on every Pick, so endpoints tied at the
	// minimum share traffic instead of the first one taking all of it.
	rotation uint64
}

// Pick selects the endpoint with the fewest in-flight requests, starting the
// scan at a rotating offset so ties are broken fairly. eps is read, never
// mutated.
func (b *LeastConnBalancer) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoEndpoints
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	start := int(b.rotation % uint64(len(eps)))
	b.rotation++
	bestIndex := -1
	bestCount := 0
	for k := range eps {
		idx := (start + k) % len(eps)
		count := b.inflight[eps[idx].Addr]
		if bestIndex < 0 || count < bestCount {
			bestIndex = idx
			bestCount = count
		}
	}
	ep := eps[bestIndex]
	b.inflight[ep.Addr]++
	return ep, nil
}

// Complete decrements the in-flight count for ep.Addr. The count reaching zero
// deletes the entry, so a repeat while the endpoint has no other request in
// flight changes nothing and never drives the count negative. It cannot make a
// repeated call harmless in general, though: counts are aggregated per address,
// so a repeat while other requests are still in flight decrements theirs. Each
// picked endpoint must be settled exactly once.
func (b *LeastConnBalancer) Complete(_ context.Context, ep discovery.Endpoint, _ error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n := b.inflight[ep.Addr]; n <= 1 {
		delete(b.inflight, ep.Addr)
	} else {
		b.inflight[ep.Addr] = n - 1
	}
}
