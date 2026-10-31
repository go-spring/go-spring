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

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

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
// [Manager.SubscribeLimits]), settings are pushed by each capability's source loop
// in the Runner phase ([Manager.ApplyLimits], [Manager.ApplyBreaker],
// [Manager.ApplyRateLimit]), and [Manager.Close] runs at destroy, after the loops
// have stopped. A Subscribe callback may re-enter the manager on the loop's own
// goroutine — the same thread that walks the table — so even that needs no
// synchronization. This is the same contract that leaves [fault.Manager.Refresh]
// and the loadbalance registration table unsynchronized.
//
// Safe for concurrent use.
type Manager struct {
	// driver is the backend every executor this manager hands out is built with,
	// fixed at construction. It is never nil.
	driver Driver

	// The three published snapshots. Each is written by its own capability's
	// Apply and read lock-free on the call path — one atomic load per capability,
	// never a read-modify-write.
	limits    atomic.Pointer[limitsSnapshot]
	breaker   atomic.Pointer[breakerSnapshot]
	rateLimit atomic.Pointer[rateSnapshot]

	// subs is the outbound per-label subscriber table (see
	// [Manager.SubscribeLimits]). A label with no subscriber leaves no entry, so
	// a repeatedly-rebuilt client leaves no residue.
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
func NewManager(driver Driver) *Manager {
	if driver == nil {
		panic("resilience: NewManager requires a non-nil Driver; pass NewDefaultDriver(nil) for the bundled backend")
	}
	m := &Manager{driver: driver, subs: map[string][]*subscriber{}}
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
// reached into. Executors follow the new generation on their next call and
// rebuild only their limits-derived values (which need no state). A push that
// changes nothing leaves every generation where it was, because generations are
// derived from content rather than from the fact that Apply ran.
func (m *Manager) ApplyLimits(s LimitsSettings) error {
	if err := ValidateLimits(s); err != nil {
		return err
	}
	m.limits.Store(&limitsSnapshot{settings: s})
	m.notifyLimits()
	return nil
}

// ApplyBreaker adopts s as the breaker half of the outbound document. Only the
// breaker generation moves; a live rate budget or retry budget keeps counting.
func (m *Manager) ApplyBreaker(s BreakerSettings) error {
	if err := ValidateBreaker(s); err != nil {
		return err
	}
	m.breaker.Store(&breakerSnapshot{settings: s})
	return nil
}

// ApplyRateLimit adopts s as the rate-limit half of the outbound document. Only
// the rate-limit generation moves; a live breaker keeps counting.
func (m *Manager) ApplyRateLimit(s RateLimitSettings) error {
	if err := ValidateRateLimit(s); err != nil {
		return err
	}
	m.rateLimit.Store(&rateSnapshot{settings: s})
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
	return m.buildLocked(service)
}

// LimitsFor returns the limits a label currently resolves to, or zero limits
// before any settings have been applied. An adapter mapping the label's deadline
// and retry schedule onto its own knobs — dubbo, typically — reads this instead
// of building an executor it will not use.
func (m *Manager) LimitsFor(label string) Limits {
	return m.limits.Load().label(label)
}

// SubscribeLimits registers cb for changes to label's LIMITS and arms it
// immediately with the current value. Only a limits Apply notifies: the other
// capabilities do not affect the knobs this subscriber reads.
//
// Callers that own their protected object rather than a [chain.Executor] — an
// adapter re-deriving its own parameters — use this to follow the config without
// an executor.
//
// cb is always called outside any manager state, so a callback that itself calls
// into the manager cannot self-deadlock; it MUST be safe for concurrent
// invocation, since an Apply may fire concurrently with the immediate call.
//
// The returned [Subscription] carries the limits cb was armed with and detaches
// the registration on [Subscription.Cancel]. A caller whose object is not
// process-lifetime MUST cancel, or the manager keeps a reference to the
// discarded object forever.
func (m *Manager) SubscribeLimits(label string, cb func(Limits)) Subscription {
	// Subscribe runs on the wiring/init phase, before the source loop starts, so
	// the table it writes is never written concurrently with the loop's walk —
	// and a callback that re-enters Subscribe (dubbo's poller does) runs on the
	// loop's own goroutine, where same-thread map growth during iteration is
	// safe by the language's contract.
	sub := &subscriber{last: m.limits.Load().label(label), cb: cb}
	m.subs[label] = append(m.subs[label], sub)
	cb(sub.last)
	return Subscription{Limits: sub.last, cancel: func() { m.unsubscribe(label, sub) }}
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
		Gen:         mixGens(lr.Gen, br.Gen, rr.Gen),
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
	return CallPolicy{
		Limits:      LimitsRef{Timeout: lp.Timeout, Retry: lp.Retry, Gen: lg},
		Breaker:     BreakerRef{Policy: bs.label(label), Key: label, Gen: bg},
		Rate:        RateRef{Policy: rs.label(label), Key: label, Gen: rg},
		LabelLimits: lp,
		Gen:         mixGens(lg, bg, rg),
	}
}

// buildLocked builds and registers label's executor and returns it. Callers
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
func (m *Manager) buildLocked(label string) chain.Executor {
	e, err := m.driver.NewExecutor(label, m)
	if err != nil || e == nil {
		panic(fmt.Sprintf("resilience: driver %T failed to build executor for %q: %v", m.driver, label, err))
	}
	m.entries.Store(label, &entry{exec: e})
	return e
}

// notifyLimits delivers the new limits of every subscribed label whose value
// changed. It runs on the source loop's goroutine.
func (m *Manager) notifyLimits() {
	for label, list := range m.subs {
		next := m.limits.Load().label(label)
		for _, sub := range list {
			// A subscriber cancelled since this apply began must not be notified:
			// its owner is already tearing down what cb drives.
			if sub.cancelled || sub.last == next {
				continue
			}
			sub.last = next
			sub.cb(next)
		}
	}
}

// unsubscribe detaches sub from label. It marks sub cancelled before removing
// it, so an apply that already collected sub still skips it at delivery time.
// Removing the last subscriber for a label drops the map entry, so a
// repeatedly-rebuilt client leaves no residue — not even an empty slice.
func (m *Manager) unsubscribe(label string, sub *subscriber) {
	sub.cancelled = true
	list := m.subs[label]
	for i, e := range list {
		if e == sub {
			list = append(list[:i:i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(m.subs, label)
		return
	}
	m.subs[label] = list
}
