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
	"context"
	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/observability"
	"sync"
)

// clientSide is the manager's outbound half: the resolver the governance center
// supplies, the registry of executors this process built for its own calls, and
// the subscriber table for callers that follow the policy without owning an
// executor.
//
// serverSide (server_manager.go) is the inbound half, and the two share NO state:
// a label resolved here as an outbound executor and the same string resolved as
// inbound inbound are different protection objects running different policies,
// built by different driver methods and wrapped by different observe layers.
// They are two types, not one type used twice, because their models are
// different types — nothing is expressed in terms of the other.
type clientSide struct {
	// resolve maps a label to the policy it runs under, or nil when this side is
	// unarmed (a disabled center), in which case every label falls to the
	// pass-through. Replaced wholesale by [Manager.Apply].
	resolve func(label string) ClientPolicy

	// subs is the per-label subscriber table (see [Manager.Subscribe]). A label
	// with no subscriber leaves no entry, so a repeatedly-rebuilt client leaves no
	// residue. Guarded by [Manager.mu].
	subs map[string][]*subscriber

	// entries registers every executor this side has built, keyed by label, each
	// with the policy it was built from. It is a sync.Map so the hot path stays
	// lock-free: a protected call loads its label's executor and is done.
	//
	// This registry is where a policy change lands. [Manager.Apply] walks it and
	// drops every entry whose policy is no longer what its label resolves to, so
	// that label's next call builds a fresh executor under the new policy — while
	// an entry that survives keeps its executor, and with it its breaker and
	// rate-limit state, so a push touching one label does not reset another.
	entries sync.Map // label -> *clientEntry

	// buildMu serializes the build path, so a label is built exactly once even
	// when several goroutines first use it at the same moment. Taking it only on
	// a miss leaves the hot path lock-free. Lock order is buildMu → [Manager.mu]:
	// nothing acquires them the other way round (a manager-level operation never
	// enters the build path).
	buildMu sync.Mutex
}

// clientEntry is one label's outbound registration: the executor built for it —
// already instrumented — and the policy it was built from, which is how
// [Manager.Apply] tells which registrations a push invalidates. Entries are held
// by POINTER: a policy is a wide struct, and the hot path must not copy one on
// every call just to reach the executor inside.
type clientEntry struct {
	exec   chain.Executor
	policy ClientPolicy
}

// policyLocked returns the policy label resolves to right now, or the zero policy
// on an unarmed side. Callers hold the manager lock; the hot path does not come
// here.
func (s *clientSide) policyLocked(label string) ClientPolicy {
	if s.resolve == nil {
		return ClientPolicy{}
	}
	return s.resolve(label)
}

// backing returns the executor label runs under, building and registering it on
// first use. With no resolver it publishes a pass-through, so an unarmed manager
// costs one atomic load per call.
//
// The observe layer is applied by [clientSide.buildLocked], on the still-private
// executor, before it is registered. That ordering is what lets the bundled driver
// attach a breaker listener through the construction-time handshake while the
// breaker does not exist yet, instead of a client trying to reach an executor
// already behind a wrapper chain.
func (s *clientSide) backing(m *Manager, label, system string) chain.Executor {
	if v, ok := s.entries.Load(label); ok {
		return v.(*clientEntry).exec
	}
	s.buildMu.Lock()
	defer s.buildMu.Unlock()
	if v, ok := s.entries.Load(label); ok { // built while we waited
		return v.(*clientEntry).exec
	}
	return s.buildLocked(m, label, system)
}

// buildLocked builds, wraps and registers label's executor and returns it. The
// pass-through is what an unarmed side publishes, and what a driver that cannot
// build one leaves the label on; it is wrapped and registered like any other
// executor, so a client on an unarmed manager is still traced and measured, and
// the wrapper is not rebuilt per call. Callers hold [clientSide.buildMu], so the
// registration is the only one for its label.
//
// It re-reads the resolver after the driver has built, because an apply may have
// landed in between: an executor holding a superseded policy would serve its label
// one generation behind until the next push, and [Manager.Apply]'s sweep cannot
// see it — that walk ended before this registration existed. Building again is the
// whole fix, and it is bounded: another round needs another apply.
func (s *clientSide) buildLocked(m *Manager, label, system string) chain.Executor {
	for {
		m.mu.Lock()
		resolve := s.resolve
		m.mu.Unlock()
		policy := ClientPolicy{}
		if resolve != nil {
			policy = resolve(label)
		}
		e := chain.Executor(noopClientExecutor{})
		if resolve != nil {
			if d, err := m.driverFor(m.Driver()); err == nil && d != nil {
				if built, err := d.NewClientExecutor(label, policy); err == nil && built != nil {
					e = built
				}
			}
		}
		m.mu.Lock()
		armed := s.resolve != nil
		current := s.policyLocked(label)
		m.mu.Unlock()
		if armed != (resolve != nil) || current != policy {
			continue
		}
		registered := observability.WrapClientExecutor(e, system, label)
		s.entries.Store(label, &clientEntry{exec: registered, policy: policy})
		return registered
	}
}

// evictChangedLocked drops every registration whose label no longer resolves to
// the policy it was built under. Callers hold the manager lock. The executors
// dropped here are not closed: [Manager.Close] is what closes, the same treatment
// [Manager.Apply] gives them when the backend itself changes.
func (s *clientSide) evictChangedLocked() {
	s.entries.Range(func(key, value any) bool {
		if v := value.(*clientEntry); v.policy != s.policyLocked(key.(string)) {
			s.entries.Delete(key)
		}
		return true
	})
}

// addSubLocked registers cb for label and arms it with the label's current policy.
// Callers must hold the manager lock.
func (s *clientSide) addSubLocked(label string, cb func(ClientPolicy)) *subscriber {
	sub := &subscriber{last: s.policyLocked(label), cb: cb}
	s.subs[label] = append(s.subs[label], sub)
	return sub
}

// collectLocked gathers the subscribers whose policy for their label changed,
// updating each one's last-delivered policy. Callers hold the manager lock
// and MUST deliver the returned notifications after unlocking.
func (s *clientSide) collectLocked() []pending {
	var todo []pending
	for label, list := range s.subs {
		next := s.policyLocked(label)
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

// unsubscribe detaches sub from label. It marks sub cancelled before removing it,
// so an apply that already collected sub into its pending list still skips it at
// delivery time. Removing the last subscriber for a label drops the map entry, so
// a repeatedly-rebuilt client leaves no residue — not even an empty slice.
func (s *clientSide) unsubscribe(m *Manager, label string, sub *subscriber) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sub.cancelled = true
	list := s.subs[label]
	for i, e := range list {
		if e == sub {
			list = append(list[:i:i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(s.subs, label)
		return
	}
	s.subs[label] = list
}

// ClientExecutorFor returns the executor the given system's service should run under.
// It is the call clients use instead of reaching for a governance center: pass
// the owning system (e.g. "redis", "gorm") and the service label, wrap the
// result with fault as desired, and install it in the command/query path.
//
// The returned executor resolves its backing implementation LAZILY, on each
// Execute, through the manager's per-label registry. That indirection is
// load-bearing, not an optimization: a client bean is constructed before the
// container has adopted the config (gs builds every bean, then runs the Init
// hooks), so an eager build would latch the unarmed pass-through permanently.
// Resolving at call time — after the whole container has wired — makes the client
// setup order irrelevant.
//
// Hot-reload is handled here, not by the caller: [Manager.Apply] drops the
// registration this handle resolves through, so the next call builds the executor
// the new policy asks for.
func (m *Manager) ClientExecutorFor(system, service string) chain.Executor {
	return &managedClientExecutor{mgr: m, system: system, label: service}
}

// NewClientExecutor builds an executor from an explicit policy, bypassing label
// resolution. It is the escape hatch for a caller that has a policy in hand but
// no service label to resolve one from; service is what the executor will be bound
// to (see [chain.Executor]). The returned executor is not registered and not
// dropped by [Manager.Apply] — the caller owns it, so a caller that needs one
// executor per service builds one per service.
func (m *Manager) NewClientExecutor(service string, p ClientPolicy) (chain.Executor, error) {
	m.mu.Lock()
	driver := m.driverName
	m.mu.Unlock()
	d, err := m.driverFor(driver)
	if err != nil {
		return nil, err
	}
	return d.NewClientExecutor(service, p)
}

// ClientPolicyFor returns the outbound policy label currently resolves to, or a
// zero policy on an unarmed manager. Callers that only need to READ the policy —
// an adapter mapping it onto its own knobs — use this instead of building an
// executor they will not use.
func (m *Manager) ClientPolicyFor(label string) ClientPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.policyLocked(label)
}

// Subscribe registers cb for OUTBOUND policy changes of label and arms it
// immediately with the current resolved policy. cb is then invoked whenever
// [Manager.Apply] produces a different policy for that label. Callers that own
// their protected object rather than an [chain.Executor] — a gateway rebuilding a
// route table, a balancer re-deriving its strategy — use this to follow the
// config without an executor.
//
// cb is always called outside the manager's lock, so a callback that itself calls
// into the Manager cannot self-deadlock; it MUST be safe for concurrent
// invocation, since an Apply may fire concurrently with the immediate call.
//
// The returned [Subscription] carries the policy cb was armed with and detaches
// the registration on [Subscription.Cancel]. A caller whose object is not
// process-lifetime MUST cancel, or the Manager keeps a reference to the discarded
// object forever.
//
// Inbound inbound has no counterpart: an inbound executor follows the config
// through its own registration, and nothing outside this package observes
// inbound changes.
func (m *Manager) Subscribe(label string, cb func(ClientPolicy)) Subscription {
	m.mu.Lock()
	sub := m.client.addSubLocked(label, cb)
	m.mu.Unlock()
	cb(sub.last)
	return Subscription{ClientPolicy: sub.last, cancel: func() { m.client.unsubscribe(m, label, sub) }}
}

// drainClientCache empties the outbound registry and returns what was in it.
func drainClientCache(c *sync.Map) []chain.Executor {
	var out []chain.Executor
	c.Range(func(key, value any) bool {
		out = append(out, value.(*clientEntry).exec)
		c.Delete(key)
		return true
	})
	return out
}

// managedClientExecutor is the stable, lazily-delegating executor [Manager.ClientExecutorFor]
// returns. It holds the manager and the label; the backing executor is resolved on
// each Execute through the manager's registry. A policy change needs nothing here:
// Apply drops the registration, and the next Execute builds the executor the new
// policy asks for.
type managedClientExecutor struct {
	mgr    *Manager
	system string
	label  string
}

func (e *managedClientExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return e.mgr.client.backing(e.mgr, e.label, e.system).Execute(ctx, fn)
}

func (e *managedClientExecutor) Close() error { return nil }

// noopClientExecutor runs fn once with no protection — the executor a label resolves to
// on a manager that is unarmed or whose driver could not build one. It is the
// zero-cost fallback that keeps client code uniform whether or not resilience is
// configured.
type noopClientExecutor struct{}

func (noopClientExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (noopClientExecutor) Close() error { return nil }

// subscriber is one caller's interest in the policy for a label. last is the most
// recent policy delivered to cb, so an apply can skip callbacks whose policy is
// unchanged and avoid re-delivering the same value.
type subscriber struct {
	last ClientPolicy
	cb   func(ClientPolicy)

	// cancelled marks a subscriber whose [Subscription.Cancel] has run. The
	// manager drops it from subs on cancel, but an Apply already in flight may
	// hold a reference to it from before the removal — so delivery checks this
	// flag under mu and skips it, making cancel take effect immediately rather
	// than after the next apply.
	cancelled bool
}

// pending is one notification collected under the manager lock and delivered
// after it is released.
type pending struct {
	cb     func(ClientPolicy)
	policy ClientPolicy
}

// Subscription is a live registration created by [Manager.Subscribe]. It carries
// the policy the callback was armed with and the ability to detach it again,
// which matters for callers whose protected objects are not process-lifetime: a
// gateway rebuilding its route table, or any client re-created on a config change.
// Without Cancel, every rebuild would leave a subscriber (and a reference to the
// discarded object) behind in the manager forever.
//
// The zero value is inert: ClientPolicy is a zero pass-through policy and Cancel is a
// no-op, so a Subscription is always safe to hold and always safe to cancel —
// including more than once.
type Subscription struct {
	// ClientPolicy is the policy the callback was armed with at registration time.
	ClientPolicy ClientPolicy

	cancel func()
}

// Cancel detaches the subscription. It is idempotent and safe to call
// concurrently with an apply: once it returns, the callback is no longer invoked.
// Cancelling the zero Subscription is a no-op.
func (s Subscription) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}
