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

package resilience

import (
	"sync/atomic"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/stdlib/errutil"
)

// Manager is the OUTBOUND resilience authority: it owns the published policy
// documents. It is the module-level counterpart of
// the governance control plane — the center owns the config DOCUMENT (rules,
// matching, source), the manager owns what to DO with the resulting policies.
//
// It holds ONE published document per capability. Three capabilities means
// three writers (one per control-plane root), so they are kept as three
// independent atomic pointers: neither a reader nor a writer ever has to touch
// another capability's state.
//
// It is deliberately a different type from the inbound `admission.Manager`. The two
// directions share the neutral policy vocabulary, but not their authority: a
// client starter can only reach the outbound object, while the server side owns
// one Guard per instance.
//
// Every mutation sits on a phase of its own, which is why the manager takes no
// lock: executors register at wiring/init ([Manager.Wrap]), settings are pushed
// by each capability's source loop in the Runner phase ([Manager.ApplyTimeoutRetry],
// [Manager.ApplyBreaker], [Manager.ApplyRateLimit]), and [Manager.Close] runs at
// destroy, after the loops have stopped. This is the same contract that leaves
// [fault.Manager.Refresh] and the loadbalance registration table unsynchronized.
//
// A push reaches the executors it built through [Manager.executors]: the shell
// is permanent (a client holds its executor for life), so what moves is the
// mutable organ inside it, never the object. This is the same shape admission
// (its Guard table) and loadbalance (its Chain table) use.
//
// Safe for concurrent use.
type Manager struct {
	// limiterFactory collects the rate-limit limiter kinds and creates their
	// counters. The set is fixed at wiring; a rule naming a missing kind is
	// rejected when its document is applied.
	limiterFactory *ratelimit.Factory

	// The three published documents. Each is written by its own capability's
	// Apply and read lock-free on the call path — one atomic load per capability,
	// never a read-modify-write.
	limits    atomic.Pointer[TimeoutRetrySettings]
	breaker   atomic.Pointer[BreakerSettings]
	rateLimit atomic.Pointer[RateLimitSettings]

	// executors is the registration table a push walks: every executor this
	// manager built. A map is fine — a push does not care about order. It is
	// mutated at wiring and destroy and read on the source loop, phases that
	// never overlap, so it is a plain map.
	executors map[*Executor]struct{}
}

// NewManager returns an outbound manager with empty documents. It owns the one
// outbound engine the process runs on: the engine follows the manager's
// published documents, so there is no backend object to select or replace.
//
// limiterFactory is the process's rate-limit factory, injected by the starter.
//
// The manager starts with empty documents, so every [Manager.Wrap] yields
// a transparent pass-through until an Apply publishes a policy. Construction is
// independent of the configuration: an executor can be built before or after a
// push, because it reads the manager per call and a later push reaches it.
func NewManager(limiterFactory *ratelimit.Factory) (*Manager, error) {
	m := &Manager{
		limiterFactory: limiterFactory,
		executors:      make(map[*Executor]struct{}),
	}
	// Publish zero documents from the start rather than leaving them absent, so
	// resolution never needs a nil check: before the first push the manager
	// serves the pass-through just as it does for an empty document.
	m.limits.Store(&TimeoutRetrySettings{})
	m.breaker.Store(&BreakerSettings{})
	m.rateLimit.Store(&RateLimitSettings{})
	return m, nil
}

// TimeoutRetrySettings returns the timeout/retry document currently published.
// One atomic load, no lock: this is the read half of the push.
func (m *Manager) timeoutRetrySettings() *TimeoutRetrySettings {
	return m.limits.Load()
}

// BreakerSettings returns the breaker document currently published.
func (m *Manager) breakerSettings() *BreakerSettings {
	return m.breaker.Load()
}

// RateLimitSettings returns the rate-limit document currently published.
func (m *Manager) rateLimitSettings() *RateLimitSettings {
	return m.rateLimit.Load()
}

// ApplyTimeoutRetry adopts s as the timeout/retry half of the outbound document
// and publishes it. It is the single entry point the governance center calls for
// this capability — once at startup, then on every push.
//
// The policy carries no materialized state, so publishing the document IS the whole
// push: the next call resolves its timeout and retry schedule from it. The
// document was validated at the source boundary (Parse → conf.Bind → Validate);
// a programmatic caller vouches for what it hands in.
func (m *Manager) ApplyTimeoutRetry(s TimeoutRetrySettings) error {
	m.limits.Store(&s)
	return nil
}

// ApplyBreaker adopts s as the breaker half of the outbound document and resets
// the breaker table of every executor whose label's breaker content moved. A
// live rate budget or retry budget keeps counting, and an executor whose content
// did not move keeps its breaker state: the comparison is by CONTENT, not by the
// fact that Apply ran.
//
// A document from a source was validated at the boundary; a programmatic caller
// vouches for what it hands in.
func (m *Manager) ApplyBreaker(s BreakerSettings) error {
	prev := m.breaker.Load()
	m.breaker.Store(&s)
	for ex := range m.executors {
		ex.resetBreakerIfChanged(prev, &s)
	}
	return nil
}

// ApplyRateLimit adopts s as the rate-limit half of the outbound document. Only
// the rate-limit organs move; a live breaker keeps counting.
//
// Every enabled policy's limiter is CHECKED first, and the document is published
// only once all of them can be built: construction is the validation, so an
// unknown kind rejects the document whole and the previous one keeps serving.
// The built limiters are discarded — a limiter carries per-key state that lives
// in the executors and must outlive the swap, so the executor rebuilds it from
// the rule's kind on first use.
//
// Once the document is published, every registered executor drops the counters
// whose key its label no longer resolves to. The sweep is idempotent and never
// touches a counter that is still active, so a push that changes nothing leaves
// every live budget counting.
//
// Document-shape validation happened at the source boundary.
func (m *Manager) ApplyRateLimit(s RateLimitSettings) error {
	if err := m.checkRateLimiters(s); err != nil {
		return err
	}
	m.rateLimit.Store(&s)
	for ex := range m.executors {
		ex.deleteUnusedLimiters(s)
	}
	return nil
}

// checkRateLimiters proves the document can be served by building one limiter
// per enabled policy and discarding it. The key is the one the executor will
// charge, so the check exercises the same scope.
func (m *Manager) checkRateLimiters(s RateLimitSettings) error {
	for i := range s.Services {
		g := &s.Services[i]
		for j := range g.Rules {
			r := &g.Rules[j]
			if r.RateLimit == 0 {
				continue
			}
			key := ruleKey(g.Service, r.ActionMatch)
			if _, err := m.limiterFactory.NewLimiter(r.Policy.Limiter, key); err != nil {
				return errutil.Explain(err, "resilience: rate-limit rule for service %q", g.Service)
			}
		}
	}
	return nil
}

// Close releases every registered outbound executor. The manager itself holds
// only in-memory state, so this is the shutdown counterpart to the Apply family:
// it lets a production driver stop background resources (metric pumps) instead
// of leaking them with the process.
//
// It is the manager BEAN's destroy method (see the starter), so the container
// calls it on shutdown — after the clients that hold those executors, since gs
// tears beans down in reverse construction order.
//
// It also publishes zero documents and drops the registration table, so a call
// that slips in after Close resolves the pass-through rather than the last
// document, which is what keeps a late caller honest about the manager no longer
// serving a policy.
func (m *Manager) Close() error {
	m.limits.Store(&TimeoutRetrySettings{})
	m.breaker.Store(&BreakerSettings{})
	m.rateLimit.Store(&RateLimitSettings{})
	m.executors = make(map[*Executor]struct{})
	return nil
}
