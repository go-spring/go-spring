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

	"go-spring.org/cloud/runtime/dataplane/outbound/chain"
)

// Manager is the OUTBOUND resilience authority: it owns the backend every
// client executor is built with, the registry of executors built for it, and the
// subscription fan-out. It is the module-level counterpart of the governance
// control plane — the center owns the config DOCUMENT (rules, matching, source),
// the manager owns what to DO with the resulting policies.
//
// It is deliberately a different type from the inbound `admission.Manager`. The two
// directions share the neutral Settings vocabulary and the [Driver] contract,
// but not their authority: a client starter can only reach the outbound object,
// while the server side owns one Guard per instance.
//
// Safe for concurrent use.
type Manager struct {
	// mu guards the pushed settings and the subscriber table. The hot path — a
	// protected call — does not take it: an armed manager is read lock-free
	// through the atomic snapshot.
	mu sync.Mutex

	// applied reports whether a Settings has been adopted and not since
	// closed. It is the build decision in buildLocked: an unarmed manager
	// publishes a literal no-op executor rather than asking the driver for one.
	applied bool

	// driver is the backend every executor this manager hands out is built with,
	// fixed at construction. It is never nil.
	driver Driver

	// settings is the outbound half last pushed by [Manager.Apply], held
	// whole — the manager stores what it was handed, it does not take it apart.
	// Its zero value is the pass-through: the state before any Apply, and after
	// Close. Replaced wholesale, never merged. Guarded by mu.
	settings Settings

	// snap is the outbound settings as the executors SEE them: one generation,
	// published whole, read lock-free by every [PolicySource.Resolve] on the call
	// path. An executor must not take mu to resolve — it resolves outside its own
	// lock, and taking the manager's there would invert the lock order against
	// the manager's build path. It is a heap object replaced wholesale on each
	// Apply, exactly like settings.
	snap atomic.Pointer[snapshot]

	// subs is the outbound per-label subscriber table (see
	// [Manager.Subscribe]). A label with no subscriber leaves no entry, so
	// a repeatedly-rebuilt client leaves no residue. Guarded by mu.
	subs map[string][]*subscriber

	// entries registers every outbound executor this process has built, keyed by
	// label. It is a sync.Map so the hot path stays lock-free: a protected call
	// loads its label's executor and is done.
	//
	// The registry is an IDENTITY table, not a change-propagation path: it
	// guarantees one executor per label, and Close drains it. A policy push does
	// not touch it — the executors follow the config themselves, through the
	// [PolicySource] they hold.
	entries sync.Map // label -> *entry

	// buildMu serializes the outbound build path, so a label is built exactly
	// once even when several goroutines first use it at the same moment. Taking
	// it only on a miss leaves the hot path lock-free. Lock order is
	// buildMu → mu: nothing acquires them the other way round (a manager-level
	// operation never enters the build path).
	buildMu sync.Mutex
}

// NewManager returns an unarmed outbound manager that builds every
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
// The manager is unarmed: it holds no registrations and no settings, so every
// [Manager.ExecutorFor] yields a transparent pass-through until
// [Manager.Apply] arms it.
func NewManager(driver Driver) *Manager {
	if driver == nil {
		panic("resilience: NewManager requires a non-nil Driver; pass NewDefaultDriver(nil) for the bundled backend")
	}
	m := &Manager{driver: driver, subs: map[string][]*subscriber{}}
	// Publish the zero document from the start rather than leaving the snapshot
	// absent: a label's generation on an unarmed manager is then the same number
	// an armed one with an empty document produces, so nothing depends on whether
	// a document has arrived yet.
	m.snap.Store(newSnapshot(Settings{}))
	return m
}

// Apply adopts s as the outbound settings, publishes them as the generation the
// executors will follow, and notifies the subscribers whose policy changed. It
// is the single entry point the governance center calls — once at startup with
// the source's snapshot, then on every push.
//
// Publishing is the whole of it. No registration is dropped, no executor is
// reached into, and no lock the call path takes is held: the executors follow
// the new generation on their own next call, each rebuilding the state of the
// labels whose rule set actually moved. A push that changes nothing for a label
// leaves that label's breaker counting, because the generation is derived from
// content (see [labelGen]) rather than from the fact that Apply ran.
//
// The executors are not closed here — [Manager.Close] is what closes.
//
// Apply always succeeds: the document's only rejectable field was the backend,
// which now lives at startup, so there is nothing left to validate here. The
// error return is kept for the caller's convenience (the center treats Apply as
// infallible) and is always nil.
func (m *Manager) Apply(s Settings) error {
	snap := newSnapshot(s)

	m.mu.Lock()
	m.settings = s
	m.applied = true
	// Publish the new generation for the executors: a fresh heap object swapped
	// in one store, so an executor resolving a call never races a push and never
	// takes mu. An executor that reads the old generation for one more call is
	// correct by construction, because everything it builds from one snapshot
	// stays consistent with it.
	m.snap.Store(snap)
	todo := m.collectSubsLocked()
	m.mu.Unlock()

	for _, p := range todo {
		p.cb(p.policy)
	}
	return nil
}

// Close releases every registered outbound executor. The manager itself holds
// only in-memory state, so this is the shutdown counterpart to
// [Manager.Apply]: it lets a production driver stop background resources
// (metric pumps) instead of leaking them with the process.
//
// It is the manager BEAN's destroy method (see the starter), so the container
// calls it on shutdown — after the clients that hold those executors, since gs
// tears beans down in reverse construction order.
//
// It also publishes the zero document, so a call that slips in after Close
// resolves the pass-through rather than the last generation: "closed" and
// "unarmed" answer the same thing, which is what keeps [Manager.Resolve]
// honest about the manager no longer being armed.
func (m *Manager) Close() error {
	m.mu.Lock()
	m.applied = false
	m.settings = Settings{}
	m.snap.Store(newSnapshot(Settings{}))
	m.subs = map[string][]*subscriber{}
	execs := drainCache(&m.entries)
	m.mu.Unlock()

	for _, e := range execs {
		_ = e.Close()
	}
	return nil
}

// ExecutorFor returns the executor the given system's service should run
// under, building it NOW — the caller receives the backing executor itself, not
// a lazy handle. It is the call clients use instead of reaching for a governance
// center: pass the owning system (e.g. "redis", "gorm") and the service label,
// wrap the result with fault as desired, and install it in the command/query
// path.
//
// Eager is correct because the caller comes AFTER the config in every wired
// path: the governance center arms this manager in its own constructor, and a
// client bean is its downstream — so by the time a client asks for its executor,
// the manager is armed. A standalone caller must honor the same order: arm first
// ([Manager.Apply]), then ask. Asking on an unarmed manager, or with a
// driver that cannot build, PANICS — governance silently off is the one outcome
// this package refuses to produce.
//
// Hot reload needs no indirection: the object the caller holds is the shell, and
// it is never replaced — [Manager.Apply] publishes a new generation and the
// executor picks it up on its next call. One label still maps to exactly ONE
// executor: a second caller for the same label receives the first's executor.
//
// The observe layer attaches its breaker listener AFTER this returns (via
// [breaker.EventListenerSetter]) — sound because the organs an executor builds
// from its policy are lazy, created on the first Execute, after composition is
// complete.
func (m *Manager) ExecutorFor(system, service string) chain.Executor {
	_ = system // the system label is the observer's concern; this engine keys on label alone
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

// PolicyFor returns the outbound policy label currently resolves to, or a
// zero policy on an unarmed manager. Callers that only need to READ the policy —
// an adapter mapping it onto its own knobs — use this instead of building an
// executor they will not use.
func (m *Manager) PolicyFor(label string) Policy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.policyLocked(label)
}

// Subscribe registers cb for OUTBOUND policy changes of label and arms it
// immediately with the current resolved policy. cb is then invoked whenever
// [Manager.Apply] produces a different policy for that label. Callers that
// own their protected object rather than a [chain.Executor] — a gateway
// rebuilding a route table, a balancer re-deriving its strategy — use this to
// follow the config without an executor.
//
// cb is always called outside the manager's lock, so a callback that itself
// calls into the manager cannot self-deadlock; it MUST be safe for concurrent
// invocation, since an Apply may fire concurrently with the immediate call.
//
// The returned [Subscription] carries the policy cb was armed with and detaches
// the registration on [Subscription.Cancel]. A caller whose object is not
// process-lifetime MUST cancel, or the manager keeps a reference to the
// discarded object forever.
//
// Inbound has no counterpart: an inbound engine follows the config through its
// own registration, and nothing outside this package observes inbound changes.
func (m *Manager) Subscribe(label string, cb func(Policy)) Subscription {
	m.mu.Lock()
	// Arm the subscriber with the label's current policy under the lock, so the
	// immediate cb below and any Apply racing it agree on the first delivery.
	sub := &subscriber{last: m.policyLocked(label), cb: cb}
	m.subs[label] = append(m.subs[label], sub)
	m.mu.Unlock()
	cb(sub.last)
	return Subscription{Policy: sub.last, cancel: func() { m.unsubscribe(label, sub) }}
}

// policyLocked returns the outbound policy label resolves to right now: the
// first Rule whose Service equals the label, else the default. Callers hold the
// manager lock; the hot path does not come here.
//
// The match is plain equality — an empty Service matches nothing, deliberately:
// a rule list must not govern every service by accident (Default is how a
// document says "everything else").
func (m *Manager) policyLocked(label string) Policy {
	for _, r := range m.settings.Rules {
		if r.Service == label {
			return r.Policy
		}
	}
	return m.settings.Default
}

// Resolve answers an outbound executor's question — what does THIS call run
// under, and which generation is that from? — WITHOUT taking mu. It is the
// lock-free read path of the whole layer: the twin of
// [Manager.policyLocked] (which is per label, and runs under the lock for
// the manager's own callers), reached on every protected call through the
// [PolicySource] each executor holds.
//
// It matches the first rule whose Service equals the call's and whose Action and
// Match the call satisfies, falling back to the default under the service key
// when none does. An unarmed (or closed) manager resolves the zero policy — the
// transparent pass-through — so the answer never depends on a nil check at the
// call site.
func (m *Manager) Resolve(op chain.Operation) CallPolicy {
	s := m.snap.Load()
	if s == nil {
		return CallPolicy{Key: op.Service}
	}
	return s.resolve(op)
}

// ResolveLabel is [PolicySource] for a backend that keys its own state by
// resource name: the label-level policy and its generation, with no rule
// matching. Lock-free, like [Manager.Resolve].
func (m *Manager) ResolveLabel(label string) CallPolicy {
	s := m.snap.Load()
	if s == nil {
		return CallPolicy{Key: label}
	}
	return s.resolveLabel(label)
}

// buildLocked builds and registers label's executor and returns it. Callers
// hold [Manager.buildMu], so the registration is the only one for its
// label.
//
// Both failure modes PANIC rather than degrade: asking an unarmed manager is a
// construction-order bug (the center arms the manager before any client asks),
// and a driver that cannot build is a startup defect — a silent pass-through
// would mean governance is off without anyone knowing. Fail fast, fail loud.
//
// The policy it reads here is the executor's CONSTRUCTION-TIME policy: what a
// driver that does not implement [PolicySourceSetter] runs every call under for
// ever, and what a driver that does implement it uses only as the pre-first-call
// value, since the first call resolves the live one.
//
// There is no re-read loop after the build: a push landing in between is no
// longer a hazard, because the executor does not capture a policy at build time
// — it resolves one per call.
func (m *Manager) buildLocked(label string) chain.Executor {
	m.mu.Lock()
	if !m.applied {
		m.mu.Unlock()
		panic("resilience: asking an unarmed manager for an executor — Apply settings first")
	}
	policy := m.policyLocked(label)
	m.mu.Unlock()

	e, err := m.driver.NewExecutor(label, policy)
	if err != nil || e == nil {
		panic(fmt.Sprintf("resilience: driver %T failed to build executor for %q: %v", m.driver, label, err))
	}
	// Install the policy source while the executor is still private — the same
	// construction-time handshake, at the same moment, as the breaker listener
	// the observe layer attaches. Drivers without the capability (an external
	// backend) are skipped: their executors keep the construction-time policy,
	// key on the label alone, and do not follow the config.
	if setter, ok := e.(PolicySourceSetter); ok {
		setter.SetPolicySource(m)
	}
	m.entries.Store(label, &entry{exec: e})
	return e
}

// collectSubsLocked gathers the subscribers whose policy for their label
// changed, updating each one's last-delivered policy. Callers hold the manager
// lock and MUST deliver the returned notifications after unlocking.
func (m *Manager) collectSubsLocked() []pending {
	var todo []pending
	for label, list := range m.subs {
		next := m.policyLocked(label)
		for _, sub := range list {
			// A subscriber cancelled since this apply began must not be notified:
			// its owner is already tearing down what cb drives.
			if sub.cancelled {
				continue
			}
			if sub.last != next {
				sub.last = next
				todo = append(todo, pending{cb: sub.cb, policy: next})
			}
		}
	}
	return todo
}

// unsubscribe detaches sub from label. It marks sub cancelled before removing
// it, so an apply that already collected sub into its pending list still skips
// it at delivery time. Removing the last subscriber for a label drops the map
// entry, so a repeatedly-rebuilt client leaves no residue — not even an empty
// slice.
func (m *Manager) unsubscribe(label string, sub *subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
