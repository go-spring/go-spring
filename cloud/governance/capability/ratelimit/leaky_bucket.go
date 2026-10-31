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

// leaky_bucket.go is the leaky-bucket counter: the token bucket's dual,
// bounding admission by a constant drain rate.

package ratelimit

import (
	"sync"
	"time"
)

// LeakyBucket is the dual of [TokenBucket]: events pour water IN, and the
// bucket drains at a constant rate per second up to capacity. A call is
// admitted when it fits without overflowing, so outflow is what is smooth —
// where a token bucket caps bursts by the bucket's size, a leaky bucket
// never admits faster than its drain rate allows over any interval. There is
// no queueing form: an event that would overflow is rejected, not held.
type LeakyBucket struct {
	mu       sync.Mutex
	rate     float64
	capacity float64
	water    float64
	last     time.Time
}

// NewLeakyBucket returns a bucket draining rate units per second, admitting
// events until capacity units are held.
func NewLeakyBucket(rate float64, capacity int) *LeakyBucket {
	return &LeakyBucket{
		rate:     rate,
		capacity: float64(capacity),
		last:     time.Now(),
	}
}

// AllowN adds n units of water when the bucket can hold them without
// overflowing.
func (b *LeakyBucket) AllowN(n float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	b.water -= now.Sub(b.last).Seconds() * b.rate
	if b.water < 0 {
		b.water = 0
	}
	b.last = now
	if b.water+n > b.capacity {
		return false
	}
	b.water += n
	return true
}
