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
	"maps"
	"sort"
	"sync"

	"go-spring.org/stdlib/errutil"
)

// model is one direction's configuration type: [ClientPolicy] for outbound calls,
// [ServerPolicy] for inbound requests. Both are all-scalar structs, so equality is
// exact change detection — a lane compares them with == and a future field that
// broke comparability would be a compile error rather than a silently missed
// refresh.
type model interface {
	comparable
	IsZero() bool
}

// lane is ONE direction's half of the manager: the resolver the governance center
// supplies, the per-label executor cache, and the per-label subscription table.
//
// Both directions are the same machinery over different models, which is why this
// is one type used twice rather than two copies. They share NO state, though: a
// label resolved here as an outbound executor and the same string resolved as
// inbound admission are different protection objects running different policies,
// and an apply on one side cannot refresh the other.
//
// E is always an interface type ([ClientExecutor] or [ServerExecutor]) — the manager
// holds one lane per direction, and each lane's methods are private to the
// manager, so the direction is fixed by which lane a public method reaches for.
type lane[M model, E any] struct {
	// resolve maps a label to the model it runs under, or nil when this direction
	// is unarmed (a disabled center, or a center that configures no admission),
	// in which case every label falls to the pass-through. Replaced wholesale by
	// [Manager.Apply].
	resolve func(label string) M

	// newBacking asks a driver for the executor protecting label under m. ok is
	// false when the driver could not build one, in which case the pass-through
	// is published instead. It is the ONE direction-specific call in the lane.
	newBacking func(d Driver, label string, m M) (E, bool)

	// refresh adopts a new model on an already-built executor — the hot-reload
	// counterpart of newBacking, and the reason a lane can keep an executor alive
	// across applies instead of rebuilding it.
	refresh func(e E, m M) error

	// wrap is applied to a freshly built backing executor before it is published:
	// the observe layer, so clients get an already-instrumented executor.
	wrap func(e E, system, label string) E

	// noop is the pass-through published for a label whose direction is unarmed or
	// whose driver could not build an executor. Sharing one instance keeps that
	// case free.
	noop E

	// subs is the per-label subscriber table. A label with no subscriber leaves no
	// entry, so a repeatedly-rebuilt client leaves no residue. Guarded by
	// [Manager.mu].
	subs map[string][]*subscriber[M]

	// cache memoizes the backing executor each label resolves to. It is a
	// sync.Map so the hot path stays lock-free, and it is cleared by
	// [Manager.Apply] whenever the backend or this lane's resolver changes, so no
	// executor outlives the config generation that built it.
	cache sync.Map // label -> E
}

// policy returns this lane's model for label, or the zero model on an unarmed
// lane. Callers in Apply hold the manager lock; the hot path does not come here.
func (l *lane[M, E]) policy(label string) M {
	if l.resolve == nil {
		var zero M
		return zero
	}
	return l.resolve(label)
}

// addSubLocked registers cb for label and arms it with the label's current model.
// Callers must hold the manager lock.
func (l *lane[M, E]) addSubLocked(label string, cb func(M)) *subscriber[M] {
	s := &subscriber[M]{last: l.policy(label), cb: cb}
	l.subs[label] = append(l.subs[label], s)
	return s
}

// collectLocked gathers the subscribers whose model for their label changed,
// updating each one's last-delivered model. Callers must hold the manager lock
// and MUST deliver the returned notifications after unlocking.
func (l *lane[M, E]) collectLocked() []pending[M] {
	var todo []pending[M]
	for label, list := range l.subs {
		next := l.policy(label)
		for _, s := range list {
			// A subscriber cancelled since this apply began must not be notified:
			// its owner is already tearing down what cb drives.
			if s.cancelled {
				continue
			}
			if s.last != next {
				s.last = next
				todo = append(todo, pending[M]{cb: s.cb, m: next})
			}
		}
	}
	return todo
}

// unsubscribe detaches s from label. It marks s cancelled before removing it, so an
// apply that already collected s into its pending list still skips it at delivery
// time. Removing the last subscriber for a label drops the map entry, so a
// repeatedly-rebuilt client leaves no residue — not even an empty slice.
func (l *lane[M, E]) unsubscribe(m *Manager, label string, s *subscriber[M]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s.cancelled = true
	list := l.subs[label]
	for i, e := range list {
		if e == s {
			list = append(list[:i:i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(l.subs, label)
		return
	}
	l.subs[label] = list
}

// backing returns the memoized executor for label, building it on first use. With
// no resolver it publishes the lane's shared pass-through, so an unarmed manager
// costs an atomic load and a sync.Map miss per call.
//
// The observe layer is applied HERE, on the still-private executor, before the
// cache publishes it. That ordering is what lets fault clients attach a breaker
// listener through the construction-time handshake while the breaker does not
// exist yet — and are built afterwards against the captured listener — instead of
// a client trying to reach an executor that is already behind a wrapper chain.
func (l *lane[M, E]) backing(m *Manager, label, system string) E {
	if v, ok := l.cache.Load(label); ok {
		return v.(E)
	}
	built := l.noop
	if exec, ok := l.buildFor(m, label); ok {
		built = exec
	}
	actual, _ := l.cache.LoadOrStore(label, l.wrap(built, system, label))
	return actual.(E)
}

// buildFor constructs the backing executor for label via the current driver and
// model, registering the refresh subscription that keeps it in step with later
// applies. ok is false when the lane is unarmed or the driver cannot produce one,
// in which case the caller publishes the pass-through.
//
// The memoization [lane.backing] performs is what guarantees this runs at most
// once per label — and so that the subscription is armed exactly once — even
// under concurrent first use.
func (l *lane[M, E]) buildFor(m *Manager, label string) (E, bool) {
	m.mu.Lock()
	resolve := l.resolve
	m.mu.Unlock()
	var zero E
	if resolve == nil {
		return zero, false
	}
	d, err := m.driverFor(m.Driver())
	if err != nil || d == nil {
		return zero, false
	}
	mdl := resolve(label)
	exec, ok := l.newBacking(d, label, mdl)
	if !ok {
		return zero, false
	}
	// The subscription lives as long as the executor, which the cache keeps for
	// the process lifetime, so there is nothing to cancel here. Arming it reads
	// the resolver AGAIN, because an apply may have landed while the driver was
	// building: in that case the executor holds the superseded model and must be
	// brought forward before it serves a call — otherwise the label would run one
	// generation behind until the next push.
	m.mu.Lock()
	s := l.addSubLocked(label, func(np M) { _ = l.refresh(exec, np) })
	cur := s.last
	m.mu.Unlock()
	if cur != mdl {
		_ = l.refresh(exec, cur)
	}
	return exec, true
}

// Manager is the centralized resilience authority: it owns the driver directory
// and, per traffic direction, the executor cache and the subscription fan-out. It
// is the module-level counterpart of the governance center — the center owns the
// config DOCUMENT (rules, matching, source), the Manager owns what to DO with the
// resulting policies and serverPolicies.
//
// The Manager is container-free: it imports no IoC concept and holds no global.
// Its dependency on the governance center is inverted into plain function types
// ([Manager.Apply] takes the two resolvers), so this package never imports
// cloud/governance and the center never reaches into this package's internals.
// The gs wiring (providing the Manager as a bean, injecting it into clients) lives
// in starter-governance.
//
// One label maps to exactly ONE backing executor per direction. That is not an
// optimization but a semantic requirement: a breaker, limiter and bulkhead are
// per-RESOURCE state, so two objects sharing a label (a rebuilt client, two pools
// for one service name) must share the breaker rather than each getting its own
// and effectively doubling the trip threshold.
//
// Safe for concurrent use.
type Manager struct {
	// mu guards subs, drivers, driver and the lanes' resolvers. The hot path — a
	// protected call — does not take it: an armed manager is read lock-free through
	// the atomic-shaped caches inside the lanes.
	mu sync.Mutex

	// drivers is the driver directory the container provided, keyed by bean name.
	// [Manager.SetDrivers] replaces it wholesale during wiring. A nil map leaves
	// the bundled driver as the only backend.
	drivers map[string]Driver

	// driver names the backend ALL services run under — both directions, since one
	// lane's executor and the other lane's admission executor are built by the same
	// driver object ([Driver] has a method per direction).
	driver string

	// client is the outbound lane, server the inbound one. Each holds its own
	// resolver, cache and subscriptions.
	client lane[ClientPolicy, ClientExecutor]
	server lane[ServerPolicy, ServerExecutor]
}

// Settings is the resilience half of the governance document: everything this
// package needs to serve its clients. The center builds it from its own [Config]
// and hands it over, which is what keeps the two layers decoupled — the center
// knows the document, the Manager knows the models.
type Settings struct {
	// Enabled false means the center is switched off: every executor and admission
	// executor is a transparent pass-through regardless of the resolvers.
	Enabled bool

	// Driver names the backend all services run under, in both directions. Empty
	// means the bundled [DefaultDriverName]. It must name a backend in the
	// directory installed by [Manager.SetDrivers], or [Manager.Apply] fails.
	Driver string

	// ResolveClientPolicy returns the outbound policy a service label runs under. It is called
	// lazily and repeatedly — labels are not known up front (a dubbo reference
	// label is derived from runtime config), so the Manager cannot precompute
	// them. A nil ResolveClientPolicy leaves outbound calls on the pass-through.
	ResolveClientPolicy func(label string) ClientPolicy

	// ResolveServerPolicy returns the inbound admission model a route label runs
	// under, for the same reasons and with the same laziness as
	// [Settings.ResolveClientPolicy]. A nil ResolveServerPolicy leaves inbound requests on the
	// pass-through, which is what a deployment that governs only outbound traffic
	// looks like.
	ResolveServerPolicy func(label string) ServerPolicy
}

// NewManager returns an unarmed Manager: it holds no drivers, no subscriptions and
// no resolvers, so every [Manager.ClientExecutorFor] and [Manager.ServerExecutorFor] yields a
// transparent pass-through until [Manager.Apply] arms it.
func NewManager() *Manager {
	m := &Manager{}
	m.client = lane[ClientPolicy, ClientExecutor]{
		newBacking: func(d Driver, label string, p ClientPolicy) (ClientExecutor, bool) {
			e, err := d.NewClientExecutor(label, p)
			return e, err == nil && e != nil
		},
		refresh: func(e ClientExecutor, p ClientPolicy) error { return e.Refresh(p) },
		wrap:    WrapClientExecutor,
		noop:    noopClientExecutor{},
		subs:    map[string][]*subscriber[ClientPolicy]{},
	}
	m.server = lane[ServerPolicy, ServerExecutor]{
		newBacking: func(d Driver, label string, a ServerPolicy) (ServerExecutor, bool) {
			e, err := d.NewServerExecutor(label, a)
			return e, err == nil && e != nil
		},
		refresh: func(e ServerExecutor, a ServerPolicy) error { return e.Refresh(a) },
		wrap:    WrapServerExecutor,
		noop:    noopServerExecutor{},
		subs:    map[string][]*subscriber[ServerPolicy]{},
	}
	return m
}

// SetDrivers installs the driver directory the container provided, keyed by bean
// name. A nil map leaves the manager on the bundled driver only. It replaces any
// previous directory wholesale and must run before [Manager.Apply], which
// validates the configured name against it.
func (m *Manager) SetDrivers(dir map[string]Driver) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if dir == nil {
		m.drivers = nil
		return
	}
	cp := make(map[string]Driver, len(dir))
	maps.Copy(cp, dir)
	m.drivers = cp
}

// Apply adopts s as the manager's settings, drops every memoized executor in both
// lanes, and re-evaluates every subscribed label, notifying the subscribers whose
// model changed. It is the single entry point the governance center calls — once
// at startup with the source's snapshot, then on every push.
//
// Adopting a new Settings rebuilds every executor from the (possibly new) driver,
// which is what makes a backend switch take effect live. A subscriber whose model
// is unchanged is not notified, so a change localized to another label does not
// churn unrelated executors.
//
// An enabled Settings whose Driver matches no installed backend is a wiring error,
// not a runtime one: the name is latched per service at first resolve, so failing
// here — where the caller can report it at startup — beats silently serving a
// pass-through. A disabled Settings is never checked, since it builds no executor.
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
	// A different backend, or a lane's first arming: every executor in that lane
	// was built by the previous resolver/driver pair and must not survive. The
	// lanes are cleared separately because a deployment may govern only one
	// direction, and the armed one should not be churned by the other arming later.
	if m.driver != driver || m.client.resolve == nil {
		clearCache(&m.client.cache)
	}
	if m.driver != driver || m.server.resolve == nil {
		clearCache(&m.server.cache)
	}
	m.driver = driver
	m.client.resolve = s.ResolveClientPolicy
	m.server.resolve = s.ResolveServerPolicy
	if !s.Enabled {
		m.client.resolve, m.server.resolve = nil, nil
	}
	todoClient := m.client.collectLocked()
	todoServer := m.server.collectLocked()
	m.mu.Unlock()

	for _, p := range todoClient {
		p.cb(p.m)
	}
	for _, p := range todoServer {
		p.cb(p.m)
	}
	return nil
}

// clearCache empties a lane's executor cache.
func clearCache(c *sync.Map) {
	c.Range(func(key, _ any) bool {
		c.Delete(key)
		return true
	})
}

// ClientExecutorFor returns the executor the given system's service should run under.
// It is the call clients use instead of reaching for a governance center: pass
// the owning system (e.g. "redis", "gorm") and the service label, wrap the
// result with fault as desired, and install it in the command/query path.
//
// The returned executor resolves its backing implementation LAZILY, on each
// Execute, through the manager's per-label cache. That indirection is
// load-bearing, not an optimization: a client bean is constructed before the
// container has adopted the config (gs builds every bean, then runs the Init
// hooks), so an eager build would latch the unarmed pass-through permanently.
// Resolving at call time — after the whole container has wired — makes the client
// setup order irrelevant.
//
// Hot-reload is handled here, not by the caller: building a backing executor
// registers a subscription, so a policy change refreshes that executor in place
// and this handle picks it up on its next call.
func (m *Manager) ClientExecutorFor(system, service string) ClientExecutor {
	return &managedClientExecutor{mgr: m, system: system, label: service}
}

// ServerExecutorFor returns the admission executor the given system's inbound route
// should run under. It is the SERVER-side counterpart of [Manager.ClientExecutorFor]:
// pass the owning system (e.g. "gin", "grpc") and the route label, and install
// the result in the inbound middleware chain.
//
// It mirrors ClientExecutorFor in every respect that matters — lazily bound backing,
// per-label cache, hot reload through a subscription — but it resolves the label
// in the SERVER lane, so it reads the admission model and never the outbound
// policy, and it hands out a [ServerExecutor] refreshed with a [ServerPolicy].
// The two directions never read each other's config, which is what lets an
// operator tune admission without touching outbound call protection.
func (m *Manager) ServerExecutorFor(system, service string) ServerExecutor {
	return &managedServerExecutor{mgr: m, system: system, label: service}
}

// NewClientExecutor builds an executor from an explicit policy, bypassing label
// resolution. It is the escape hatch for a caller that has a policy in hand but
// no service label to resolve one from; service is what the executor will be
// bound to (see [ClientExecutor]). The returned executor is not memoized and not
// refreshed by [Manager.Apply] — the caller owns it, so a caller that needs one
// executor per service builds one per service.
func (m *Manager) NewClientExecutor(service string, p ClientPolicy) (ClientExecutor, error) {
	m.mu.Lock()
	driver := m.driver
	m.mu.Unlock()
	d, err := m.driverFor(driver)
	if err != nil {
		return nil, err
	}
	return d.NewClientExecutor(service, p)
}

// ClientPolicyFor returns the outbound policy label currently resolves to, or a zero
// policy on an unarmed manager. Callers that only need to READ the policy — an
// adapter mapping it onto its own knobs — use this instead of building an executor
// they will not use.
func (m *Manager) ClientPolicyFor(label string) ClientPolicy {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.client.policy(label)
}

// Subscribe registers cb for OUTBOUND policy changes of label and arms it
// immediately with the current resolved policy. cb is then invoked whenever
// [Manager.Apply] produces a different policy for that label. Callers that own
// their protected object rather than an [ClientExecutor] — a gateway rebuilding a route
// table, a balancer re-deriving its strategy — use this to follow the config
// without an executor.
//
// cb is always called outside the manager's lock, so a callback that itself calls
// into the Manager cannot self-deadlock; it MUST be safe for concurrent
// invocation, since an Apply may fire concurrently with the immediate call.
// [ClientExecutor.Refresh] satisfies that.
//
// The returned [Subscription] carries the policy cb was armed with and detaches
// the registration on [Subscription.Cancel]. A caller whose object is not
// process-lifetime MUST cancel, or the Manager keeps a reference to the discarded
// object forever.
//
// Inbound admission has no counterpart: an admission executor is refreshed by the
// subscription its own build arms, and nothing outside this package observes
// admission changes.
func (m *Manager) Subscribe(label string, cb func(ClientPolicy)) Subscription {
	m.mu.Lock()
	s := m.client.addSubLocked(label, cb)
	m.mu.Unlock()
	cb(s.last)
	return Subscription{ClientPolicy: s.last, cancel: func() { m.client.unsubscribe(m, label, s) }}
}

// Driver returns the backend name currently in force, defaulting to
// [DefaultDriverName] when unset or unarmed.
func (m *Manager) Driver() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.driver == "" {
		return DefaultDriverName
	}
	return m.driver
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

// Close releases every memoized executor in both lanes. The Manager itself holds
// only in-memory state, so this is the shutdown counterpart to [Manager.Apply]: it
// lets a production driver stop background resources (metric pumps) instead of
// leaking them with the process.
func (m *Manager) Close() error {
	m.mu.Lock()
	m.client.resolve, m.server.resolve = nil, nil
	clearCache(&m.client.cache)
	clearCache(&m.server.cache)
	m.client.subs = map[string][]*subscriber[ClientPolicy]{}
	m.server.subs = map[string][]*subscriber[ServerPolicy]{}
	m.mu.Unlock()
	return nil
}

// driverFor resolves name against the installed driver directory, falling back to
// the bundled driver for the empty name and [DefaultDriverName], so an unwired or
// default-configured manager always has a backend. It must NOT be called with mu
// held — [Manager.Apply] resolves against m.drivers directly instead.
func (m *Manager) driverFor(name string) (Driver, error) {
	m.mu.Lock()
	dir := m.drivers
	m.mu.Unlock()
	return resolveBackend(dir, name, DefaultDriverName, "driver", NewDefaultDriver(nil))
}

// resolveBackend picks the backend named name out of dir, the name-keyed
// directory of backends the container provided (see the discovery package for the
// same shape). The empty name and defaultName both resolve to fallback, so a
// process that contributes no backend still runs on the bundled one. A name
// matching nothing is an error listing what IS available, so a typo in a
// configured backend name is diagnosable instead of silently ignored. kind
// labels that error; dir may be nil.
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

// managedClientExecutor is the stable, lazily-delegating executor [Manager.ClientExecutorFor]
// returns. It holds the manager and the label; the backing executor is resolved on
// each Execute through the manager's cache. Refresh is a no-op here because
// hot-reload is driven on the backing executor by its own subscription.
type managedClientExecutor struct {
	mgr    *Manager
	system string
	label  string
}

func (e *managedClientExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return e.mgr.client.backing(e.mgr, e.label, e.system).Execute(ctx, fn)
}

func (e *managedClientExecutor) Refresh(ClientPolicy) error { return nil }

func (e *managedClientExecutor) Close() error { return nil }

// managedServerExecutor is the inbound counterpart of [managedClientExecutor]:
// [Manager.ServerExecutorFor] returns one, and each Execute resolves the backing
// admission executor through the server lane's cache.
type managedServerExecutor struct {
	mgr    *Manager
	system string
	label  string
}

func (e *managedServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return e.mgr.server.backing(e.mgr, e.label, e.system).Execute(ctx, fn)
}

func (e *managedServerExecutor) Refresh(ServerPolicy) error { return nil }

func (e *managedServerExecutor) Close() error { return nil }

// noopClientExecutor runs fn once with no protection — the executor a label resolves to
// on a manager that is unarmed or whose driver could not build one. It is the
// zero-cost fallback that keeps client code uniform whether or not resilience is
// configured.
type noopClientExecutor struct{}

func (noopClientExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (noopClientExecutor) Refresh(ClientPolicy) error { return nil }

func (noopClientExecutor) Close() error { return nil }

// noopServerExecutor is the inbound counterpart of [noopClientExecutor]: the
// pass-through a route resolves to when admission is unarmed, so the middleware
// chain is installed unconditionally.
type noopServerExecutor struct{}

func (noopServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

func (noopServerExecutor) Refresh(ServerPolicy) error { return nil }

func (noopServerExecutor) Close() error { return nil }

// subscriber is one caller's interest in the model for a label. last is the most
// recent model delivered to cb, so an apply can skip callbacks whose model is
// unchanged and avoid re-delivering the same value.
type subscriber[M model] struct {
	last M
	cb   func(M)

	// cancelled marks a subscriber whose [Subscription.Cancel] has run. The
	// manager drops it from subs on cancel, but an Apply already in flight may
	// hold a reference to it from before the removal — so delivery checks this
	// flag under mu and skips it, making cancel take effect immediately rather
	// than after the next apply.
	cancelled bool
}

// pending is one notification collected under the manager lock and delivered
// after it is released.
type pending[M model] struct {
	cb func(M)
	m  M
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
