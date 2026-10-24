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

// Package bulkhead provides a direction-neutral concurrency-isolation stage:
// it bounds how many callers may hold a slot at the same time, so one slow
// dependency or handler cannot consume every worker the caller has.
//
// A nil *Bulkhead is the OFF stage; a holder that configures no isolation keeps
// nil and checks it before calling, instead of constructing an instance. [New]
// requires a positive concurrency. An enabled bulkhead rejects a caller that
// finds it full unless [WithQueueTimeout] gives it a short window to wait.
package bulkhead

import (
	"context"
	"sync/atomic"
	"time"
)

// Bulkhead is one concurrency-isolation stage: at most maxConcurrent callers
// hold a slot at once. It carries no direction vocabulary; both call
// directions may use it, and code assembling its own middleware can take one
// directly.
//
// A non-nil Bulkhead is always enabled: build it with [New]. The OFF stage is a
// nil *Bulkhead, which a holder checks before calling — methods require a
// non-nil receiver. The zero value is not a valid bulkhead and must not be used.
//
// A Bulkhead contains an atomic counter and must not be copied after first use.
type Bulkhead struct {
	slots        chan struct{}
	queueTimeout time.Duration

	// queued counts callers currently blocked waiting for a slot. It is only
	// touched in the queueing branch of [Bulkhead.Acquire].
	queued atomic.Int64
}

// Stats is the bulkhead's occupancy at one instant.
type Stats struct {
	InUse    int // slots currently held
	Queued   int // callers blocked waiting for a slot
	Capacity int // slots in total
}

// Stats reports the bulkhead's current occupancy. It takes no lock.
func (b *Bulkhead) Stats() Stats {
	return Stats{
		InUse:    len(b.slots),
		Queued:   int(b.queued.Load()),
		Capacity: cap(b.slots),
	}
}

// Option customizes a Bulkhead at construction.
type Option func(*Bulkhead)

// WithQueueTimeout lets a caller that finds the bulkhead full wait up to
// timeout for a slot. It is optional: without it the bulkhead rejects
// immediately (fail fast), which is the safer default for isolation. A
// non-positive timeout keeps fail-fast as well.
func WithQueueTimeout(timeout time.Duration) Option {
	return func(b *Bulkhead) { b.queueTimeout = timeout }
}

// New builds a bulkhead admitting at most maxConcurrent callers at once.
// maxConcurrent must be positive: "no isolation" is a nil *Bulkhead at the
// holder, never a disabled instance, so New panics on a non-positive value and
// the holder guarantees it. By default a caller that finds the bulkhead full is
// rejected immediately; pass [WithQueueTimeout] to make it wait for a slot.
func New(maxConcurrent int, opts ...Option) *Bulkhead {
	if maxConcurrent <= 0 {
		panic("bulkhead: New requires maxConcurrent > 0")
	}
	b := &Bulkhead{slots: make(chan struct{}, maxConcurrent)}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// QueueTimeout reports how long a caller that finds the bulkhead full may
// wait for a slot before being refused. A direction's adapter can surface it
// as its own retry-after hint.
func (b *Bulkhead) QueueTimeout() time.Duration {
	return b.queueTimeout
}

// Acquire takes one slot and returns the function that releases it. ok is
// false when no slot was obtained; nothing is consumed in that case and the
// returned function does nothing.
//
// Every successful Acquire and its release form a pair the caller owns: the
// release must run exactly once. Releasing twice returns a slot this caller
// does not hold, so the bulkhead loses one unit of capacity for good; never
// releasing pins the slot for good. Neither can be detected here — the slots
// are indistinguishable, and there is no per-acquisition state — so exact
// pairing is the caller's contract, not the bulkhead's.
//
// A caller whose context is already done is refused before touching the slots:
// a request that can no longer be served must not occupy isolation capacity.
// The OFF stage has no method at all — that is why it is a nil *Bulkhead — so
// a holder that configures no isolation admits the request itself instead of
// calling Acquire.
func (b *Bulkhead) Acquire(ctx context.Context) (release func(), ok bool) {
	if ctx.Err() != nil {
		return func() {}, false
	}

	if b.queueTimeout <= 0 {
		select {
		case b.slots <- struct{}{}:
			return func() { <-b.slots }, true
		default:
			return func() {}, false
		}
	}

	// Count this caller as queued only while it is actually waiting: a caller
	// that finds a free slot still passes through the select, but its decrement
	// fires as soon as Acquire returns.
	b.queued.Add(1)
	defer b.queued.Add(-1)

	waitCtx, cancel := context.WithTimeout(ctx, b.queueTimeout)
	defer cancel()
	select {
	case b.slots <- struct{}{}:
		return func() { <-b.slots }, true
	case <-waitCtx.Done():
		return func() {}, false
	}
}
