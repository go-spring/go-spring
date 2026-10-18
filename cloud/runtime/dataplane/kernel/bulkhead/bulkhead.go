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

package bulkhead

import (
	"context"
	"sync"
	"time"
)

// Bulkhead is one concurrency-isolation stage: at most maxConcurrent callers
// hold a slot at once. It carries no direction vocabulary; both call
// directions may use it, and a caller assembling its own middleware can take
// one directly.
//
// The zero value — and a Bulkhead built with maxConcurrent <= 0 — admits every
// caller and its release is a no-op.
type Bulkhead struct {
	sem      chan struct{}
	queueFor time.Duration
}

// New builds a bulkhead admitting at most maxConcurrent concurrent callers. A
// caller that finds the bulkhead full may wait up to queueFor for a slot; 0
// rejects immediately.
func New(maxConcurrent int, queueFor time.Duration) *Bulkhead {
	b := &Bulkhead{queueFor: queueFor}
	if maxConcurrent > 0 {
		b.sem = make(chan struct{}, maxConcurrent)
	}
	return b
}

// QueueFor is how long a caller finding the bulkhead full may wait. A
// direction's adapter can report it as its own retry-after hint.
func (b *Bulkhead) QueueFor() time.Duration {
	if b == nil {
		return 0
	}
	return b.queueFor
}

// Acquire takes one slot and returns the release function for it. ok is false
// when no slot was obtained (and nothing is consumed). The release function is
// idempotent.
func (b *Bulkhead) Acquire(ctx context.Context) (release func(), ok bool) {
	if b == nil || b.sem == nil {
		return func() {}, true
	}
	if b.queueFor <= 0 {
		select {
		case b.sem <- struct{}{}:
			return b.releaser(), true
		default:
			return func() {}, false
		}
	}
	qctx, cancel := context.WithTimeout(ctx, b.queueFor)
	defer cancel()
	select {
	case b.sem <- struct{}{}:
		return b.releaser(), true
	case <-qctx.Done():
		return func() {}, false
	}
}

// releaser returns a function that returns the slot exactly once.
func (b *Bulkhead) releaser() func() {
	var once sync.Once
	return func() {
		once.Do(func() { <-b.sem })
	}
}
