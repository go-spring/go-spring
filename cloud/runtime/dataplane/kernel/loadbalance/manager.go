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
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
)

// Manager is the centralized endpoint-selection authority: it maps a resource
// label to a [Selection] and keeps every sink that follows that label in step
// with it.
//
// It is the selection counterpart of resilience.Manager, and follows the same
// split: the governance center owns the config DOCUMENT (parsing, the source,
// duplicate-label rejection) and PUSHES this Manager the two data halves it
// needs — a [Settings.Default] and a rule list. The Manager owns the matching
// (exact label, else Default) and what to DO with the result.
//
// What it does with the result is what the client asked for. The one case is
// [Chain] (built through [Manager.Chain]): the endpoint-selection
// chain a client owns registers itself under its label and the manager REBUILDS
// it from scratch whenever that label's selection moves, so nothing of the
// replaced configuration is still choosing traffic (see [Chain]).
//
// The Manager is container-free and holds no global: the gs wiring (providing it
// as a bean, injecting it into the clients that own chains) lives in
// starter-governance.
//
// Safe for concurrent use.
type Manager struct {
	// mu guards subs and the pushed settings. It is never taken on the
	// request path — a Pick reads the pool's own selection, not these tables.
	mu sync.Mutex

	// subs is the per-label table of registered chains. A label with no chain
	// leaves no entry.
	subs map[string][]*Chain

	// settings is the document last pushed by [Manager.Apply], held whole — the
	// Manager stores what it was handed, it does not take it apart. The zero value
	// is the initial (and pass-through) state: every label resolves the zero
	// Selection, so every pool keeps the strategy it was built with.
	settings Settings

	// snap is the settings as the CHAINS see them: one generation, published
	// whole, read lock-free by every [SelectionSource.ResolveSelection] on the
	// request path. A chain must not take mu to resolve — it resolves outside its
	// own lock, and taking the manager's there would invert the lock order
	// against the manager's build path.
	snap atomic.Pointer[selectionSnapshot]
}

// (The manager pushes nothing: a registered chain IS its own sink, and it reads
// the published generation on each request — see [Chain.sync].)

// Settings is the selection half of the governance document, PUSHED whole: the
// center hands over the data, and this package does the matching. It mirrors
// [fault.Config] — a global default plus a rule list — so the family's three
// modules take the same shape (see the package doc of cloud/runtime/controlplane/governance).
//
// The zero Settings configures nothing: every label resolves the zero Selection,
// which a chain reads as the base configuration (round-robin, suspension off) and
// a subscriber reads as "keep what you have". There
// is no arming flag; an empty document and an absent one look the same, and both
// are a pass-through.
type Settings struct {
	// Default is the selection every label runs under when no Rule matches it.
	Default Selection

	// Rules are the per-label overrides: the first Rule whose Service equals the
	// label wins. An empty Service matches NOTHING (it is not a catch-all — use
	// Default for that), so a rule list cannot accidentally govern every label.
	//
	// Labels are unique across Rules — the center rejects a duplicate document
	// before it reaches here — because a linear scan would otherwise let list
	// order decide which entry applies.
	Rules []Rule
}

// Rule is one per-label selection override. See [Settings.Rules].
type Rule struct {
	// Service is the resource label this Rule matches, exact-compare — the same
	// identity a client starter registers its chain under (see [Manager.Chain]).
	Service string

	// Selection is what the matched label runs under.
	Selection Selection
}

// NewManager returns an unarmed Manager. It resolves strategy names against the
// package registry (see [Register]), which is process-wide, so a manager needs
// no strategy directory of its own.
//
// The manager starts with no settings, so every pool keeps the strategy it was
// built with until the first [Manager.Apply]. Bindings taken before that are
// remembered and come alive on that Apply.
func NewManager() *Manager {
	m := &Manager{subs: map[string][]*Chain{}}
	// Publish the zero document from the start rather than leaving the snapshot
	// absent: an unarmed manager's generation for a label is then the SAME number
	// an armed one with a zero document produces, so a chain does not rebuild
	// merely because a document arrived.
	m.snap.Store(newSelectionSnapshot(Settings{}))
	return m
}

// Apply adopts s as the manager's settings and publishes them as the generation
// the chains will follow. It is the single entry point the governance center
// calls — once at startup with the source's snapshot, then on every push.
//
// Publishing is the whole of it: Apply does not walk the registered chains and
// does not rebuild anything. The manager answers what a label's selection IS and
// which generation that is ([SelectionSource]); each [Chain] notices on its own
// next request that its pool was built from a different generation and rebuilds
// then. A push that does not move a label leaves that label's chain — and its
// in-flight counts, its hash ring and its observed failures — untouched, because
// the generation is derived from CONTENT (see [labelGen]) rather than from the
// fact that Apply ran.
//
// The settings REPLACE the previous ones wholesale rather than merging, so a
// document that drops a rule disarms that label — there is no separate off
// switch to remember.
//
// ctx is not consulted: nothing on this path does I/O or runs caller code any
// more. It is kept for the caller's convenience and for the shape the three
// governance layers share.
func (m *Manager) Apply(ctx context.Context, s Settings) {
	_ = ctx
	snap := newSelectionSnapshot(s)

	m.mu.Lock()
	m.settings = s
	m.snap.Store(snap)
	m.mu.Unlock()
}

// ResolveSelection is [SelectionSource] for this manager: what label resolves to
// right now, and the generation that answer belongs to. It takes no lock — a
// request path reaches it on every Pick — and it never fails: an unarmed (or
// closed) manager resolves the zero selection, which a chain reads as the base
// configuration.
func (m *Manager) ResolveSelection(label string) (Selection, uint64) {
	r := m.snap.Load().resolve(label)
	return r.sel, r.gen
}

// Chain builds and registers a [Chain] under label — the manager-side entry a
// caller uses to obtain a governed chain: registration and the first generation
// happen in ONE step, so a caller cannot build a chain and forget to register it,
// or start from a stale selection.
//
// The first generation is built from the selection in force at this moment, not
// from the zero selection, so a chain created after the document arrived already
// runs its policy instead of waiting for the next push. An unarmed manager has
// no selection, which builds the base chain.
//
// The registration is one-way and permanent: there is no detach. The chain MUST
// outlive the manager that governs it (which is what a bean-managed client is),
// because the manager keeps the pointer — a chain that died first would be
// neither collected nor safe to keep. Everything is released by [Manager.Close],
// at the end of the manager's life.
//
// The returned chain is the caller's outward surface: hold it, [Chain.Pick] and
// [Chain.Complete] through it, and the generations underneath it follow the
// manager's settings — the manager itself never touches it again.
//
// ctx carries the caller's trace into a refused selection's log line; a chain
// built where no such context exists (a transport assembled at wiring time) may
// pass [context.Background].
func (m *Manager) Chain(ctx context.Context, src discovery.Resolver, label string) *Chain {
	m.mu.Lock()
	c := newChain(src, label, m)
	m.subs[label] = append(m.subs[label], c)
	// Arm it now, from the selection in force, so a chain created after the
	// document arrived already runs its policy. Done under the manager lock so a
	// chain cannot be published between reading the settings and building from
	// them.
	c.sync(ctx)
	m.mu.Unlock()
	return c
}

// selectionForLocked resolves label under the manager lock: the first Rule whose
// Service equals the label, else the Default. Callers must hold mu.
//
// The match is plain equality — an empty Service matches nothing, deliberately:
// a rule list must not be able to govern every label by accident (Default is how
// a document says "everything else").
func (m *Manager) selectionForLocked(label string) Selection {
	for _, r := range m.settings.Rules {
		if r.Service == label {
			return r.Selection
		}
	}
	return m.settings.Default
}

// Close drops every subscription and the pushed settings. It is the shutdown
// counterpart to [Manager.Apply], and it is the
// manager BEAN's destroy method (see the starter), so the container calls it on
// shutdown — after the chains it governs, since gs tears beans down in reverse
// construction order.
//
// The manager holds no resources of its own — no goroutines, no connections —
// so this exists for one reason: it is the ONLY way the references it keeps on
// its sinks are released. Registering (see [Manager.Chain]) is one-way, so
// nothing else can drop them, and a manager that is
// going away must stop pinning the sinks it governs.
//
// Closing does not stop a [Chain] from serving: a chain's generation stays
// current and every [Chain.Pick] keeps working. What ends is the rebuild
// path — a chain outliving a closed manager simply never moves again.
func (m *Manager) Close() error {
	m.mu.Lock()
	m.settings = Settings{}
	m.snap.Store(newSelectionSnapshot(Settings{}))
	m.subs = map[string][]*Chain{}
	m.mu.Unlock()
	return nil
}
