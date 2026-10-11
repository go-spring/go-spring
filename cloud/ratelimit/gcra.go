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

// gcra.go is the Generic Cell Rate Algorithm: the token bucket's dual in one
// stored timestamp, the shape that ports to an atomic store script most
// easily.

package ratelimit

import (
	"sync"
	"time"
)

// GCRA is the Generic Cell Rate Algorithm — the token bucket expressed as
// virtual scheduling. It stores no bucket, only the THEORETICAL arrival time
// (TAT) of the next admissible event: a call arriving at now is admitted when
// its TAT is no more than burst tolerance in the future, and admitting it
// pushes the TAT one interval (1/rate) further. The admission decision is a
// single compare-and-add over one timestamp, which is why GCRA is the shape
// that ports to an atomic store script (Redis Lua, e.g.) with the least state
// per key.
//
// Throughput equals [TokenBucket] at the same rate and burst (they are
// mathematically dual); choose GCRA when the one-timestamp state matters.
type GCRA struct {
	mu    sync.Mutex
	rate  float64
	burst time.Duration // burst tolerance: how far ahead the TAT may sit
	tat   time.Time     // theoretical arrival time of the next event
}

// NewGCRA returns a limiter admitting rate events per second with a burst
// tolerance of burst extra events in an instant.
func NewGCRA(rate float64, burst int) *GCRA {
	return &GCRA{rate: rate, burst: time.Duration(float64(burst) / rate * float64(time.Second))}
}

// AllowN consumes n events' worth of schedule when they fit inside the burst
// tolerance.
func (g *GCRA) AllowN(n int) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	if g.tat.IsZero() {
		g.tat = now // first event: the schedule starts here
	}
	ahead := g.tat.Sub(now) // how far ahead of now the schedule sits
	if ahead > g.burst {
		return false // the schedule is beyond the burst tolerance
	}
	g.tat = g.tat.Add(time.Duration(float64(n) / g.rate * float64(time.Second)))
	return true
}
