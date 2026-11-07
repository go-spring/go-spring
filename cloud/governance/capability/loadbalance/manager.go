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
	"sync/atomic"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Manager is the centralized endpoint-selection authority: it maps a resource
// service to a [Selection] and rebuilds the [Chain] registered under a service
// whose selection moved. A service maps to one chain; a second registration is
// an error (see [Manager.Chain]).
//
// It is the selection counterpart of resilience.Manager and follows the fault
// kernel's shape ([fault.Manager]): the governance center owns the config
// DOCUMENT (parsing and the source) and PUSHES this Manager its two data halves
// — the default [Selection] and a rule list. The Manager owns the matching
// (exact service, else the default) and the rebuild: a push that moves a service
// builds that service's chain a new generation right there, on the push, so the
// request path is one atomic load and nothing else.
//
// The push walks a REGISTRATION table, not a live directory: chains are built
// at wiring time (a bean's constructor, a gateway's compile) and the source
// loop starts only in the Runner phase, so registration is COMPLETE before the
// first push and the two never race — the same phase contract that leaves
// [fault.Manager.Refresh] unsynchronized. A chain created at runtime, after the
// loop runs, is outside the contract and follows nothing.
//
// The Manager is container-free and holds no global: the gs wiring (providing it
// as a bean, injecting it into the clients that own chains) lives in
// starter-governance.
type Manager struct {
	// factory builds the strategy a selection names. It is collected at wiring and
	// never changes, so the manager needs no strategy directory of its own.
	factory *Factory

	// snap is the settings as published: one generation, replaced whole by
	// [Manager.Apply]. It is read lock-free by [Manager.Chain]'s arming.
	snap atomic.Pointer[Settings]

	// chains is the per-service table of registered chains, the walk a push
	// rebuilds. A service maps to exactly one chain: a second registration is a
	// wiring error, not a second entry (see [Manager.Chain]). It is mutated at
	// wiring and read on the loop ([Manager.Apply]) — phases that never overlap,
	// so it is a plain map. [Manager.Close] drops it.
	chains map[string]*Chain
}

// Settings is the selection half of the governance document, PUSHED whole: the
// center hands over the data, and this package does the matching. It mirrors
// [fault.Config] — a global default plus a rule list — so the family's three
// modules take the same shape (see the package doc of
// cloud/governance/controlplane/loadbalance).
//
// A selection need not name a strategy: resolution falls through rule, then
// document default, then the global base strategy (round-robin). A governance
// document is therefore allowed to configure only other capabilities and leave
// selection alone. There is no arming flag; an empty document and an absent one
// look the same, and both resolve to the base strategy.
type Settings struct {
	// Selection is the default every service runs under when no Rule matches it.
	// It is embedded so the default block binds at the document root, the same
	// shape a [Rule] uses for its own selection.
	Selection

	// Rules are the per-service overrides: the first Rule whose Service equals the
	// service applies. Resolution is a linear scan, so list order decides when two
	// rules name the same service, and a rule whose Service is empty simply never
	// matches; the default is how a document says "everything else". Neither is
	// checked for: the document is read one way, and a rule that cannot be built
	// is what fails.
	Rules []Rule `value:"${services:=}"`
}

// Rule is one per-service selection override. See [Settings.Rules].
type Rule struct {
	// Service is the identity this Rule matches, exact-compare — the same
	// identity a client starter registers its chain under (see [Manager.Chain]).
	Service string `value:"${service:=}"`

	// Selection is what the matched service runs under.
	Selection
}

// Selection is the resolved endpoint-selection policy: the strategy (by name),
// the strategy's own parameters, and the outlier-suspension thresholds.
//
// It is the COMPLETE description of how a service selects endpoints: a resolved
// selection leaves nothing to the configuration it replaces (see [Chain]). Like
// the resilience capability policies it is ONE definition — the bindable form IS
// the runtime form; there is no separate binding twin.
type Selection struct {
	// Balancer is the strategy name to select endpoints with (e.g.
	// "least_conn"), naming a strategy exported by a [Provider] bean — a
	// built-in strategy, or one a deployment contributed. Resolution fills an
	// empty name with the global base strategy (round-robin) as its last layer;
	// [Factory.Create] itself still refuses an empty name, so only the
	// resolution layer may hand it one.
	Balancer string `value:"${balancer:=}"`

	// ConsistentHashReplicas is the consistent-hash strategy's virtual-node count
	// per endpoint. Only that strategy reads it; another strategy leaves it
	// alone. The key carries the strategy prefix so flattening the selection does
	// not turn it into a generic, ambiguous "replicas". An unset value gets the
	// default from the tag, and a code-built selection may omit it too: the
	// constructor applies the same fallback. The ceiling keeps a pushed document
	// from allocating an unbounded ring; it mirrors the balancer's maxReplicas.
	ConsistentHashReplicas int `value:"${consistent-hash-replicas:=100}" expr:"$ > 0 && $ <= 1000"`

	// ZoneAware turns on locality preference on top of whatever strategy Balancer
	// names: candidates are first narrowed to the caller's zone and the strategy
	// picks among them (see [ZoneAwareBalancer]). It is a filter, not a strategy,
	// so the rule states its balancer once.
	ZoneAware bool `value:"${zone-aware:=false}"`

	// ZoneKey is the endpoint metadata key locality is matched on. It is read
	// only when [Selection.ZoneAware] is on; empty means the shared zone key
	// ([discovery.MetaKeyZone]), applied when the filter is built.
	ZoneKey string `value:"${zone-key:=}"`

	// Params is the extension seam for strategies this package does not know: a
	// contributed balancer reads the keys it owns out of this map and rejects the
	// rest itself. Built-in strategies do not read it and simply leave it alone,
	// so a stray key neither fails the document nor changes their behavior.
	Params map[string]string `value:"${balancer-params:=}"`

	// OutlierThreshold is the consecutive-failure count that suspends an
	// endpoint. 0 disables suspension.
	OutlierThreshold int `value:"${outlier-threshold:=0}" expr:"$ >= 0"`

	// OutlierSuspendFor is the cool-down before a suspended endpoint gets a
	// half-open trial request. Ignored when [Selection.OutlierThreshold] is 0.
	OutlierSuspendFor time.Duration `value:"${outlier-suspend-for:=0}" expr:"int($) >= 0"`

	// Selector narrows the endpoint set this service may choose from, before the
	// strategy picks — one [matcher.StringMatch] per metadata key, the shared
	// matching vocabulary (`selector.version.prefix=v2`, op always spelled out).
	// It is the DYNAMIC half of endpoint narrowing: the static half is the
	// discovery query's selector, fixed by the client's own configuration, and
	// when both are set both must hold. Because it rides the selection document,
	// a pushed rule can move traffic between versions or zones without a
	// redeploy, and a change rebuilds the chain's generation like any other
	// selection change.
	//
	// Empty means no narrowing. A selector that matches nothing leaves the pool
	// with no candidates, which surfaces as [ErrNoEndpoints] — there is
	// deliberately no implicit fallback, because silently falling back would
	// make a narrow rule (a canary, say) widen to every instance without anyone
	// being told.
	Selector map[string]matcher.StringMatch `value:"${selector:=}"`
}

// Equal reports whether s and other carry the same policy. Params and Selector
// are compared by content, so a selection rebuilt from the same document is
// equal even though it is a different value. It runs once per chain per push,
// never on the request path.
func (s Selection) Equal(other Selection) bool {
	return s.Balancer == other.Balancer &&
		s.OutlierThreshold == other.OutlierThreshold &&
		s.OutlierSuspendFor == other.OutlierSuspendFor &&
		s.ConsistentHashReplicas == other.ConsistentHashReplicas &&
		s.ZoneAware == other.ZoneAware &&
		s.ZoneKey == other.ZoneKey &&
		maps.Equal(s.Params, other.Params) &&
		maps.Equal(s.Selector, other.Selector)
}

// NewManager returns an unarmed Manager over the collected strategy factories.
// factories is required.
//
// The manager starts on an empty document; resolution fills the global base
// strategy (round-robin), so every chain builds that generation until the first
// [Manager.Apply]. Chains taken before that are remembered and rebuilt by that
// Apply.
func NewManager(factory *Factory) *Manager {
	m := &Manager{factory: factory, chains: map[string]*Chain{}}
	// Publish an empty document from the start rather than leaving the snapshot
	// absent, so a chain armed before the first push takes a generation rather
	// than none, and the first real document only rebuilds the services it
	// actually moves.
	m.snap.Store(&Settings{})
	return m
}

// Apply adopts s as the manager's settings and rebuilds every chain whose service
// the document moved. It is the single entry point the governance center calls
// — once at startup with the source's snapshot, then on every push — and it
// runs on that source's single loop, which is why it takes no lock.
//
// Creation is the validation: every selection a registered chain actually
// resolves to is built before anything is committed, so a document that cannot
// serve one of this process's chains returns an error and leaves both the
// snapshot and every chain exactly as they were. A push therefore adopts a
// document whole or not at all; there is no per-service degradation to reason
// about. A rule for a service no chain serves is just data — it is built when a
// chain asks for that service, not eagerly here.
//
// The rebuild is decided by CONTENT, not by the fact that Apply ran: a service's
// generation is the selection it resolves to, so a push that does not move a
// service leaves that service's chain — and its in-flight counts, hash ring and
// observed failures — untouched.
//
// The settings REPLACE the previous ones wholesale rather than merging, so a
// document that drops a rule disarms that service — there is no separate off
// switch to remember.
//
// ctx is the source loop's context, carried into the rejection log line;
// nothing on this path does I/O or runs caller code beyond the builds above.
func (m *Manager) Apply(ctx context.Context, settings Settings) error {
	nextPools, err := m.buildNextGenerations(settings)
	if err != nil {
		log.Error(ctx, log.TagAppDef, err,
			"loadbalance: document rejected, keeping the current configuration")
		return err
	}
	// buildNextGenerations touched nothing, so only now does the document take
	// effect — as a whole.
	m.snap.Store(&settings)
	for service, c := range m.chains {
		if p := nextPools[service]; p != nil {
			// The replaced generation carries suspension state and its share of
			// the process-wide gauge; close it so the count returns.
			c.cur.Swap(p).Close()
		}
	}
	return nil
}

// buildNextGenerations is the build half of Apply: for every registered chain it
// compares the selection in force with the selection the document resolves to,
// and builds the next generation for the chains whose selection moved — which is
// also how the new document is validated, since a selection that cannot serve
// its chain fails the build. It touches neither the snapshot nor any chain: a
// failed build leaves the caller exactly where it was, and Apply commits the
// returned generations only once every one of them exists. A service missing
// from the map keeps the generation it already has.
//
// Only the selections a registered chain actually resolves to are built. A rule
// for a service this process does not serve is just data in the document; it is
// read when a chain asks for that service, not eagerly to prove it is good.
func (m *Manager) buildNextGenerations(settings Settings) (map[string]*Pool, error) {
	nextPools := make(map[string]*Pool, len(m.chains))
	for service, c := range m.chains {
		curPool := c.cur.Load()
		sel := findSelection(settings, service)
		if sel.Equal(curPool.Selection()) {
			continue
		}
		// The base source is the same in every generation (the selection's
		// selector is applied per Pick, not baked into it), so rebuilding
		// over the source in force is rebuilding over the chain's target.
		p, err := m.buildPool(curPool.src, sel)
		if err != nil {
			return nil, errutil.Explain(err, "loadbalance: service %q", service)
		}
		nextPools[service] = p
	}
	return nextPools, nil
}

// findSelection is the service-level resolution, in priority order: the first
// rule whose Service equals the service, else the document default. A resolved
// selection must name a strategy: one that does not is replaced WHOLLY by the
// global base ({Balancer: round-robin}), so any other knob on it does not apply
// either. List order therefore decides when a service repeats. It runs where a
// push or an arming needs the answer — on the manager, never on the request
// path.
func findSelection(settings Settings, service string) Selection {
	sel := settings.Selection
	for _, r := range settings.Rules {
		if r.Service == service {
			sel = r.Selection
			break
		}
	}
	if sel.Balancer == "" {
		sel = Selection{Balancer: RoundRobin}
	}
	return sel
}

// buildPool assembles one INNER-LAYER generation from sel over src: the strategy
// the selection names, plus a fresh tracker carrying the selection's suspension
// thresholds, bound together by a new [Pool]. It is a pure function: it reads
// nothing but its arguments and touches no chain state, so a failed build leaves
// the caller exactly where it was.
//
// sel is already resolved (see [findSelection]), so its strategy name is
// present. An earlier generation is gone by design — there is nothing "in force"
// to keep.
func (m *Manager) buildPool(src discovery.Resolver, sel Selection) (*Pool, error) {
	bal, err := m.factory.Create(sel)
	if err != nil {
		return nil, err
	}
	p := NewPool(src, bal, NewTracker(TrackerConfig{
		Threshold:  sel.OutlierThreshold,
		SuspendFor: sel.OutlierSuspendFor,
	}), sel)
	return p, nil
}

// Chain builds a governed [Chain] for service and arms it in one step, so a
// caller cannot obtain a chain that still runs a stale selection.
//
// The first generation is built from the selection in force at this moment, not
// from the zero selection, so a chain created after the document arrived
// already runs its policy. An unarmed manager resolves every service to the
// global base strategy, round-robin. A later push cannot leave an unbuildable
// selection in force — [Manager.Apply] rejects such a document whole — so
// arming normally cannot fail; the error is still reported if it does.
//
// The chain is registered under its service for every later [Manager.Apply] to
// rebuild. Registration happens at wiring, before the source loop starts (see
// the Manager doc), and there is no detach: the manager keeps the pointer until
// [Manager.Close].
//
// A service maps to exactly one chain. A second registration returns an error
// rather than joining a list: two clients on one governance label would fork the
// selection state, and only the first source would ever be consulted — a silent
// misconfiguration that must surface at wiring, not in routing behavior.
func (m *Manager) Chain(ctx context.Context, src discovery.Resolver, service string) (*Chain, error) {
	if _, exists := m.chains[service]; exists {
		// Two clients on one governance label would fork the selection state, and
		// the second source would be silently ignored, so fail at wiring instead.
		return nil, errutil.Explain(nil,
			"loadbalance: service %q already has a chain; one service must map to one client", service)
	}
	sel := findSelection(*m.snap.Load(), service)
	p, err := m.buildPool(src, sel)
	if err != nil {
		return nil, err
	}
	c := &Chain{}
	c.cur.Store(p)
	m.chains[service] = c
	return c, nil
}

// Close drops the registration table. It is the manager BEAN's destroy method
// (see the starter), so the container calls it on shutdown. The manager holds
// no goroutines and no connections — releasing the chain references is the only
// thing shutdown owes it.
//
// Closing does not stop a [Chain] from serving: a chain's generation stays
// current and every [Chain.Pick] keeps working. What ends is the rebuild path.
func (m *Manager) Close() error {
	m.chains = nil
	return nil
}

// Chain is the OUTER layer of client-side load balancing: the egress a request
// path talks to, and the object that follows the governance selection for its
// service. It owns a whole endpoint-selection chain — the [Balancer], its
// [Tracker] and the inner [Pool] that binds them to a discovery source — as an
// immutable generation, REBUILT FROM SCRATCH by [Manager.Apply] whenever the
// selection its service resolves to moves. Callers hold the chain, not the pool:
// [Chain.Pick] (with the [PickResult] it returns) and [Chain.Selection] are the
// outward surface, each delegating to the generation in force.
//
// A changed selection means a new balancer, a new tracker and a new pool,
// sharing nothing with the generation it replaces. After a config change nothing
// from the old configuration is still in force — which is the property a Chain
// exists for, and also its price: in-flight counts, a hash ring and observed
// failures all start over. The alternative would be to retune the pool in place,
// which is exactly what a Chain is built to avoid: a change must not leave any
// of the configuration it replaced still deciding traffic.
//
// # The manager pushes; the chain holds still
//
// A rebuild happens ON [Manager.Apply], on the governance source's single loop —
// never on the request path. A request path's whole contact with refresh is one
// atomic load of the generation in force: no lock, no comparison, no lazy
// first-request build. This is the fault kernel's shape ([fault.Manager.Fire]
// loads its core the same way): the stable shell publishes, the hot path reads.
//
// The service IS the key, and it maps to exactly one chain: a second client on
// the same service must be rejected at wiring (see [Manager.Chain]) rather than
// silently sharing or forking the selection state.
//
// A chain carries no identity of its own beyond the generation it points at.
// The service it follows is its key in the manager's registration table, and the
// source its generations are built over lives inside each [Pool] — the base
// source, unchanged from one generation to the next, because the selection's
// selector is applied in [Pool.Pick] rather than baked into the source. A
// TARGET change (another service, another discovery backend) is therefore a new
// registration, and since a service already has a chain that registration is
// refused: rebinding a live chain is not a supported operation.
type Chain struct {
	// cur is the generation in force: the pool a request pays attention to. It
	// is read on every request and stored only by the manager's loop on a
	// rebuild, so it is an atomic pointer — neither side ever blocks on the
	// other. The pool carries the selection it was built from (see
	// [Pool.Selection]), which is also the change-test record: one pointer, one
	// record, no way for the two to disagree.
	cur atomic.Pointer[Pool]
}

// Pick selects one live, healthy, non-suspended endpoint from the generation in
// force. It is the chain-level egress: a request path talks to the chain and
// never to the pool underneath. See [Pool.Pick] for the filtering and fallback
// semantics.
//
// The result carries the generation that made the choice, so the request settles
// on that same generation — see [PickResult.Complete]. The caller must not
// re-load the chain to settle it: by then a push may have replaced the
// generation, and the outcome would land on a pool that never saw the request.
//
// Pick is a pure read: one atomic load, then the pool's own work. Refresh
// happens entirely on [Manager.Apply], so a Pick never triggers, waits on, or
// observes a half-built generation.
//
// ctx is the request's, and is threaded to the pool's endpoint source.
func (c *Chain) Pick(ctx context.Context, info PickInfo) (PickResult, error) {
	return c.cur.Load().Pick(ctx, info)
}

// Selection returns the selection the generation in force was built from. It is
// an inspection and test helper.
func (c *Chain) Selection() Selection { return c.cur.Load().Selection() }
