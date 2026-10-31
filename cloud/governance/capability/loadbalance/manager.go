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
	"sync/atomic"
	"time"

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/log"
)

// Manager is the centralized endpoint-selection authority: it maps a resource
// label to a [Selection] and rebuilds every [Chain] registered under a label
// whose selection moved.
//
// It is the selection counterpart of resilience.Manager and follows the fault
// kernel's shape ([fault.Manager]): the governance center owns the config
// DOCUMENT (parsing, the source, duplicate-label rejection) and PUSHES this
// Manager its two data halves — a [Settings.Default] and a rule list. The
// Manager owns the matching (exact label, else Default) and the rebuild: a push
// that moves a label builds that label's chains a new generation right there,
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
	// snap is the settings as published: one generation, replaced whole by
	// [Manager.Apply]. It is read lock-free by [Manager.Chain]'s arming.
	snap atomic.Pointer[Settings]

	// chains is the per-label table of registered chains, the walk a push
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
// Every RULE must name a strategy: the zero Selection is not valid (see [New]).
// The DEFAULT may name none — a governance document is allowed to configure only
// other capabilities and leave selection alone — and then means the base
// configuration, round-robin with suspension off (see [Settings.withDefaults]).
// There is no arming flag; an empty document and an absent one look the same,
// and both are a pass-through.
type Settings struct {
	// Default is the selection every label runs under when no Rule matches it.
	Default Selection `value:"${:=}"`

	// Rules are the per-label overrides: the first Rule whose Service equals the
	// label wins. An empty Service matches NOTHING (it is not a catch-all — use
	// Default for that), so a rule list cannot accidentally govern every label.
	//
	// Labels are unique across Rules — [Settings.Validate] rejects a duplicate
	// document before it reaches here — because a linear scan would otherwise let
	// list order decide which entry applies.
	Rules []Rule `value:"${services:=}"`
}

// Validate is called by conf.Bind, so an invalid document is rejected at the
// source boundary rather than reaching the center.
func (s Settings) Validate() error {
	return Validate(s)
}

// Rule is one per-label selection override. See [Settings.Rules].
type Rule struct {
	// Service is the resource label this Rule matches, exact-compare — the same
	// identity a client starter registers its chain under (see [Manager.Chain]).
	Service string `value:"${service:=}"`

	// Selection is what the matched label runs under.
	Selection
}

// Selection is the resolved endpoint-selection policy: the strategy (by name),
// the strategy's own parameters, and the outlier-suspension thresholds.
//
// It is the COMPLETE description of how a label selects endpoints: a resolved
// selection leaves nothing to the configuration it replaces (see [Chain]). Like
// the resilience capability policies it is ONE definition — the bindable form IS
// the runtime form; there is no separate binding twin.
type Selection struct {
	// Balancer is the strategy name to select endpoints with (e.g.
	// "least_conn"). It is REQUIRED: it names an entry in the process's strategy
	// registry — a built-in strategy, or one a deployment contributed as a
	// [Factory] bean. An empty name is a construction error (see [New]); the
	// base strategy is stated rather than implied, both by the base document an
	// unarmed manager serves and by the governance translation.
	Balancer string `value:"${balancer:=}"`

	// ConsistentHash groups the consistent-hash strategy's own knobs, and is
	// read only when Balancer names it (see [Validate], which rejects a set
	// group under any other strategy — a parameter aimed at another strategy is
	// an error, never a silently dropped value).
	ConsistentHash ConsistentHashParams `value:"${consistent-hash:=}"`

	// ZoneAware groups the zone-aware filter's own knobs, read only when
	// Balancer names it (see [ConsistentHash] for the strictness rule).
	ZoneAware ZoneAwareParams `value:"${zone-aware:=}"`

	// Params is the extension seam for strategies this package does not know: a
	// contributed balancer reads the keys it owns out of this map and rejects
	// the rest itself, exactly as the built-in groups reject theirs. It is
	// valid only under a contributed Balancer name — a rule naming a built-in
	// strategy with loose params is rejected whole (see [Validate]).
	Params map[string]string `value:"${balancer-params:=}"`

	// OutlierThreshold is the consecutive-failure count that suspends an
	// endpoint. 0 disables suspension.
	OutlierThreshold int `value:"${outlier-threshold:=0}" expr:"$ >= 0"`

	// OutlierSuspendFor is the cool-down before a suspended endpoint gets a
	// half-open trial request. Ignored when [Selection.OutlierThreshold] is 0.
	OutlierSuspendFor time.Duration `value:"${outlier-suspend-for:=0}" expr:"int($) >= 0"`

	// Selector narrows the endpoint set this label may choose from, before the
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

// NewManager returns an unarmed Manager. It resolves strategy names against the
// package registry (see [Register]), which is process-wide, so a manager needs
// no strategy directory of its own.
//
// The manager starts on the base document (round-robin), so every chain builds
// the base generation until the first [Manager.Apply]. Chains taken before that
// are remembered and rebuilt by that Apply.
func NewManager() *Manager {
	m := &Manager{chains: map[string][]*Chain{}}
	// Publish the base document from the start rather than leaving the snapshot
	// absent, so a chain armed before the first push takes a generation number
	// rather than none, and the first real document only rebuilds the labels it
	// actually moves. The balancer is named explicitly because [New] refuses an
	// empty one; round-robin is the base strategy, not an implied fallback.
	m.snap.Store(&Settings{Default: Selection{Balancer: RoundRobin}})
	return m
}

// Apply adopts s as the manager's settings and rebuilds every chain whose label
// the document moved. It is the single entry point the governance center calls
// — once at startup with the source's snapshot, then on every push — and it
// runs on that source's single loop, which is why it takes no lock.
//
// The rebuild is decided by CONTENT, not by the fact that Apply ran: a label's
// generation is the fingerprint of the selection it resolves to (see
// [labelGen]), so a push that does not move a label leaves that label's chains
// — and their in-flight counts, hash rings and observed failures — untouched.
//
// The settings REPLACE the previous ones wholesale rather than merging, so a
// document that drops a rule disarms that label — there is no separate off
// switch to remember.
//
// ctx is threaded into the chains' rebuild log lines; nothing on this path does
// I/O or runs caller code any more than that.
func (m *Manager) Apply(ctx context.Context, s Settings) {
	s = s.withDefaults()
	m.snap.Store(&s)
	for label, cs := range m.chains {
		for _, c := range cs {
			m.adopt(ctx, label, c, resolveSelection(&s, label))
		}
	}
}

// adopt moves c onto sel when the selection it resolves has moved. It is the
// manager's write: the chain is a passive holder, and everything that compares,
// builds, logs and swaps happens here, on the manager's single loop.
//
// The change test reads the generation in force: the selection recorded there is
// the only record the chain keeps, so "what to compare against" and "what is
// serving" cannot disagree. A push that leaves the label's answer where it was
// rebuilds nothing, so the generation's tracker state, hash ring and in-flight
// counts survive.
//
// A strategy the selection names that fails to build is not fatal: the current
// generation stays in force, so a bad rule degrades a chain to its last good
// configuration instead of taking its client down — the rule the governance
// source's contract forces, since a push has no error channel. A repeated push
// of the same bad document therefore retries the build and logs again; the
// attempt fails before allocating anything, and each push is its own event.
func (m *Manager) adopt(ctx context.Context, label string, c *Chain, sel Selection) {
	cur := c.cur.Load()
	if sameSelection(sel, cur.Selection()) {
		return
	}

	// The base source is the same in every generation (the selection's selector
	// is applied per Pick, not baked into it), so rebuilding over the source in
	// force is rebuilding over the chain's target.
	p, err := buildGeneration(cur.src, sel)
	if err != nil {
		log.Warn(ctx, log.TagAppDef,
			"loadbalance: endpoint selection rejected, keeping the resource's current strategy",
			log.String("label", label),
			log.Err(err))
		return
	}
	c.cur.Store(p)
}

// Chain builds a governed [Chain] for label and arms it in one step, so a
// caller cannot obtain a chain that still runs a stale selection.
//
// The first generation is built from the selection in force at this moment, not
// from the zero selection, so a chain created after the document arrived
// already runs its policy. An unarmed manager serves the base document, which
// builds the round-robin chain. A selection that fails to build — a rule naming an
// unregistered strategy — is a CONSTRUCTION error: the chain does not exist yet,
// there is nothing to keep in force, and the caller learns it the way it learns
// every other construction failure. Only a LATER push degrades to the last good
// generation (see [Chain]'s doc), because a push has no error channel.
//
// The chain is registered under its label for every later [Manager.Apply] to
// rebuild. Registration happens at wiring, before the source loop starts (see
// the Manager doc), and there is no detach: the manager keeps the pointer until
// [Manager.Close].
func (m *Manager) Chain(ctx context.Context, src discovery.Resolver, label string) (*Chain, error) {
	c := &Chain{}
	sel := resolveSelection(m.snap.Load(), label)
	p, err := buildGeneration(src, sel)
	if err != nil {
		return nil, err
	}
	c.cur.Store(p)
	m.chains[label] = append(m.chains[label], c)
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
// label. It owns a whole endpoint-selection chain — the [Balancer], its
// [Tracker] and the inner [Pool] that binds them to a discovery source — as an
// immutable generation, REBUILT FROM SCRATCH by [Manager.Apply] whenever the
// selection its label resolves to moves. Callers hold the chain, not the pool:
// [Chain.Pick], [Chain.Complete] and [Chain.Selection] are the
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
// The label IS the key: several chains may share a label (two clients on one
// service name), and each is rebuilt independently by the same push.
//
// A chain carries no identity of its own beyond the generation it points at.
// The label it follows is its key in the manager's registration table, and the
// source its generations are built over lives inside each [Pool] — the base
// source, unchanged from one generation to the next, because the selection's
// selector is applied in [Pool.Pick] rather than baked into the source. A
// TARGET change (another service, another discovery backend) is therefore a new
// registration and a new chain, which is the caller's decision: only the caller
// knows what its upstream points at.
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
// Pick is a pure read: one atomic load, then the pool's own work. Refresh
// happens entirely on [Manager.Apply], so a Pick never triggers, waits on, or
// observes a half-built generation.
//
// ctx is the request's, and is threaded to the pool's endpoint source.
func (c *Chain) Pick(ctx context.Context, info PickInfo) (discovery.Endpoint, error) {
	return c.cur.Load().Pick(ctx, info)
}

// Complete settles the request that Pick issued to ep. Invoke it exactly once
// per picked endpoint, after the request ends. See [Pool.Complete].
func (c *Chain) Complete(ctx context.Context, ep discovery.Endpoint, err error) {
	c.cur.Load().Complete(ctx, ep, err)
}

// Selection returns the selection the generation in force was built from. It is
// an inspection and test helper.
func (c *Chain) Selection() Selection { return c.cur.Load().Selection() }

// buildGeneration assembles one INNER-LAYER generation from s over src: the
// strategy the selection names, plus a fresh tracker carrying the selection's
// suspension thresholds, bound together by a new [Pool]. It is a pure function:
// it reads nothing but its arguments and touches no chain state, so a failed
// build leaves the caller exactly where it was.
//
// The resolved selection is the COMPLETE truth for this label, so the strategy
// name must be present here: unlike in-place application there is no "keep what
// is in force" ([Selection.Balancer]) — an earlier generation is gone by
// design. The base strategy is round-robin, but it is stated by whatever
// produces the [Settings], not implied by an absent name.
func buildGeneration(src discovery.Resolver, s Selection) (*Pool, error) {
	bal, err := New(s)
	if err != nil {
		return nil, err
	}
	p := NewPool(src, bal, NewTracker(TrackerConfig{
		Threshold:  s.OutlierThreshold,
		SuspendFor: s.OutlierSuspendFor,
	}), s)
	return p, nil
}

// resolveSelection is the label-level resolution: the first rule whose Service
// equals the label, else the default. Labels are unique across rules (the
// center rejects a duplicate document), so list order cannot decide which entry
// applies. It runs where a push or an arming needs the answer — on the manager,
// never on the request path.
func resolveSelection(s *Settings, label string) Selection {
	for _, r := range s.Rules {
		if r.Service == label {
			return r.Selection
		}
	}
	return s.Default
}

// sameSelection reports whether two selections carry the same policy. Params is
// a map, so equality is field-by-field with the map compared by key set — it
// runs once per chain per push, never on the request path.
func sameSelection(a, b Selection) bool {
	if a.Balancer != b.Balancer ||
		a.OutlierThreshold != b.OutlierThreshold ||
		a.OutlierSuspendFor != b.OutlierSuspendFor ||
		a.ConsistentHash != b.ConsistentHash ||
		a.ZoneAware != b.ZoneAware ||
		len(a.Params) != len(b.Params) ||
		len(a.Selector) != len(b.Selector) {
		return false
	}
	for k, v := range a.Params {
		if b.Params[k] != v {
			return false
		}
	}
	for i, t := range a.Selector {
		if b.Selector[i] != t {
			return false
		}
	}
	return true
}
