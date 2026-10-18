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

// engine.go is the bundled engine behind [Guard]: one server instance's
// long-lived state manager. It owns the instance's rate budget, bulkhead,
// breaker and the policy generation it materialized. Nothing here allocates a
// manager per request; a request only gets a [Result] and a completion closure
// that points back at the state it borrowed from.

package admission

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/breaker"
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
)

// Option customizes a Guard at construction.
type Option func(*options)

type options struct {
	fault   Fault
	limiter ratelimit.ClusterLimiter
}

// WithFault installs the inbound fault-injection stage. A nil Fault means no
// injection.
func WithFault(f Fault) Option {
	return func(o *options) { o.fault = f }
}

// WithClusterLimiter makes the rate-limit stage charge a shared store instead
// of the instance-local budget. A nil limiter keeps the count local (the
// default).
func WithClusterLimiter(l ratelimit.ClusterLimiter) Option {
	return func(o *options) { o.limiter = l }
}

// state is one materialized policy generation. A request captures the pointer
// it was admitted under, so a config push that replaces the state cannot change
// the stages a request in flight is settling against.
type state struct {
	policy  Policy
	rule    string
	rate    *RateLimiter
	breaker *Breaker
	bulk    *Bulkhead
}

type server struct {
	id   Identity
	opts options
	mgr  *Manager

	mu       sync.Mutex
	closed   bool
	listener breaker.EventListener
	st       *state
}

// New builds a standalone Guard with a fixed policy. It is not connected to a
// [Manager], so the policy never changes; use [Manager.GuardFor] when the
// instance follows a pushed configuration.
func New(id Identity, p Policy, opts ...Option) Guard {
	o := options{}
	for _, f := range opts {
		f(&o)
	}
	s := &server{id: id, opts: o}
	s.st = buildState(p, "", id.Service, nil, o.limiter)
	return s
}

// buildState materializes one generation of the instance's stages.
func buildState(p Policy, rule, service string, listener breaker.EventListener, limiter ratelimit.ClusterLimiter) *state {
	return &state{
		policy:  p,
		rule:    rule,
		rate:    NewRateLimiter(p, service, limiter),
		breaker: NewBreaker(p, service, listener),
		bulk:    NewBulkhead(p.MaxConcurrent, p.BulkheadQueueDuration),
	}
}

// apply installs a new policy generation. A policy identical to the current one
// is a no-op, so a redundant push does not reset a live breaker.
func (s *server) apply(p Policy, rule string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	if s.st != nil && s.st.policy == p && s.st.rule == rule {
		return
	}
	s.st = buildState(p, rule, s.id.Service, s.listener, s.opts.limiter)
}

// SetBreakerEventListener attaches l for this instance's breaker. It is meant to
// be called at wiring time, before the first request; attaching it rebuilds the
// materialized state so the breaker is created with the listener in place.
func (s *server) SetBreakerEventListener(l breaker.EventListener) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listener = l
	if s.st != nil {
		s.st = buildState(s.st.policy, s.st.rule, s.id.Service, l, s.opts.limiter)
	}
}

// Close marks the instance closed and unregisters it from its manager.
func (s *server) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	if s.mgr != nil {
		s.mgr.unregister(s)
	}
	return nil
}

// closeLocal marks the instance closed without reaching back into the manager;
// the manager's own Close uses it while holding the registry's lock.
func (s *server) closeLocal() {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
}

// Admit runs the pre-handler stages in their fixed order — bulkhead, rate
// limit, breaker gate, fault — and returns this request's judgment.
func (s *server) Admit(ctx context.Context, req Request) (Result, Done) {
	s.mu.Lock()
	st := s.st
	s.mu.Unlock()
	if st == nil {
		return admitted(st, 0, FaultResult{}), noopDone
	}

	start := time.Now()

	// Bulkhead: one slot is held for the whole request and returned by Done.
	release, ok := st.bulk.Acquire(ctx)
	if !ok {
		return refused(st, VerdictBulkheadFull, ErrBulkheadFull, st.bulk.QueueFor(), start), noopDone
	}

	// Rate limit: the request is charged before anything downstream runs.
	if !st.rate.Allow(ctx) {
		release()
		return refused(st, VerdictRateLimited, ErrRateLimited, st.policy.RateLimitMaxWait, start), noopDone
	}

	// Breaker gate. A configured degradation turns the open circuit into the
	// neutral "degraded" verdict; the cause stays in Err so an adapter can tell
	// the two apart.
	if !st.breaker.Allow() {
		release()
		v := VerdictCircuitOpen
		if st.policy.DegradeOnOpen {
			v = VerdictDegraded
		}
		return refused(st, v, ErrCircuitOpen, 0, start), noopDone
	}

	// Fault injection is innermost: it runs only for an admitted request, and an
	// injected failure is charged to the breaker immediately (the adapter never
	// runs the handler, so there is no completion to wait for).
	if s.opts.fault != nil {
		fr, err := s.opts.fault.Fire(ctx, req)
		if err != nil {
			st.breaker.Record(OutcomeFailure, time.Since(start))
			release()
			r := refused(st, VerdictFaultInjected, err, 0, start)
			if fr.Rule != "" {
				r.rule = fr.Rule
			}
			r.fault = fr
			return r, noopDone
		}
		r := admitted(st, time.Since(start), fr)
		return r, newCompletion(ctx, st, start, release)
	}

	return admitted(st, time.Since(start), FaultResult{}), newCompletion(ctx, st, start, release)
}

func admitted(st *state, latency time.Duration, fr FaultResult) Result {
	r := Result{
		verdict: VerdictAdmitted,
		latency: latency,
		fault:   fr,
	}
	if st != nil {
		r.rule = st.rule
		r.timeout = st.policy.Timeout
	}
	return r
}

func refused(st *state, v Verdict, err error, retryAfter time.Duration, start time.Time) Result {
	r := Result{
		verdict:    v,
		err:        err,
		retryAfter: retryAfter,
		latency:    time.Since(start),
	}
	if st != nil {
		r.rule = st.rule
		r.timeout = st.policy.Timeout
	}
	return r
}

func noopDone(Outcome, error) {}

// completion is one request's completion state. It is the only per-request
// allocation the engine makes, and it is not a manager: it holds the state
// pointer, the bulkhead release, the start instant and an optional deadline
// fallback.
type completion struct {
	once    sync.Once
	st      *state
	start   time.Time
	release func()
	timer   atomic.Pointer[time.Timer]
}

// newCompletion returns the request's Done. A context with a deadline arms a
// fallback settlement so a missing Done cannot leak a bulkhead slot or strand a
// half-open trial; without a deadline the caller must settle.
func newCompletion(ctx context.Context, st *state, start time.Time, release func()) Done {
	c := &completion{st: st, start: start, release: release}
	if dl, ok := ctx.Deadline(); ok {
		if wait := time.Until(dl); wait > 0 {
			c.timer.Store(time.AfterFunc(wait, func() {
				c.settle(OutcomeUnknown, errNotSettled)
			}))
		}
	}
	return c.settle
}

// settle releases the bulkhead slot and folds the outcome into the breaker. It
// is exactly-once: a deadline fallback and an explicit Done cannot both run.
func (c *completion) settle(o Outcome, err error) {
	c.once.Do(func() {
		if t := c.timer.Load(); t != nil {
			t.Stop()
		}
		c.release()
		c.st.breaker.Record(o, time.Since(c.start))
	})
}

// compile-time assertions.
var (
	_ Guard                       = (*server)(nil)
	_ breaker.EventListenerSetter = (*server)(nil)
)
