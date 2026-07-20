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

// Package lock defines a framework-agnostic, zero-dependency abstraction for
// distributed locking and leader election.
//
// It answers one question for multi-replica deployments (K8s Deployment with
// replicas > 1, several processes behind a queue, ...): "may this replica run
// this exclusive piece of work right now?". A scheduled job, a singleton
// background worker or a one-off migration acquires a named lock (or wins an
// election) and only the holder proceeds; the others wait or skip.
//
// The abstraction is split from its backends deliberately. Unlike
// [go-spring.org/spring/resilience] and [go-spring.org/spring/discovery], there
// is no global string-keyed driver registry, because a lock backend needs a live
// client (a Redis connection, an etcd/consul client) rather than a declarative
// policy. The seam is therefore the [Locker] interface itself: each backend ships
// as its own starter module that builds a client and exports a Locker bean, and
// switching backend is a blank-import swap with no change to business code. The
// bundled [MemoryLocker] is an in-process implementation with no dependencies, so
// the framework and its tests run standalone.
//
// [Election] builds leader election on top of any Locker, so the same code elects
// a leader whether it is backed by Redis, etcd, consul or the in-memory locker.
package lock

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

// ErrNotHeld is returned by [Lock.Unlock] when the lock is no longer owned by the
// caller — it has expired, been lost, or was already released.
var ErrNotHeld = errors.New("lock: not held by caller")

// ErrLockHeld is returned by [Locker.TryAcquire] via ok=false; it is also usable
// by backends that need to surface contention as an error.
var ErrLockHeld = errors.New("lock: already held")

// Lock is a currently held distributed lock. Implementations must be safe for
// concurrent use, and [Lock.Unlock] must be idempotent.
type Lock interface {
	// Key returns the resource key this lock guards.
	Key() string

	// Token returns the unique fencing token identifying this acquisition. A
	// downstream resource can use it to reject writes from a stale holder.
	Token() string

	// Unlock releases the lock. It is idempotent: releasing an already-released
	// or lost lock returns nil. It returns [ErrNotHeld] only when the backend can
	// prove the lock was taken over by someone else.
	Unlock(ctx context.Context) error

	// Lost returns a channel that is closed when the lock is no longer held
	// because its lease expired or renewal failed. A critical section should
	// select on it and abort its work when it fires. The channel never carries a
	// value; it is only closed.
	Lost() <-chan struct{}
}

// Locker acquires distributed locks. Implementations must be safe for concurrent
// use.
type Locker interface {
	// Acquire blocks until the lock for key is held, ctx is done, or a
	// non-retriable error occurs. On success it returns a held [Lock] whose lease
	// is kept alive until Unlock.
	Acquire(ctx context.Context, key string, opts ...Option) (Lock, error)

	// TryAcquire attempts to take the lock once without blocking. ok reports
	// whether it was acquired; when ok is false the lock is held elsewhere and
	// the returned Lock is nil. A non-nil err signals a backend failure, distinct
	// from ordinary contention.
	TryAcquire(ctx context.Context, key string, opts ...Option) (l Lock, ok bool, err error)

	// Close releases backend resources (sessions, background goroutines). It is
	// safe to call more than once. It does not release locks already handed out;
	// callers own their [Lock] handles.
	Close() error
}

// Options controls a single acquisition. A zero Options is normalized to the
// defaults documented on each field.
type Options struct {
	// TTL is the lease duration after which the lock auto-expires if not renewed,
	// so a crashed holder cannot block forever. Defaults to 30s when zero.
	TTL time.Duration

	// RenewInterval is how often the holder refreshes the lease. Defaults to
	// TTL/3 when zero; a negative value disables auto-renew (the lock then expires
	// after TTL regardless of how long the work runs).
	RenewInterval time.Duration

	// RetryInterval is how often [Locker.Acquire] retries while the lock is
	// contended. Defaults to 100ms when zero.
	RetryInterval time.Duration

	// Token is the fencing token / owner id. Defaults to a random 16-byte hex
	// string when empty, which is what most callers want.
	Token string
}

// Option mutates [Options].
type Option func(*Options)

// WithTTL sets the lease duration.
func WithTTL(d time.Duration) Option { return func(o *Options) { o.TTL = d } }

// WithRenewInterval sets the auto-renew interval; a negative value disables renew.
func WithRenewInterval(d time.Duration) Option { return func(o *Options) { o.RenewInterval = d } }

// WithRetryInterval sets the [Locker.Acquire] retry interval under contention.
func WithRetryInterval(d time.Duration) Option { return func(o *Options) { o.RetryInterval = d } }

// WithToken sets an explicit fencing token instead of a generated one.
func WithToken(token string) Option { return func(o *Options) { o.Token = token } }

// Apply builds a normalized Options from opts. Backends call it at the top of
// Acquire/TryAcquire so defaults are consistent everywhere.
func Apply(opts ...Option) Options {
	var o Options
	for _, fn := range opts {
		fn(&o)
	}
	o.normalize()
	return o
}

func (o *Options) normalize() {
	if o.TTL <= 0 {
		o.TTL = 30 * time.Second
	}
	if o.RenewInterval == 0 {
		o.RenewInterval = o.TTL / 3
	}
	if o.RetryInterval <= 0 {
		o.RetryInterval = 100 * time.Millisecond
	}
	if o.Token == "" {
		o.Token = newToken()
	}
}

// newToken returns a random 16-byte hex string used as a fencing token.
func newToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
