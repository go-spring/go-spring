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
	"fmt"
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
)

// activeRateKeys is the counter-key set label resolves to under the current
// rate snapshot. Executors use it to drop local counters whose rule is gone.
func (m *Manager) activeRateKeys(label string) map[string]struct{} {
	rs := m.rateLimit.Load()
	if rs == nil {
		return nil
	}
	return rs.activeKeys(label)
}

// Manager is the OUTBOUND resilience authority: it owns the backend every
// client executor is built with, the registry of executors built for it, and the
// subscription fan-out. It is the module-level counterpart of the governance
// control plane — the center owns the config DOCUMENT (rules, matching, source),
// the manager owns what to DO with the resulting policies.
//
// It holds ONE snapshot per capability, each with its own generation. Three
// capabilities means three writers (one per control-plane root, once they are
// split), so they are kept as three independent atomic pointers: neither a
// reader nor a writer ever has to touch another capability's state.
//
// It is deliberately a different type from the inbound `admission.Manager`. The two
// directions share the neutral vocabulary and the [Driver] contract, but not
// their authority: a client starter can only reach the outbound object, while the
// server side owns one Guard per instance.
//
// Every mutation sits on a phase of its own, which is why the manager takes no
// lock: executors and subscriptions are built at wiring/init ([Manager.ExecutorFor],
// [Manager.OnLimits]), settings are pushed by each capability's source loop
// in the Runner phase ([Manager.ApplyLimits], [Manager.ApplyBreaker],
// [Manager.ApplyRateLimit]), and [Manager.Close] runs at destroy, after the loops
// have stopped. This is the same contract that leaves [fault.Manager.Refresh]
// and the loadbalance registration table unsynchronized.
//
// Safe for concurrent use.
type Manager struct {
	// driver is the backend every executor this manager hands out is built with,
	// fixed at construction. It is never nil.
	driver Driver

	// limiters collects the rate-limit limiter kinds and creates their
	// counters. The set is fixed at wiring; a rule naming a missing kind is
	// rejected when its document is applied.
	limiters *ratelimit.Factory

	// The three published snapshots. Each is written by its own capability's
	// Apply and read lock-free on the call path — one atomic load per capability,
	// never a read-modify-write.
	limits    atomic.Pointer[limitsSnapshot]
	breaker   atomic.Pointer[breakerSnapshot]
	rateLimit atomic.Pointer[rateSnapshot]

	// subs is the outbound per-label dirty-signal table (see
	// [Manager.OnLimits]). A label with no subscriber leaves no entry.
	subs map[string][]*subscriber

	// entries registers every outbound executor this process has built, keyed by
	// label. It is a sync.Map so the hot path stays lock-free: a protected call
	// loads its label's executor and is done.
	//
	// The registry is an IDENTITY table, not a change-propagation path: it
	// guarantees one executor per label, and Close drains it. A policy push does
	// not touch it — the executors follow the config themselves, through the
	// manager they hold.
	entries sync.Map // label -> *entry

	// buildMu serializes the outbound build path, so a label is built exactly
	// once even when several goroutines first use it at the same moment. Taking
	// it only on a miss leaves the hot path lock-free.
	buildMu sync.Mutex
}

// NewManager returns an outbound manager with empty snapshots that builds every
// executor with driver. driver is the backend chosen at startup — the starter
// injects the [Driver] bean the container selected from
// ${spring.resilience.driver} — and it is held for the manager's lifetime: a
// backend swap resets every executor's breaker and limiter state, so it is a
// deploy-time decision, not a pushed one.
//
// driver MUST be non-nil: it is the process's backend, not an optional feature,
// so a caller without a backend of its own passes [NewDefaultDriver] explicitly
// rather than silently here. A nil driver panics — an invalid construction is
// refused, never wrapped.
//
// The manager starts with empty snapshots, so every [Manager.ExecutorFor] yields
// a transparent pass-through until an Apply publishes a policy. Construction is
// independent of the configuration: an executor can be built before, during or
// after a push, because it reads the manager per call.
func NewManager(driver Driver, limiters *ratelimit.Factory) *Manager {
	if driver == nil {
		panic("resilience: NewManager requires a non-nil Driver; pass NewDefaultDriver() for the bundled backend")
	}
	if limiters == nil {
		panic("resilience: NewManager requires a non-nil ratelimit.Factory")
	}
	m := &Manager{driver: driver, limiters: limiters, subs: map[string][]*subscriber{}}
	// Publish zero snapshots from the start rather than leaving them absent: a
	// label's generation is then the same number an applied empty document
	// produces, so nothing depends on whether a document has arrived yet.
	m.limits.Store(&limitsSnapshot{})
	m.breaker.Store(&breakerSnapshot{})
	m.rateLimit.Store(&rateSnapshot{})
	return m
}

// ApplyLimits adopts s as the limits half of the outbound document and publishes
// it. It is the single entry point the governance center calls for this
// capability — once at startup, then on every push.
//
// Publishing is the whole of it: no registration is dropped, no executor is
// reached into. The document was validated at the source boundary (Parse →
// conf.Bind → Validate); a programmatic caller vouches for what it hands in.
// Executors follow the new generation on their next call and
// rebuild only their limits-derived values (which need no state). A push that
// changes nothing leaves every generation where it was, because generations are
// derived from content rather than from the fact that Apply ran.
func (m *Manager) ApplyLimits(s LimitsSettings) error {
	m.limits.Store(&limitsSnapshot{settings: s})
	m.notifyLimits()
	return nil
}

// ApplyBreaker adopts s as the breaker half of the outbound document. Only the
// breaker generation moves; a live rate budget or retry budget keeps counting.
// A document from a source was validated at the boundary; a programmatic caller
// vouches for what it hands in.
func (m *Manager) ApplyBreaker(s BreakerSettings) error {
	m.breaker.Store(&breakerSnapshot{settings: s})
	return nil
}

// ApplyRateLimit adopts s as the rate-limit half of the outbound document. Only
// the rate-limit generation moves; a live breaker keeps counting.
//
// Every enabled policy's counter is BUILT first, and the document is published
// only once all of them build: construction is the validation, so an unknown
// kind rejects the document whole and the previous snapshot keeps serving. The
// built counters are discarded — a counter carries per-key state that lives in
// the executors and must outlive the swap, so the executor rebuilds it from the
// rule's kind on first use.
//
// Document-shape validation happened at the source boundary.
func (m *Manager) ApplyRateLimit(s RateLimitSettings) error {
	if err := m.buildRateCounters(s); err != nil {
		return err
	}
	m.rateLimit.Store(&rateSnapshot{settings: s})
	return nil
}

// buildRateCounters builds one counter per enabled policy, proving the document
// can be served. The scope is the rule's service label — the same scope the
// executor will use.
func (m *Manager) buildRateCounters(s RateLimitSettings) error {
	if s.Default.RateLimit > 0 {
		if _, err := m.limiters.NewLimiter(s.Default.Limiter, ""); err != nil {
			return errutil.Explain(err, "resilience: default rate-limit policy")
		}
	}
	for i := range s.Rules {
		if s.Rules[i].RateLimit == 0 {
			continue
		}
		if _, err := m.limiters.NewLimiter(s.Rules[i].Policy.Limiter, s.Rules[i].Service); err != nil {
			return errutil.Explain(err, "resilience: rate-limit rule for service %q", s.Rules[i].Service)
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
// It also publishes zero snapshots, so a call that slips in after Close resolves
// the pass-through rather than the last generation, which is what keeps
// [Manager.Resolve] honest about the manager no longer serving a policy.
func (m *Manager) Close() error {
	m.limits.Store(&limitsSnapshot{})
	m.breaker.Store(&breakerSnapshot{})
	m.rateLimit.Store(&rateSnapshot{})
	m.subs = nil
	for _, e := range drainCache(&m.entries) {
		_ = e.Close()
	}
	return nil
}

// ExecutorFor returns the executor the given service should run under, building
// it NOW — the caller receives the backing executor itself, not a lazy handle.
// It is the call clients use instead of reaching for a governance center: pass
// the service label, wrap the result with fault as desired, and install it in
// the command/query path. The protocol system is deliberately NOT a parameter:
// this engine keys on the label alone, and the observer (which does need the
// system) composes around it in [go-spring.org/cloud/governance.Runtime.ExecutorFor].
//
// Construction reads no configuration, so ordering does not matter: the manager
// publishes empty snapshots in [NewManager] and every executor it builds reads
// the live ones per call. Asking before any settings arrive yields a pass-through,
// not an error. A driver that cannot build still PANICS — a silent pass-through
// would mean governance is off without anyone knowing.
//
// Hot reload needs no indirection: the object the caller holds is the shell, and
// it is never replaced — an Apply publishes a new generation and the executor
// picks it up on its next call. One label still maps to exactly ONE executor: a
// second caller for the same label receives the first's executor.
//
// The observe layer wraps the returned executor afterwards, but the two planes
// stay independent: the request observer records the call, while the executor
// publishes its own organ state from this package, so no construction-time
// handshake is needed.
func (m *Manager) ExecutorFor(service string) chain.Executor {
	if v, ok := m.entries.Load(service); ok {
		return v.(*entry).exec
	}
	m.buildMu.Lock()
	defer m.buildMu.Unlock()
	if v, ok := m.entries.Load(service); ok { // built while we waited
		return v.(*entry).exec
	}
	return m.build(service)
}

// LimitsFor returns the limits a label currently resolves to, or zero limits
// before any settings have been applied. An adapter mapping the label's deadline
// and retry schedule onto its own knobs — dubbo, typically — reads this instead
// of building an executor it will not use.
func (m *Manager) LimitsFor(label string) Limits {
	return m.limits.Load().label(label)
}

// OnLimits registers notify as a dirty signal for label's LIMITS: when a limits
// Apply changes the limits label resolves to, notify fires — with no value
// delivered. The subscriber re-reads the value itself ([Manager.LimitsFor] or
// its own poll), which is what an adapter re-deriving its own parameters from
// the config wants. Only a limits Apply notifies: the other capabilities do not
// affect the knobs this subscriber reads.
//
// Registration is pure bookkeeping — nothing is invoked here — and it runs on
// the wiring/init phase, before the source loop starts, so the table it writes
// is never written concurrently with the loop's walk.
//
// Registration is for the process lifetime: there is no cancel. An owner whose
// object is not process-lifetime must keep its notify safe to call after the
// object is discarded.
func (m *Manager) OnLimits(label string, notify func()) {
	m.subs[label] = append(m.subs[label], &subscriber{last: m.limits.Load().label(label), notify: notify})
}

// Resolve answers an outbound executor's question — what does THIS call run
// under, and which generations are those from? — lock-free over the three
// published snapshots, reached on every protected call by the executor that
// holds this manager.
//
// Each capability matches the first rule whose Service equals the call's and
// whose Action and Match the call satisfies, falling back to its default under
// the service key when none does. A manager with no settings (or a closed one)
// resolves zero policies — the transparent pass-through — so the answer never
// depends on a nil check at the call site.
func (m *Manager) Resolve(op chain.Operation) CallPolicy {
	ls, bs, rs := m.limits.Load(), m.breaker.Load(), m.rateLimit.Load()
	if ls == nil || bs == nil || rs == nil {
		return CallPolicy{
			Breaker: BreakerRef{Key: op.Service},
			Rate:    RateRef{Key: op.Service},
		}
	}
	lr := ls.resolve(op)
	br := bs.resolve(op)
	rr := rs.resolve(op)
	return CallPolicy{
		Limits:      lr,
		Breaker:     br,
		Rate:        rr,
		LabelLimits: ls.label(op.Service),
		Gen:         mixGens(lr.Gen, br.Gen, rs.gen(op.Service)),
	}
}

// ResolveLabel is for a backend that keys its own state by resource name: the
// label-level policy of every capability and its generations, with no rule
// matching. Lock-free, like [Manager.Resolve].
func (m *Manager) ResolveLabel(label string) CallPolicy {
	ls, bs, rs := m.limits.Load(), m.breaker.Load(), m.rateLimit.Load()
	if ls == nil || bs == nil || rs == nil {
		return CallPolicy{
			Breaker: BreakerRef{Key: label},
			Rate:    RateRef{Key: label},
		}
	}
	lg, bg, rg := ls.gen(label), bs.gen(label), rs.gen(label)
	lp := ls.label(label)
	rate := RateRef{Policy: rs.label(label), Key: label}
	return CallPolicy{
		Limits:      LimitsRef{Timeout: lp.Timeout, Retry: lp.Retry, Gen: lg},
		Breaker:     BreakerRef{Policy: bs.label(label), Key: label, Gen: bg},
		Rate:        rate,
		LabelLimits: lp,
		Gen:         mixGens(lg, bg, rg),
	}
}

// build builds and registers label's executor and returns it. Callers
// hold [Manager.buildMu], so the registration is the only one for its
// label.
//
// Construction reads NO configuration: the executor follows the manager's
// published snapshots on every call, starting with the first one. The manager
// therefore publishes empty snapshots in [NewManager], so asking for an executor
// before any settings arrive is legitimate — it is a pass-through, not a
// construction-order bug.
//
// A driver that cannot build PANICs rather than degrade: it is a startup defect,
// and a silent pass-through would mean governance is off without anyone knowing.
// Fail fast, fail loud.
func (m *Manager) build(label string) chain.Executor {
	e, err := m.driver.NewExecutor(label, m)
	if err != nil || e == nil {
		panic(fmt.Sprintf("resilience: driver %T failed to build executor for %q: %v", m.driver, label, err))
	}
	m.entries.Store(label, &entry{exec: e})
	return e
}

// notifyLimits fires the dirty signal of every subscribed label whose limits
// changed. It runs on the source loop's goroutine.
func (m *Manager) notifyLimits() {
	for label, list := range m.subs {
		next := m.limits.Load().label(label)
		for _, sub := range list {
			if sub.last == next {
				continue
			}
			sub.last = next
			sub.notify()
		}
	}
}
