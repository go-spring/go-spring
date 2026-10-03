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

package loadbalance

import (
	"context"
	"maps"
	"sync"

	"go-spring.org/log"
)

// Manager is the centralized endpoint-selection authority: it resolves a
// resource label to a [Selection] and keeps every bound pool in step with it.
//
// It is the selection counterpart of resilience.Manager, and follows the same
// split: the governance center owns the config DOCUMENT (rules, matching,
// source) and hands this Manager a way to resolve a label; the Manager owns what
// to DO with the result — here, pushing it into the pools and subscribers that
// asked to follow that label.
//
// The Manager is container-free and holds no global: the gs wiring (providing it
// as a bean, injecting it into the clients that own pools) lives in
// starter-governance.
//
// Safe for concurrent use.
type Manager struct {
	// mu guards subs and resolve. It is never taken on the request path — a
	// Pick reads the pool's own selection, not this table.
	mu sync.Mutex

	// subs is the per-label subscriber table. A label with no subscriber leaves
	// no entry.
	subs map[string][]*subscriber

	// resolve maps a resource label to its selection. It is nil on an unarmed
	// manager, in which case every pool keeps the strategy it was built with.
	resolve func(label string) Selection

	// dir is the strategy directory every bound pool's balancer is built from:
	// the built-in strategies plus whatever [Factory] beans the container
	// contributed (see [NewManager]). It is never nil on a manager built by
	// [NewManager].
	dir Directory
}

// Settings is the selection half of the governance document: everything this
// package needs to serve the clients that own pools.
type Settings struct {
	// Enabled false means the center is switched off: every bound pool is put
	// back under the zero Selection (keep the strategy, disable suspension) and
	// later changes notify nobody.
	Enabled bool

	// Resolve returns the selection a resource label runs under. It is called
	// lazily and repeatedly — labels are not known up front — so the Manager
	// cannot precompute them.
	Resolve func(label string) Selection
}

// NewManager returns an unarmed Manager over the built-in strategies extended
// by factories — the additional [Factory] beans the container contributed,
// keyed by bean name. That map IS the strategy directory a `balancer` name in a
// rule is resolved against, and it is taken here rather than through a setter
// because the directory is fixed for the manager's lifetime: the container
// hands over everything it has at construction and nothing installs a strategy
// afterwards.
//
// A nil or empty factories leaves the manager on the built-in strategies alone,
// which is what a process that contributes none looks like. An empty name, a
// nil factory, or a name that shadows a built-in strategy is an error — a
// deployment must not silently replace round_robin — and no manager is
// returned, so a rejected contribution cannot strip the process of the
// strategies it already had.
//
// The manager is unarmed: no resolver yet, so every pool keeps the strategy it
// was built with until [Manager.Apply] arms it. Bindings taken while it is
// unarmed are remembered and come alive on that Apply.
func NewManager(factories map[string]Factory) (*Manager, error) {
	dir, err := NewDirectory(factories)
	if err != nil {
		return nil, err
	}
	return &Manager{subs: map[string][]*subscriber{}, dir: dir}, nil
}

// Build constructs the strategy name from the manager's directory, using params
// as the strategy's own parameters. It is how an adapter that cannot hold a
// [Pool] — a gRPC balancer, say — resolves a governed strategy name against the
// same directory the pools use, so a contributed strategy works in both.
func (m *Manager) Build(name string, params map[string]string) (Balancer, error) {
	return m.directory().Build(name, NewParams(params))
}

// directory returns the strategy directory in force. It is never nil: a manager
// built by [NewManager] starts with the built-ins, and one built as a bare
// &Manager{} falls back to them.
func (m *Manager) directory() Directory {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.dir == nil {
		m.dir = builtinDirectory()
	}
	return m.dir
}

// Apply adopts s as the manager's settings and re-evaluates every subscribed
// label, notifying the subscribers whose selection changed. It is the single
// entry point the governance center calls — once at startup with the source's
// snapshot, then on every push.
//
// A subscriber whose selection is unchanged is not notified, so a change
// localized to another label does not churn unrelated pools.
func (m *Manager) Apply(s Settings) {
	var todo []pending

	m.mu.Lock()
	if !s.Enabled {
		m.resolve = nil
	} else {
		m.resolve = s.Resolve
	}
	for label, list := range m.subs {
		next := m.selectionForLocked(label)
		for _, sub := range list {
			// A subscriber cancelled since this apply began must not be
			// notified: its owner is already tearing down what it drives.
			if sub.cancelled {
				continue
			}
			if !selectionEqual(sub.last, next) {
				sub.last = next
				todo = append(todo, pending{label: label, apply: sub.apply, s: next})
			}
		}
	}
	m.mu.Unlock()

	for _, p := range todo {
		m.deliver(p)
	}
}

// Bind wires pool's endpoint selection to label's managed policy: the current
// selection is applied immediately and again on every change, in place — no pool
// rebuild, and the next [Pool.Pick] already sees it.
//
// Bind is the one place a strategy NAME becomes a strategy: it resolves the
// name against the manager's [Directory] and hands the pool a built [Balancer],
// so a pool never holds the factory table. A name that fails to build —
// unknown, or a params bag the strategy rejects — is IGNORED rather than fatal:
// the pool degrades to the last good strategy instead of taking the client
// down, and the error is returned for [Manager.Subscribe] to log (the
// governance source contract has no error channel). The suspension half is
// applied unconditionally, so a rule that only retunes thresholds still works.
//
// The returned func detaches the binding. A pool whose lifetime is not the whole
// process MUST call it, or the Manager keeps a callback pointing at a dead pool.
// Binding an unarmed manager is safe and is the normal case for a pool built
// during bean construction: the binding is remembered and armed by the first
// [Manager.Apply]. A nil pool is ignored.
func (m *Manager) Bind(pool *Pool, label string) (stop func()) {
	if pool == nil {
		return func() {}
	}
	return m.Subscribe(label, func(s Selection) error {
		pool.ApplySuspension(TrackerConfig{Threshold: s.OutlierThreshold, SuspendFor: s.OutlierSuspendFor})
		if s.Balancer == "" {
			return nil // keep the pool's current strategy
		}
		bal, err := m.directory().Build(s.Balancer, NewParams(s.Params))
		if err != nil {
			return err
		}
		pool.ApplyBalancer(bal, s)
		return nil
	})
}

// Subscribe wires apply to label's managed selection: it is invoked immediately
// with the current selection, then again on every change. It is the escape hatch
// for a caller that resolves its own sink rather than owning a [Pool] — a balancer
// re-deriving its strategy, a client that maps the selection onto its own knobs.
//
// apply must be safe for concurrent invocation. A non-nil error it returns — a
// strategy the sink rejected — is logged here, tagged with the label, because the
// push channel has no error return.
//
// Subscribing to an UNARMED manager is meaningful, not a no-op: the sink is armed
// with the zero Selection (which applies as "keep the strategy, disable
// suspension") and is notified as soon as the manager is armed. That is what makes
// binding safe during bean construction, which can run BEFORE the center bean's
// [governance.Center.GoLive] — the moment a pool is built, the manager has no
// resolver yet, and dropping the subscription here would silently lose the
// binding for the process lifetime.
//
// The returned func detaches the subscription and is idempotent.
func (m *Manager) Subscribe(label string, apply func(Selection) error) (stop func()) {
	m.mu.Lock()
	cur := m.selectionForLocked(label)
	sub := &subscriber{last: cur, apply: apply}
	m.subs[label] = append(m.subs[label], sub)
	m.mu.Unlock()

	m.deliver(pending{label: label, apply: apply, s: cur})
	return func() { m.unsubscribe(label, sub) }
}

// SelectionFor returns the selection label currently resolves to, or the zero
// Selection on an unarmed manager.
func (m *Manager) SelectionFor(label string) Selection {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.selectionForLocked(label)
}

// Enabled reports whether the manager is armed.
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resolve != nil
}

// selectionForLocked resolves label under the manager lock. Callers must hold mu.
func (m *Manager) selectionForLocked(label string) Selection {
	if m.resolve == nil {
		return Selection{}
	}
	return m.resolve(label)
}

// deliver runs one notification, logging a rejection with its label so the
// operator can see which resource refused which strategy.
func (m *Manager) deliver(p pending) {
	if err := p.apply(p.s); err != nil {
		log.Warn(context.Background(), log.TagAppDef,
			log.String("label", p.label),
			log.Err(err),
			log.Msg("loadbalance: endpoint selection rejected, keeping the resource's current strategy"))
	}
}

// unsubscribe detaches sub from label. It marks sub cancelled before removing it,
// so an apply that already collected it still skips it at delivery time. Removing
// the last subscriber for a label drops the map entry, so a repeatedly-rebuilt
// client leaves no residue — not even an empty slice.
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

// subscriber is one pool's (or client's) interest in the selection for a label.
// last is the most recent selection delivered, so an apply can skip sinks whose
// selection is unchanged.
type subscriber struct {
	last  Selection
	apply func(Selection) error

	// cancelled marks a subscriber whose stop func has run. The manager drops it
	// from subs on cancel, but an Apply already in flight may hold a reference to
	// it from before the removal — so delivery checks this flag under mu and skips
	// it, making cancel take effect immediately rather than after the next apply.
	cancelled bool
}

// pending is one notification collected under the manager lock and delivered
// after it is released.
type pending struct {
	label string
	apply func(Selection) error
	s     Selection
}

// selectionEqual reports whether two resolved selections are equivalent for the
// purpose of change detection. [Selection] carries a parameter map and so is not
// comparable with ==; every field is compared explicitly instead.
func selectionEqual(a, b Selection) bool {
	return a.Balancer == b.Balancer &&
		maps.Equal(a.Params, b.Params) &&
		a.OutlierThreshold == b.OutlierThreshold &&
		a.OutlierSuspendFor == b.OutlierSuspendFor
}
