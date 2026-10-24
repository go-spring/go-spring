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
// long-lived state manager. It owns the instance's rate budget, bulkhead and the
// policy generation it materialized. Nothing here allocates a manager per
// request; a request only gets a [Result] and a completion closure that points
// back at the state it borrowed from.

package admission

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/metric"

	"go-spring.org/cloud/runtime/dataplane/kernel/bulkhead"
	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/stdlib/errutil"
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
	policy Policy
	rule   string
	rate   *RateLimiter
	bulk   *bulkhead.Bulkhead
}

type server struct {
	id   Identity
	opts options
	mgr  *Manager

	mu     sync.Mutex
	closed bool
	st     *state

	// observeOnce and observeReg are this instance's own state plane (see
	// observe.go): registered lazily on the first Admit, unregistered by Close.
	observeOnce sync.Once
	observeReg  metric.Registration
}

// New builds a standalone Guard with a fixed policy. It is not connected to a
// [Manager], so the policy never changes; use [Manager.GuardFor] when the
// instance follows a pushed configuration.
//
// An error means the policy cannot be built (a negative max-concurrent); a
// valid policy always yields a Guard.
func New(id Identity, p Policy, opts ...Option) (Guard, error) {
	o := options{}
	for _, f := range opts {
		f(&o)
	}
	st, err := buildState(p, "", id.Service, o.limiter)
	if err != nil {
		return nil, err
	}
	return &server{id: id, opts: o, st: st}, nil
}

// buildState materializes one generation of the instance's stages.
func buildState(p Policy, rule, service string, limiter ratelimit.ClusterLimiter) (*state, error) {
	if p.MaxConcurrent < 0 {
		return nil, errutil.Explain(nil, "admission: max-concurrent must be >= 0")
	}
	var bulk *bulkhead.Bulkhead
	if p.MaxConcurrent > 0 {
		bulk = bulkhead.New(p.MaxConcurrent, bulkhead.WithQueueTimeout(p.BulkheadQueueDuration))
	}
	return &state{
		policy: p,
		rule:   rule,
		rate:   NewRateLimiter(p, service, limiter),
		bulk:   bulk,
	}, nil
}

// ValidateSettings reports whether every policy in s can be built into a Guard.
// It is the pre-commit check a configuration center can run before pushing s,
// so a document is rejected before it reaches [Manager.Apply].
func ValidateSettings(s Settings) error {
	if err := validatePolicy(s.Default); err != nil {
		return errutil.Explain(err, "admission: default policy")
	}
	for i := range s.Rules {
		if err := validatePolicy(s.Rules[i].Policy); err != nil {
			return errutil.Explain(err, "admission: rule %q", s.Rules[i].Service)
		}
	}
	return nil
}

// validatePolicy runs the same checks construction does, without keeping the
// stages.
func validatePolicy(p Policy) error {
	if p.MaxConcurrent < 0 {
		return errutil.Explain(nil, "admission: max-concurrent must be >= 0")
	}
	return nil
}

// Close marks the instance closed, releases its state observer and unregisters
// it from its manager.
func (s *server) Close() error {
	s.closeObserve()
	if s.mgr != nil {
		s.mgr.unregister(s)
	}
	return nil
}

// closeLocal marks the instance closed without reaching back into the manager;
// the manager's own Close uses it while holding the registry's lock.
func (s *server) closeLocal() {
	s.closeObserve()
}

// Admit runs the pre-handler stages in their fixed order — bulkhead, rate
// limit, fault — and returns this request's judgment.
func (s *server) Admit(ctx context.Context, req Request) (Result, Done) {
	s.readyObserve()
	s.mu.Lock()
	st := s.st
	s.mu.Unlock()
	if st == nil {
		return admitted(st, 0, FaultResult{}), func() {}
	}

	start := time.Now()

	// Bulkhead: one slot is held for the whole request and returned by Done. A
	// nil stage is no stage at all; the release stays a no-op so the rest of the
	// path is unconditional.
	release := func() {}
	if st.bulk != nil {
		var ok bool
		release, ok = st.bulk.Acquire(ctx)
		if !ok {
			return refused(st, VerdictBulkheadFull, ErrBulkheadFull, st.bulk.QueueTimeout(), start),
				func() {}
		}
	}

	// Rate limit: the request is charged before anything downstream runs.
	if !st.rate.Allow(ctx) {
		release()
		return refused(st, VerdictRateLimited, ErrRateLimited, st.policy.RateLimitMaxWait, start),
			func() {}
	}

	// Fault injection is innermost: it runs only for an admitted request. An
	// injected failure still returns the bulkhead slot the request borrowed.
	if s.opts.fault != nil {
		fr, err := s.opts.fault.Fire(ctx, req)
		if err != nil {
			release()
			r := refused(st, VerdictFaultInjected, err, 0, start)
			if fr.Rule != "" {
				r.rule = fr.Rule
			}
			r.fault = fr
			return r, func() {}
		}
		r := admitted(st, time.Since(start), fr)
		return r, newCompletion(ctx, release)
	}

	return admitted(st, time.Since(start), FaultResult{}), newCompletion(ctx, release)
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

// completion is one request's completion state. It is the only per-request
// allocation the engine makes, and it is not a manager: it holds the bulkhead
// release and an optional deadline fallback.
type completion struct {
	once    sync.Once
	release func()
	timer   atomic.Pointer[time.Timer]
}

// newCompletion returns the request's Done. A context with a deadline arms a
// fallback release so a missing Done cannot leak a bulkhead slot; without a
// deadline the caller must settle.
func newCompletion(ctx context.Context, release func()) Done {
	c := &completion{release: release}
	if dl, ok := ctx.Deadline(); ok {
		if wait := time.Until(dl); wait > 0 {
			c.timer.Store(time.AfterFunc(wait, c.settle))
		}
	}
	return c.settle
}

// settle returns the bulkhead slot. It is exactly-once: a deadline fallback and
// an explicit Done cannot both run.
func (c *completion) settle() {
	c.once.Do(func() {
		if t := c.timer.Load(); t != nil {
			t.Stop()
		}
		c.release()
	})
}

// compile-time assertion.
var _ Guard = (*server)(nil)
