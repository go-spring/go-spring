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

func init() { Register(LeastConn, func() Balancer { return newLeastConn() }) }

// NewLeastConn returns a least-connections [Balancer]: each request goes to the
// endpoint currently serving the fewest in-flight requests. This adapts to
// slow instances automatically — a backend that is struggling accumulates
// in-flight requests and stops being picked until it drains.
//
// The caller MUST invoke the returned [Result.Done] when the request finishes;
// otherwise the in-flight count leaks and that endpoint is starved.
func NewLeastConn() Balancer { return newLeastConn() }

func newLeastConn() *leastConn {
	return &leastConn{inflight: map[string]int{}}
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

func (b *leastConn) Pick(eps []discovery.Endpoint, _ PickInfo) (Result, error) {
	if len(eps) == 0 {
		return Result{}, ErrNoAvailable
	}

	b.mu.Lock()
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
	b.mu.Unlock()

	done := false
	return Result{
		Endpoint: ep,
		Done: func(DoneInfo) {
			b.mu.Lock()
			defer b.mu.Unlock()
			if done {
				return // guard against a double Done leaking the counter negative
			}
			done = true
			if n := b.inflight[ep.Addr]; n <= 1 {
				delete(b.inflight, ep.Addr)
			} else {
				b.inflight[ep.Addr] = n - 1
			}
		},
	}, nil
}
