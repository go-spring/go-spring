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
	"slices"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Manager is the centralized endpoint-selection authority: it maps a resource
// service to a [Selection] and rebuilds every [Chain] registered under a service
// whose selection moved.
//
// It is the selection counterpart of resilience.Manager and follows the fault
// kernel's shape ([fault.Manager]): the governance center owns the config
// DOCUMENT (parsing and the source) and PUSHES this
// Manager its two data halves — the default [Selection] and a rule list. The
// Manager owns the matching (exact service, else the default) and the rebuild: a push
// that moves a service builds that service's chains a new generation right there,
// on the push, so the request path is one atomic load and nothing else.
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
	// factories builds the strategy a selection names. It is collected at wiring
	// and never changes, so the manager needs no strategy directory of its own.
	factories *Factory

	// snap is the settings as published: one generation, replaced whole by
	// [Manager.Apply]. It is read lock-free by [Manager.Chain]'s arming.
	snap atomic.Pointer[Settings]

	// chains is the per-service table of registered chains, the walk a push
	// rebuilds. It is mutated at wiring ([Manager.Chain]) and read on the loop
	// ([Manager.Apply]) — phases that never overlap, so it is a plain map.
	// [Manager.Close] drops it.
	chains map[string][]*Chain
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
// selection alone.
// There is no arming flag; an empty document and an absent one look the same,
// and both resolve to the base strategy.
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

	// ConsistentHash groups the consistent-hash strategy's own knobs. Only that
	// strategy reads it; another strategy leaves it alone.
	ConsistentHash ConsistentHashParams `value:"${consistent-hash:=}"`

	// ZoneAware groups the zone-aware filter's own knobs, read only when Balancer
	// names it.
	ZoneAware ZoneAwareParams `value:"${zone-aware:=}"`

	// Params is the extension seam for strategies this package does not know: a
	// contributed balancer reads the keys it owns out of this map and rejects
	// the rest itself. Built-in strategies do not read it and simply leave it
	// alone, so a stray group neither fails the document nor changes their
	// behaviour.
	Params map[string]string `value:"${balancer-params:=}"`

	// OutlierThreshold is the consecutive-failure count that suspends an
	// endpoint. 0 disables suspension.
	OutlierThreshold int `value:"${outlier-threshold:=0}" expr:"$ >= 0"`

	// OutlierSuspendFor is the cool-down before a suspended endpoint gets a
	// half-open trial request. Ignored when [Selection.OutlierThreshold] is 0.
	OutlierSuspendFor time.Duration `value:"${outlier-suspend-for:=0}" expr:"int($) >= 0"`

	// Selector narrows the endpoint set this service may choose from, before the
	// strategy picks — a conjunction over the endpoint's metadata, see
	// [matcher.Selector]. It is the DYNAMIC half of endpoint narrowing: the
	// static half is the discovery query's selector, fixed by the client's own
	// configuration, and when both are set both must hold. Because
	// it rides the selection document, a pushed rule can move traffic between
	// versions or zones without a redeploy, and a change rebuilds the chain's
	// generation like any other selection change.
	//
	// Empty means no narrowing. A selector that matches nothing leaves the pool
	// with no candidates, which surfaces as [ErrNoEndpoints] — there is
	// deliberately no implicit fallback, because silently falling back would
	// make a narrow rule (a canary, say) widen to every instance without anyone
	// being told.
	Selector matcher.Selector `value:"${selector:=}"`
}

// Equal reports whether s and other carry the same policy. Params and Selector
// are compared by content, so a selection rebuilt from the same document is
// equal even though it is a different value. It runs once per chain per push,
// never on the request path.
func (s Selection) Equal(other Selection) bool {
	return s.Balancer == other.Balancer &&
		s.OutlierThreshold == other.OutlierThreshold &&
		s.OutlierSuspendFor == other.OutlierSuspendFor &&
		s.ConsistentHash == other.ConsistentHash &&
		s.ZoneAware == other.ZoneAware &&
		maps.Equal(s.Params, other.Params) &&
		slices.Equal(s.Selector, other.Selector)
}

// NewManager returns an unarmed Manager over the collected strategy factories.
// factories is required.
//
// The manager starts on an empty document; resolution fills the global base
// strategy (round-robin), so every chain builds that generation until the first
// [Manager.Apply]. Chains taken before that are remembered and rebuilt by that
// Apply.
func NewManager(factories *Factory) *Manager {
	if factories == nil {
		panic("loadbalance: NewManager requires a non-nil Factory")
	}
	m := &Manager{factories: factories, chains: map[string][]*Chain{}}
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
// service leaves that service's chains — and their in-flight counts, hash rings and
// observed failures — untouched.
//
// The settings REPLACE the previous ones wholesale rather than merging, so a
// document that drops a rule disarms that service — there is no separate off
// switch to remember.
//
// ctx is the source loop's context, carried into the rejection log line;
// nothing on this path does I/O or runs caller code beyond the builds above.
func (m *Manager) Apply(ctx context.Context, s Settings) error {
	next, err := m.plan(&s)
	if err != nil {
		log.Error(ctx, log.TagAppDef, err,
			"loadbalance: document rejected, keeping the current configuration")
		return err
	}
	// plan touched nothing, so only now does the document take effect — as a
	// whole.
	m.snap.Store(&s)
	for service, chains := range m.chains {
		for i, c := range chains {
			if p := next[service][i]; p != nil {
				c.cur.Store(p)
			}
		}
	}
	return nil
}

// plan is the build half of Apply: it returns the rebuilt pools keyed by
// service, each slice parallel to m.chains[service], without touching the
// snapshot or any chain. A nil entry is a chain the document did not move. A
// failed build therefore leaves the caller exactly where it was, and Apply
// commits the cached pools only once every one of them exists.
//
// Only the selections a registered chain actually resolves to are built. A rule
// for a service this process does not serve is just data in the document; it is
// read when a chain asks for that service, not eagerly to prove it is good.
func (m *Manager) plan(s *Settings) (map[string][]*Pool, error) {
	next := make(map[string][]*Pool, len(m.chains))
	for service, chains := range m.chains {
		pools := make([]*Pool, len(chains))
		sel := findSelection(s, service)
		for i, c := range chains {
			cur := c.cur.Load()
			if sel.Equal(cur.Selection()) {
				continue
			}
			// The base source is the same in every generation (the selection's
			// selector is applied per Pick, not baked into it), so rebuilding
			// over the source in force is rebuilding over the chain's target.
			p, err := m.buildPool(cur.src, sel)
			if err != nil {
				return nil, errutil.Explain(err, "loadbalance: service %q", service)
			}
			pools[i] = p
		}
		next[service] = pools
	}
	return next, nil
}

// findSelection is the service-level resolution, in priority order: the first
// rule whose Service equals the service, else the document default. A
// selection that names no strategy is not a usable selection, so it is replaced
// by the global base — round-robin — which is also what an unconfigured
// capability resolves to. List order therefore decides when a service repeats.
// It runs where a push or an arming needs the answer — on the manager, never on
// the request path.
func findSelection(s *Settings, service string) Selection {
	sel := s.Selection
	for _, r := range s.Rules {
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

// buildPool assembles one INNER-LAYER generation from s over src: the strategy
// the selection names, plus a fresh tracker carrying the selection's suspension
// thresholds, bound together by a new [Pool]. It is a pure function: it reads
// nothing but its arguments and touches no chain state, so a failed build leaves
// the caller exactly where it was.
//
// s is already resolved (see [findSelection]), so its strategy name is present.
// An earlier generation is gone by design — there is nothing "in force" to keep.
func (m *Manager) buildPool(src discovery.Resolver, s Selection) (*Pool, error) {
	bal, err := m.factories.Create(s)
	if err != nil {
		return nil, err
	}
	p := NewPool(src, bal, NewTracker(TrackerConfig{
		Threshold:  s.OutlierThreshold,
		SuspendFor: s.OutlierSuspendFor,
	}), s)
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
func (m *Manager) Chain(ctx context.Context, src discovery.Resolver, service string) (*Chain, error) {
	sel := findSelection(m.snap.Load(), service)
	p, err := m.buildPool(src, sel)
	if err != nil {
		return nil, err
	}
	c := &Chain{}
	c.cur.Store(p)
	m.chains[service] = append(m.chains[service], c)
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
