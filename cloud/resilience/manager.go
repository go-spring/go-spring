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
	"sync"
)

// Manager is the centralized resilience authority: it owns the backend it builds
// executors with and, per traffic direction, the registry of executors built for
// it and the subscription fan-out. It is the module-level counterpart of the
// governance center — the center owns the config DOCUMENT (rules, matching,
// source), the Manager owns what to DO with the resulting policies.
//
// The Manager is container-free: it imports no IoC concept and holds no global.
// Its dependency on the governance center is inverted into plain types
// ([Manager.Apply] takes the pushed settings), so this package never imports
// cloud/governance and the center never reaches into this package's internals.
// The gs wiring (providing the Manager as a bean, injecting it into clients) lives
// in starter-governance.
//
// The BACKEND, unlike the policies, is not pushed: it is fixed at construction —
// the [Driver] the container selected from ${spring.resilience.driver}, a
// startup decision, since switching it drops every executor's state. See
// [NewManager].
//
// One label maps to exactly ONE executor per direction. That is not an
// optimization but a semantic requirement: a breaker, limiter and bulkhead are
// per-RESOURCE state, so two objects sharing a label (a rebuilt client, two pools
// for one service name) must share the breaker rather than each getting its own
// and effectively doubling the trip threshold.
//
// Safe for concurrent use.
type Manager struct {
	// mu guards the sides' pushed settings and subscriber tables. The hot path — a
	// protected call — does not take it: an armed manager is read lock-free
	// through the sync.Map inside each side.
	mu sync.Mutex

	// applied reports whether a Settings has been adopted and not since closed.
	// It is what [Manager.Enabled] answers, and it is the build decision in each
	// side's buildLocked: an unarmed manager publishes a literal no-op executor
	// rather than asking the driver for one.
	applied bool

	// driver is the backend every executor this manager hands out is built with,
	// fixed at construction ([NewManager]): both directions use it, since a
	// [Driver] has a method per direction. It is never nil.
	driver Driver

	// client is the outbound side (client_manager.go), server the inbound one
	// (server_manager.go). Each holds its own pushed settings and registry.
	client clientSide
	server serverSide
}

// Settings is the resilience half of the governance document, PUSHED whole: the
// center hands over the two direction blocks and this package does the matching.
// It mirrors [fault.Config] — a default plus a rule list per direction — so the
// family's three modules take the same shape (see the package doc of
// cloud/governance).
//
// It carries policies only, never the backend: which [Driver] builds the
// executors is a startup decision ([NewManager]), not part of the tunable
// document.
//
// The zero Settings configures nothing: every label resolves a zero policy, which
// every caller reads as the pass-through. There is no arming flag — an empty
// document is a pass-through, and so is an absent one.
type Settings struct {
	// Client is the outbound half: what to apply when this process CALLS something.
	Client ClientSettings

	// Server is the inbound half: what to apply against the requests it RECEIVES.
	// It is a separate block with a separate type on purpose — a policy written for
	// one direction cannot be served to the other, and the compiler says so if the
	// two are ever crossed.
	Server ServerSettings
}

// ClientSettings is the pushed outbound half of the document: the policy every
// service runs under, plus per-service overrides. See [Settings.Client].
type ClientSettings struct {
	// Default is the policy applied to every service label no [ClientRule]
	// matches. Zero means the pass-through, which is what a deployment that
	// governs only inbound traffic looks like.
	Default ClientPolicy

	// Rules are the per-service overrides: the first Rule whose Service equals the
	// label wins. An empty Service matches NOTHING (it is not a catch-all — use
	// Default for that). Labels are unique across Rules; the center rejects a
	// duplicate document before it reaches here.
	Rules []ClientRule
}

// ClientRule is one per-service outbound override. See [ClientSettings.Rules].
type ClientRule struct {
	// Service is the service label this Rule matches, exact-compare — the identity
	// a client starter passes when resolving an outbound executor.
	Service string

	// ClientPolicy is what the matched service runs under; it fully REPLACES
	// Default rather than merging field-wise.
	ClientPolicy
}

// ServerSettings is the pushed inbound half: the policy every route runs under,
// plus per-route overrides. It carries no retry — [ServerPolicy] cannot express
// one. See [Settings.Server].
type ServerSettings struct {
	// Default is the policy applied to every route label no [ServerRule] matches.
	Default ServerPolicy

	// Rules are the per-route overrides, matched by the same exact label the
	// inbound middleware passes. An empty Service matches NOTHING. Labels are
	// unique across Rules.
	Rules []ServerRule
}

// ServerRule is one per-route inbound override. See [ServerSettings.Rules].
type ServerRule struct {
	// Service is the route label this Rule matches, exact-compare — the identity
	// the inbound middleware passes (a listening address, or a caller-supplied
	// label).
	Service string

	// ServerPolicy is what the matched route runs under; it fully REPLACES Default.
	ServerPolicy
}

// NewManager returns an unarmed Manager that builds every executor with driver.
// driver is the backend chosen at startup — the starter injects the [Driver] bean
// the container selected from ${spring.resilience.driver} — and it is held for
// the manager's lifetime: a backend swap resets every breaker, limiter and
// bulkhead, so it is a deploy-time decision, not a pushed one.
//
// A nil driver falls back to the bundled [NewDefaultDriver], which is what a
// standalone caller (and a test) passes, and what a process that contributes no
// backend of its own runs on.
//
// The manager is unarmed: it holds no registrations and no settings, so every
// [Manager.ClientExecutorFor] and [Manager.ServerExecutorFor] yields a
// transparent pass-through until [Manager.Apply] arms it.
func NewManager(driver Driver) *Manager {
	if driver == nil {
		driver = NewDefaultDriver(nil)
	}
	m := &Manager{driver: driver}
	m.client = clientSide{subs: map[string][]*subscriber{}}
	return m
}

// Apply adopts s as the manager's settings, drops every registration the new
// settings invalidate in both sides, and notifies the subscribers whose policy
// changed. It is the single entry point the governance center calls — once at
// startup with the source's snapshot, then on every push.
//
// A registration is invalid when its label no longer resolves to the policy it was
// built under. Everything else is left alone: a subscriber whose policy is
// unchanged is not notified, and a label whose policy is unchanged keeps its
// executor — and its breaker state — so a change localized to another label does
// not churn unrelated services.
//
// Dropping a registration is the whole of "apply a new policy": the next call on
// that label builds a fresh executor from the driver, so every stage starts clean,
// and there is no second path by which a policy can change. The executors dropped
// here are not closed — [Manager.Close] is what closes.
//
// Apply always succeeds: the document's only rejectable field was the backend,
// which now lives at startup, so there is nothing left to validate here. The
// error return is kept for the caller's convenience (the center treats Apply as
// infallible) and is always nil.
func (m *Manager) Apply(s Settings) error {
	m.mu.Lock()
	m.client.settings = s.Client
	m.server.settings = s.Server
	m.applied = true
	m.client.evictChangedLocked()
	m.server.evictChangedLocked()
	todo := m.client.collectLocked()
	m.mu.Unlock()

	for _, p := range todo {
		p.cb(p.policy)
	}
	return nil
}

// Enabled reports whether the governance center that drives this manager has
// applied a Settings — i.e. whether the manager has been armed at all. Callers
// that guard an optional governance path with "is governance driving me?" read
// this instead of resolving a policy and testing it for zero.
//
// It is deliberately about ADOPTION, not about content: a center that went live
// over a document configuring nothing still reports true, so a caller (dubbo)
// subscribes and keeps following later pushes. A manager that was never armed,
// and one whose [Manager.Close] has run, report false — which is exactly the
// pass-through case.
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.applied
}

// Close releases every registered executor in both sides. The Manager itself holds
// only in-memory state, so this is the shutdown counterpart to [Manager.Apply]: it
// lets a production driver stop background resources (metric pumps) instead of
// leaking them with the process.
//
// It is the manager BEAN's destroy method (see the starter), so the container
// calls it on shutdown — after the clients that hold those executors, since gs
// tears beans down in reverse construction order.
func (m *Manager) Close() error {
	m.mu.Lock()
	m.applied = false
	m.client.settings = ClientSettings{}
	m.server.settings = ServerSettings{}
	m.client.subs = map[string][]*subscriber{}
	clientExecs := drainClientCache(&m.client.entries)
	serverExecs := drainServerCache(&m.server.entries)
	m.mu.Unlock()

	for _, e := range clientExecs {
		_ = e.Close()
	}
	for _, e := range serverExecs {
		_ = e.Close()
	}
	return nil
}
