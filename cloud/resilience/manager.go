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
	"maps"
	"sort"
	"sync"

	"go-spring.org/stdlib/errutil"
)

// Manager is the centralized resilience authority: it owns the driver directory
// and, per traffic direction, the registry of executors built for it and the
// subscription fan-out. It is the module-level counterpart of the governance
// center — the center owns the config DOCUMENT (rules, matching, source), the
// Manager owns what to DO with the resulting policies.
//
// The Manager is container-free: it imports no IoC concept and holds no global.
// Its dependency on the governance center is inverted into plain function types
// ([Manager.Apply] takes the two resolvers), so this package never imports
// cloud/governance and the center never reaches into this package's internals.
// The gs wiring (providing the Manager as a bean, injecting it into clients) lives
// in starter-governance.
//
// One label maps to exactly ONE executor per direction. That is not an
// optimization but a semantic requirement: a breaker, limiter and bulkhead are
// per-RESOURCE state, so two objects sharing a label (a rebuilt client, two pools
// for one service name) must share the breaker rather than each getting its own
// and effectively doubling the trip threshold.
//
// Safe for concurrent use.
type Manager struct {
	// mu guards the sides' resolvers and subscriber tables. The hot path — a
	// protected call — does not take it: an armed manager is read lock-free
	// through the sync.Map inside each side.
	mu sync.Mutex

	// drivers is the driver directory the container provided, keyed by bean name.
	// It is fixed at construction — see [NewManager] — so it is read without mu.
	// A nil map leaves the bundled driver as the only backend.
	drivers map[string]Driver

	// driverName names the backend ALL services run under — both directions, since
	// the outbound and inbound executors are built by the same driver object
	// ([Driver] has a method per direction).
	driverName string

	// client is the outbound side (client_manager.go), server the inbound one
	// (server_manager.go). Each holds its own resolver and registry.
	client clientSide
	server serverSide
}

// Settings is the resilience half of the governance document: everything this
// package needs to serve its clients. The center builds it from its own [Config]
// and hands it over, which is what keeps the two layers decoupled — the center
// knows the document, the Manager knows the policies.
type Settings struct {
	// Enabled false means the center is switched off: every executor and inbound
	// executor is a transparent pass-through regardless of the resolvers.
	Enabled bool

	// Driver names the backend all services run under, in both directions. Empty
	// means the bundled [DefaultDriverName]. It must name a backend in the
	// directory the manager was built over ([NewManager]), or [Manager.Apply]
	// fails.
	Driver string

	// ResolveClientPolicy returns the outbound policy a service label runs under. It is
	// called lazily and repeatedly — labels are not known up front (a dubbo
	// reference label is derived from runtime config), so the Manager cannot
	// precompute them. A nil ResolveClientPolicy leaves outbound calls on the
	// pass-through.
	ResolveClientPolicy func(label string) ClientPolicy

	// ResolveServerPolicy returns the inbound inbound policy a route label runs
	// under, for the same reasons and with the same laziness as
	// [Settings.ResolveClientPolicy]. It resolves over the document's inbound half,
	// whose rules carry no retry. A nil ResolveServerPolicy leaves inbound requests
	// on the pass-through, which is what a deployment that governs only outbound
	// traffic looks like.
	//
	// The two resolvers answer with DIFFERENT types on purpose: a policy written
	// for one direction cannot be served to the other, and the compiler says so if
	// the two fields are ever crossed.
	ResolveServerPolicy func(label string) ServerPolicy
}

// NewManager returns an unarmed Manager over the driver directory drivers —
// the [Driver] beans the container contributed, keyed by bean name. It is taken
// here rather than through a setter because the directory is fixed for the
// manager's lifetime: the container hands over everything it has at
// construction, and [Manager.Apply] only validates a configured name against
// it. The map is copied, so the manager owns its directory.
//
// A nil drivers leaves the manager on the bundled driver alone, which is what a
// process that contributes none looks like.
//
// The manager is unarmed: it holds no registrations and no resolvers, so every
// [Manager.ClientExecutorFor] and [Manager.ServerExecutorFor] yields a
// transparent pass-through until [Manager.Apply] arms it.
func NewManager(drivers map[string]Driver) *Manager {
	m := &Manager{}
	m.client = clientSide{subs: map[string][]*subscriber{}}
	if drivers != nil {
		m.drivers = make(map[string]Driver, len(drivers))
		maps.Copy(m.drivers, drivers)
	}
	return m
}

// Apply adopts s as the manager's settings, drops every registration the new
// settings invalidate in both sides, and notifies the subscribers whose policy
// changed. It is the single entry point the governance center calls — once at
// startup with the source's snapshot, then on every push.
//
// A registration is invalid when its backend changed (every executor was built by
// the previous driver, whatever its policy says) or when its label no longer
// resolves to the policy it was built under. Everything else is left alone: a
// subscriber whose policy is unchanged is not notified, and a label whose policy
// is unchanged keeps its executor — and its breaker state — so a change localized
// to another label does not churn unrelated services.
//
// Dropping a registration is the whole of "apply a new policy": the next call on
// that label builds a fresh executor from the driver, so every stage starts clean,
// and there is no second path by which a policy can change. The executors dropped
// here are not closed — [Manager.Close] is what closes.
//
// An enabled Settings whose Driver matches no installed backend is a wiring error,
// not a runtime one: failing here — where the caller can report it at startup —
// beats silently serving a pass-through. A disabled Settings is never checked,
// since it builds no executor.
func (m *Manager) Apply(s Settings) error {
	m.mu.Lock()
	driver := s.Driver
	if driver == "" {
		driver = DefaultDriverName
	}
	if s.Enabled && (s.ResolveClientPolicy != nil || s.ResolveServerPolicy != nil) {
		if _, err := resolveBackend(m.drivers, driver, DefaultDriverName, "driver", NewDefaultDriver(nil)); err != nil {
			m.mu.Unlock()
			return err
		}
	}
	if m.driverName != driver {
		clearCache(&m.client.entries)
		clearCache(&m.server.entries)
		m.driverName = driver
	}
	m.client.resolve = s.ResolveClientPolicy
	m.server.resolve = s.ResolveServerPolicy
	if !s.Enabled {
		m.client.resolve, m.server.resolve = nil, nil
	}
	m.client.evictChangedLocked()
	m.server.evictChangedLocked()
	todo := m.client.collectLocked()
	m.mu.Unlock()

	for _, p := range todo {
		p.cb(p.policy)
	}
	return nil
}

// clearCache empties a side's registry.
func clearCache(c *sync.Map) {
	c.Range(func(key, _ any) bool {
		c.Delete(key)
		return true
	})
}

// Driver returns the backend name currently in force, defaulting to
// [DefaultDriverName] when unset or unarmed.
func (m *Manager) Driver() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.driverName == "" {
		return DefaultDriverName
	}
	return m.driverName
}

// Enabled reports whether the manager is armed with an outbound resolver — i.e.
// whether the governance center that drives it is switched on. Callers that guard
// an optional governance path with "is governance on?" read this instead of
// resolving a policy and testing it for zero: an unarmed manager and a disabled
// center both report false, which is exactly the pass-through case.
func (m *Manager) Enabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.resolve != nil
}

// Close releases every registered executor in both sides. The Manager itself holds
// only in-memory state, so this is the shutdown counterpart to [Manager.Apply]: it
// lets a production driver stop background resources (metric pumps) instead of
// leaking them with the process.
func (m *Manager) Close() error {
	m.mu.Lock()
	m.client.resolve, m.server.resolve = nil, nil
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

// driverFor resolves name against the installed driver directory, falling back to
// the bundled driver for the empty name and [DefaultDriverName], so an unwired or
// default-configured manager always has a backend. The directory never changes
// after construction, so no lock is needed.
func (m *Manager) driverFor(name string) (Driver, error) {
	return resolveBackend(m.drivers, name, DefaultDriverName, "driver", NewDefaultDriver(nil))
}

// resolveBackend picks the backend named name out of dir, the name-keyed
// directory of backends the container provided. The empty name and defaultName
// both resolve to fallback, so a process that contributes no backend still runs
// on the bundled one. A name matching nothing is an error listing what IS
// available, so a typo in a configured backend name is diagnosable instead of
// silently ignored. kind labels that error; dir may be nil.
func resolveBackend[T any](dir map[string]T, name, defaultName, kind string, fallback T) (T, error) {
	if d, ok := dir[name]; ok {
		return d, nil
	}
	if name == "" || name == defaultName {
		return fallback, nil
	}
	available := []string{defaultName}
	for k := range dir {
		if k != defaultName {
			available = append(available, k)
		}
	}
	sort.Strings(available)
	var zero T
	// defaultName is already in the list, so the name that missed is the only
	// one absent from it.
	return zero, errutil.Explain(nil, "resilience: no %s named %q (available: %v)", kind, name, available)
}
