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

package at

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"

	"go-spring.org/spring/resilience"
)

// ErrUnknownTransaction is returned when a branch registers under, or a caller
// resolves, an XID the coordinator does not know (never begun, or already
// resolved).
var ErrUnknownTransaction = errors.New("at: unknown global transaction")

// Option configures the in-process [Coordinator] built by [NewCoordinator].
type Option func(*coordinator)

// WithGlobalLock sets the [GlobalLock] released when a global transaction
// resolves. Without it the coordinator provides no write-write isolation, which
// is acceptable for single-writer setups and tests; pass one in production.
func WithGlobalLock(l GlobalLock) Option { return func(c *coordinator) { c.lock = l } }

// WithObserver sets the [Observer] used to instrument every branch phase, e.g. a
// starter that opens otel spans.
func WithObserver(o Observer) Option { return func(c *coordinator) { c.observer = o } }

// WithRetry sets the [RetryPolicy] applied to each branch's second-phase
// operation. The zero value (default) means a single attempt; a non-zero policy
// is useful because commit/rollback are expected to eventually succeed.
func WithRetry(p RetryPolicy) Option { return func(c *coordinator) { c.retry = p } }

// NewCoordinator returns the bundled in-process [Coordinator]. With no options it
// uses no global lock and no observer, which is a valid transparent setup for
// tests and single-writer development.
func NewCoordinator(opts ...Option) Coordinator {
	c := &coordinator{active: make(map[string][]Branch)}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type coordinator struct {
	lock     GlobalLock
	observer Observer
	retry    RetryPolicy

	mu     sync.Mutex
	active map[string][]Branch // xid -> registered branches, in registration order
}

func (c *coordinator) Begin(ctx context.Context) (context.Context, string) {
	xid := newXID()
	c.mu.Lock()
	c.active[xid] = nil
	c.mu.Unlock()
	return WithXID(ctx, xid), xid
}

func (c *coordinator) Register(_ context.Context, xid string, b Branch) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	branches, ok := c.active[xid]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownTransaction, xid)
	}
	// Deduplicate by resource id: a database that writes several times in one
	// global transaction is committed/rolled back exactly once.
	if slices.ContainsFunc(branches, func(x Branch) bool { return x.ID() == b.ID() }) {
		return nil
	}
	c.active[xid] = append(branches, b)
	return nil
}

func (c *coordinator) Commit(ctx context.Context, xid string) error {
	branches, err := c.take(xid)
	if err != nil {
		return err
	}
	defer c.release(ctx, xid)

	var errs []error
	for _, b := range branches {
		if e := c.runPhase(ctx, xid, b, PhaseCommit, b.Commit); e != nil {
			errs = append(errs, fmt.Errorf("branch %q commit: %w", b.ID(), e))
		}
	}
	return errors.Join(errs...)
}

func (c *coordinator) Rollback(ctx context.Context, xid string) error {
	branches, err := c.take(xid)
	if err != nil {
		return err
	}
	defer c.release(ctx, xid)

	// Undo in reverse registration order, mirroring the reverse-order compensation
	// of Saga/TCC: later branches are unwound before earlier ones.
	var errs []error
	for _, b := range slices.Backward(branches) {
		if e := c.runPhase(ctx, xid, b, PhaseRollback, b.Rollback); e != nil {
			errs = append(errs, fmt.Errorf("branch %q rollback: %w", b.ID(), e))
		}
	}
	return errors.Join(errs...)
}

// take removes and returns the branches for xid, erroring if it is unknown. It
// makes resolution single-shot: a second Commit/Rollback of the same xid finds
// nothing and reports ErrUnknownTransaction.
func (c *coordinator) take(xid string) ([]Branch, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	branches, ok := c.active[xid]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrUnknownTransaction, xid)
	}
	delete(c.active, xid)
	return branches, nil
}

// release frees the global lock for xid. A lock error is intentionally swallowed:
// the transaction has already been resolved and failing the whole operation on a
// lock-release error would be worse than a stranded lock, which a lock backend's
// own TTL / monitoring should reclaim.
func (c *coordinator) release(ctx context.Context, xid string) {
	if c.lock != nil {
		_ = c.lock.Release(ctx, xid)
	}
}

// runPhase runs one branch phase under the retry policy and observer. The
// observer end func always fires with the phase's final error so failures are
// observable.
func (c *coordinator) runPhase(ctx context.Context, xid string, b Branch, phase Phase,
	fn func(context.Context, string) error) error {

	end := func(error) {}
	if c.observer != nil {
		ctx, end = c.observer.Begin(ctx, xid, b.ID(), phase)
	}
	err := runWithPolicy(ctx, c.retry, b.ID(), func(ctx context.Context) error {
		return fn(ctx, xid)
	})
	end(err)
	return err
}

// newXID returns a random 16-byte hex global transaction id.
func newXID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// runWithPolicy runs fn once when the policy is zero, or under the bundled
// resilience "default" executor (retry, per-attempt timeout, ...) otherwise. It
// reuses [resilience.Policy] so AT phase retries and outbound resilience share
// one knob set instead of duplicating retry logic here.
func runWithPolicy(ctx context.Context, p RetryPolicy, resource string, fn func(context.Context) error) error {
	if p == (RetryPolicy{}) {
		return fn(ctx)
	}
	drv, err := resilience.MustGetDriver("default")
	if err != nil {
		return err
	}
	exec, err := drv.NewExecutor(p)
	if err != nil {
		return err
	}
	defer func() { _ = exec.Close() }()
	return exec.Execute(ctx, resource, fn)
}
