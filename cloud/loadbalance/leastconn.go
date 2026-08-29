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
	Register(LeastConn, NewLeastConn)
}

// leastConn tracks in-flight request counts per endpoint address. Counts persist
// across Pick calls (that is the whole point) and are keyed by Addr so the set
// can change underneath without losing accounting for surviving addresses.
type leastConn struct {
	mu       sync.Mutex
	inflight map[string]int
	// rr breaks ties fairly so equally-loaded endpoints share traffic instead of
	// all piling onto the first one.
	rr uint64
}

// NewLeastConn returns a least-connections [Balancer]: each request goes to the
// endpoint currently serving the fewest in-flight requests. This adapts to
// slow instances automatically — a backend that is struggling accumulates
// in-flight requests and stops being picked until it drains.
//
// The caller MUST invoke [Balancer.Complete] when the request finishes;
// otherwise the in-flight count leaks and that endpoint is starved.
func NewLeastConn() Balancer {
	return &leastConn{inflight: map[string]int{}}
}

func (b *leastConn) Pick(eps []discovery.Endpoint, _ PickInfo) (discovery.Endpoint, error) {
	if len(eps) == 0 {
		return discovery.Endpoint{}, ErrNoAvailable
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	// Scan for the minimum in-flight count. Start the scan at a rotating offset
	// so that among endpoints tied at the minimum the choice rotates.
	start := int(b.rr % uint64(len(eps)))
	b.rr++
	best := -1
	bestN := 0
	for k := range eps {
		idx := (start + k) % len(eps)
		n := b.inflight[eps[idx].Addr]
		if best < 0 || n < bestN {
			best = idx
			bestN = n
		}
	}
	ep := eps[best]
	b.inflight[ep.Addr]++
	return ep, nil
}

// Complete decrements the in-flight count for ep.Addr. The count reaching zero
// deletes the entry, so a repeated Complete for the same request is a harmless
// no-op rather than a negative count.
func (b *leastConn) Complete(ep discovery.Endpoint, _ error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n := b.inflight[ep.Addr]; n <= 1 {
		delete(b.inflight, ep.Addr)
	} else {
		b.inflight[ep.Addr] = n - 1
	}
}
