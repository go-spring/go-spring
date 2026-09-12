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

// Package governance is the centralized service-governance authority for the
// process. Where each client starter used to bind its own gs.Dync[resilience.
// Config] and subscribe its own OnChanged handler — eleven near-identical
// copies across redis/gorm/mongo/es/neo4j/bigcache/memcached/gin/http-client/
// gateway — govern collapses that to ONE refreshable [Config] and ONE fan-out.
//
// The governance authority is a process singleton, but callers never hold or
// name a [*center]: the package exposes a set of free functions (the "facade" in
// global.go — [Enabled], [Driver], [PolicyFor], [Register], [OnReady]) that are
// the sole public surface. [*center] is an internal implementation detail,
// built and registered by starter-govern; nothing outside this package ever
// obtains one. This mirrors the neutral global seams the package already exposes
// for resilience ([resilience.ExecutorFor]) and fault injection, but is the
// direct surface for callers (like starter-dubbo) that already import governance.
//
// Config awareness is contract-first: the center consumes its own [Source]
// interface (source.go) — a snapshot plus a change subscription — and nothing
// else. This package is container-free: it imports no IoC concepts at all, so
// the whole governance family is usable from any runtime. The gs wiring (the
// bean registration, binding the injected [Source], the seam arming) lives in
// the starter-governance module — importing that starter is what makes
// cloud/governance live in a gs app, exactly like any other starter/core pair.
//
// Governance scope: govern covers every client that goes through the
// resilience Executor seam. dubbo, which has its own URL-param governance
// model, is adapted separately (its timeout/retries are driven from the same
// center via an adapter); dubbo-unique knobs (loadbalance/cluster/serialization)
// stay in a dubbo-specific config section.
package governance

import (
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
)

// Config is the single source of truth for governance. A [Source] delivers it
// (starter-governance binds the bean-injected source); every client reads a
// resolved policy from the [center] instead of carrying its own resilience
// config. Driver/Enabled
// live at the top level so all resources share one backend selection and one
// on/off switch; per-resource policies live under Rules, matched by the same
// label passed to [center.policyFor] (e.g. "redis:cache", "gorm:mysql:primary",
// "gin:api", "dubbo:com.example.Foo:1.0.0").
type Config struct {
	// Enabled gates the whole center. When false, PolicyFor always returns a
	// zero Policy (transparent pass-through) regardless of Default/Rules,
	// so importing starter-governance with no configured source is a no-op.
	Enabled bool `value:"${enabled:=false}"`

	// Driver names the resilience backend all resources use ("default" or
	// "sentinel"). Centralizing the driver means one place switches the backend
	// for every client, instead of each starter's own ${...driver}.
	Driver string `value:"${driver:=default}"`

	// Default is the policy applied to every resource that no Rule matches.
	// Most deployments set only this and let every resource share it; per-resource
	// exceptions go under Rules. It embeds resilience.PolicyConfig — NOT
	// resilience.Config — so only policy knobs are bindable here; the on/off
	// switch and backend selection are process-wide at the top of the document
	// ([Config.Enabled], [Config.Driver]) and deliberately NOT re-bindable per
	// resource. Bind via govern.default.* (e.g. govern.default.attempt-timeout=500ms).
	Default resilience.PolicyConfig `value:"${default:=}"`

	// Rules are per-resource policy entries. PolicyFor returns the first Rule
	// whose Resources contains the label; when no Rule matches it returns
	// Default. Each Rule's embedded resilience.Config fully replaces Default
	// for the matched resource — not a field-wise merge: a resilience.Policy
	// field of 0 means "disabled", so a partial merge could not distinguish
	// "explicitly set to 0" from "left unset". List more specific Rules first.
	// Bind via indexed properties:
	//
	//	govern.rules[0].resources=redigo:cache
	//	govern.rules[0].timeout=100ms
	//	govern.rules[1].resources=gorm:mysql:orders,gorm:mysql:logs
	//	govern.rules[1].timeout=3s
	//
	// The resource label (with its colons) lives in a value, not a key, so it
	// is dot-safe and needs no escaping in .properties or YAML — unlike a
	// map keyed by label, where the colon would have to appear in the key.
	Rules []Rule `value:"${rules:=}"`

	// Fault is the process-wide fault-injection config (chaos engineering), a
	// sibling concern to resilience governance that rides the SAME [Config]
	// (and so the same source) rather than its own. starter-governance builds
	// one global *fault.Injector from it (in [center.goLive]) and registers it
	// behind the neutral [fault.InjectorFor] seam, so every client/server
	// starter resolves fault injection through that seam instead of each
	// binding its own fault config. Per-
	// resource fault differences live under fault.Config.Rules (matched by the
	// same resource label passed to the executor/Apply seam). A zero Fault
	// (Enabled false) injects nothing. Bind via govern.fault.* (e.g.
	// govern.fault.enabled=true, govern.fault.rate=0.5).
	Fault fault.Config `value:"${fault:=}"`
}

// Rule is one per-resource policy entry. Resources are the resource labels it
// applies to (exact match against any of them); the first matching Rule in
// [Config.Rules] wins, so list specific Rules before broad ones. The embedded
// resilience.PolicyConfig supplies the policy fields and binds at the same key
// as the Rule (govern.rules[n].attempt-timeout, .max-retries, ...), since gs
// promotes value tags through an embedded struct. Like Default it carries no
// Enabled/Driver: those are process-wide, not per-resource.
type Rule struct {
	// Resources are the resource labels this Rule matches, exact-compare. A
	// resource label is what a starter passes to the executor/fault seam — e.g.
	// "redis:cache", "gorm:mysql:primary", "http:user-svc", "gin::8080". Comma-
	// separated for multiple. Empty matches nothing (use Default instead).
	Resources []string `value:"${resources:=}"`
	resilience.PolicyConfig
}

// Center is the runtime governance authority. It holds an atomic snapshot of
// the current [Config] and, on [center.refresh], notifies every registered
// subscriber whose resolved policy changed — so a single config push fans out
// to all clients through one OnChanged handler, not one per starter bean.
// Safe for concurrent use.
//
// The center is container-free: it knows sources only through the [Source]
// contract. The gs wiring (binding the injected source, arming
// the seams, marking the authority live) lives in starter-governance's wiring
// bean, which drives this package's facade ([BindDefault], [GoLive]).
// [newCenter] is the direct construction path used by tests and [Arm].
type center struct {
	cfg atomic.Pointer[Config]

	mu   sync.Mutex
	subs map[string][]*subscriber // label -> subscribers

	// srcMu guards src; the handle indirection is what makes source
	// replacement safe: callbacks from a REPLACED source are dropped by
	// comparing handle pointers (never interface values, which can panic on
	// non-comparable dynamic types), because a replaced source cannot always
	// be unsubscribed — so stale callbacks must no-op instead of retracted.
	srcMu sync.Mutex
	src   *sourceHandle // active source; nil until BindDefault or SetSource binds one

	// injector is the ONE process-wide fault injector, built by [Center.goLive]
	// from the bound source's Fault config. It is always built (a disabled
	// injector is a no-op), so fault can be toggled on at runtime via
	// hot-reload; its config is swapped in place from the adopt sink.
	injector *fault.Injector
}

// subscriber is one client's interest in the policy for a label. last is the
// most recent policy delivered to cb, so Refresh can skip callbacks whose
// policy is unchanged (no spurious executor rebuilds on an unrelated key
// change) and avoid re-delivering the same value.
type subscriber struct {
	last resilience.Policy
	cb   func(resilience.Policy)

	// cancelled marks a subscriber whose [Subscription.Cancel] has run. The
	// center drops it from subs on cancel, but a Refresh already in flight may
	// hold a reference to it from before the removal — so Refresh checks this
	// flag under mu and skips it, making cancel take effect immediately rather
	// than after the next refresh.
	cancelled bool
}

// Subscription is a live registration created by [Register]. It carries the
// policy the callback was armed with and the ability to detach it again, which
// matters for callers whose protected objects are not process-lifetime: a
// gateway rebuilding its route table, or any client re-created on a config
// change. Without Cancel, every rebuild would leave a subscriber (and a
// reference to the discarded object) behind in the center forever.
//
// The zero value is inert: Policy is a zero pass-through policy and Cancel is a
// no-op, so a Subscription is always safe to hold and always safe to cancel —
// including more than once.
type Subscription struct {
	// Policy is the policy the callback was armed with at registration time.
	Policy resilience.Policy

	cancel func()
}

// Cancel detaches the subscription. It is idempotent and safe to call
// concurrently with a config push: once it returns, the callback is no longer
// invoked. Cancelling the zero Subscription is a no-op.
func (s Subscription) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

// sourceHandle tokens the active [Source] so callbacks from a REPLACED source
// can be identified and dropped without comparing interface values (which can
// panic on non-comparable dynamic types).
type sourceHandle struct{ src Source }

// newCenter snapshots cfg and returns a Center that resolves policies from it.
// The cfg is adopted atomically; callers mutate it only via Refresh. This is the
// direct construction path (tests, [Arm]); the starter-managed path uses the
// package singleton in global.go plus [BindDefault] and [GoLive].
func newCenter(cfg Config) *center {
	c := &center{subs: map[string][]*subscriber{}}
	c.cfg.Store(&cfg)
	return c
}

// goLive completes the center's startup on the package singleton: it builds the
// process-wide fault injector from the CURRENT snapshot, registers the
// executor/fault seams, and marks the authority live (firing any [OnReady]
// callbacks). Idempotent. The wiring starter calls it once after binding the
// default source; registering the seams here (rather than in a Runner.Run) is
// safe: both resolve lazily at call time.
func (c *center) goLive() {
	if c.injector != nil {
		return
	}
	cfg := *c.cfg.Load()
	c.injector = fault.NewInjector(cfg.Fault)
	resilience.RegisterExecutorProvider(c.executorFor)
	fault.RegisterInjector(c.injector)
	loadbalance.RegisterSelectionProvider(c.selectionFor)
	markLive()
}

// adopt applies one config from whatever source: it fans resilience out via
// [center.refresh] AND swaps the fault injector's config. It is the single sink
// for ALL source pushes, which is what keeps the fault chain source-agnostic —
// fault.Config always rides inside [Config], so any Source drives it for free.
func (c *center) adopt(cfg Config) {
	c.refresh(cfg)
	if c.injector != nil {
		c.injector.SetConfig(cfg.Fault)
	}
}

// bindSource makes s the active source: install its handle, subscribe with a
// stale guard (callbacks from a previously bound source no-op), then adopt s's
// snapshot. The explicit snapshot adopt matches the [Source] contract that
// Subscribe delivers changes only — Snapshot seeds the present.
func (c *center) bindSource(s Source) {
	h := &sourceHandle{src: s}
	c.srcMu.Lock()
	c.src = h
	c.srcMu.Unlock()
	s.Subscribe(func(cfg Config) {
		if !c.isActive(h) {
			return // replaced source: drop
		}
		c.adopt(cfg)
	})
	c.adopt(s.Snapshot())
}

// isActive reports whether h is still the bound source handle.
func (c *center) isActive(h *sourceHandle) bool {
	c.srcMu.Lock()
	defer c.srcMu.Unlock()
	return c.src == h
}

// setSource eagerly binds s as the active source. Before the wiring starter's
// [BindDefault] it pre-empts the default source; afterwards it late-arms —
// s.Snapshot() applies immediately and later pushes drive the center, while the
// previous source's callbacks go stale via the handle guard. Eager binding
// avoids a pending-registration state entirely, and works on the standalone
// path (Arm-built centers, tests) where nothing would ever consume a pending
// value.
func (c *center) setSource(s Source) {
	if s == nil {
		panic("governance: SetSource(nil)")
	}
	c.bindSource(s)
}

// bindDefault installs s as the active source only when none is bound yet, so
// an explicit [SetSource] (callable at any time) always outranks the wiring
// default. The wiring starter calls it once at startup with the bean-injected
// source. The check-then-bind is
// not atomic with concurrent SetSource, but wiring runs single-threaded before
// the app serves; the guard machinery makes a lost race harmless anyway (the
// loser's callbacks go stale).
func (c *center) bindDefault(s Source) {
	if s == nil {
		return
	}
	c.srcMu.Lock()
	bound := c.src != nil
	c.srcMu.Unlock()
	if !bound {
		c.bindSource(s)
	}
}

// executorFor is the governance-backed provider registered with
// resilience.RegisterExecutorProvider. For a resource label it builds the
// executor the center resolves (the center's driver + the label's policy) and
// subscribes it to policy changes — so a hot-reload of the governance config refreshes the
// executor in place. It is a pure function: the ONLY memoization is the
// LoadOrStore cache in resilience.resolve (provider.go), which also guarantees
// this provider is invoked at most once per label, so the Register
// subscription is armed exactly once even under concurrent first use.
func (c *center) executorFor(label string) resilience.Executor {
	exec, err := resilience.NewExecutor(c.driver(), c.policyFor(label))
	if err != nil || exec == nil {
		return nil // resilience.resolve falls back to a no-op executor
	}
	// The subscription lives as long as the executor, which the resolve cache
	// keeps for the process lifetime, so there is nothing to cancel here.
	_ = c.register(label, func(p resilience.Policy) { _ = exec.Refresh(p) })
	return exec
}

// selectionFor is the governance-backed provider registered with
// loadbalance.RegisterSelectionProvider. It resolves label's policy — the SAME
// label (and so the same rule) that drives the resource's protection executor,
// which is why endpoint selection and protection are configured in one place —
// and applies its selection half to the caller's pool now and on every change.
//
// Unlike [center.executorFor] there is no per-label memoization to hang the
// subscription on: the sink is the caller's pool, and one label may legitimately
// back several pools (two entries sharing a service name, a rebuilt client). Each
// bind therefore gets its own subscription, and the returned stop func is its
// Cancel, so a pool that goes away detaches instead of leaving a callback
// pointing at it. A zero policy (governance disabled, or a label with no rule)
// applies as "keep the strategy, disable suspension" — a no-op.
func (c *center) selectionFor(label string, apply func(loadbalance.Selection)) func() {
	sub := c.register(label, func(p resilience.Policy) {
		apply(loadbalance.Selection{
			Balancer:          p.Balancer,
			OutlierThreshold:  p.OutlierThreshold,
			OutlierSuspendFor: p.OutlierSuspendFor,
		})
	})
	return sub.Cancel
}

// Destroy closes the active source when it happens to be closeable (the
// [Source] contract keeps Close optional — see source.go), else it is a no-op:
// the center itself holds only in-memory subscribers and an atomic snapshot.
func (c *center) destroy() error {
	c.srcMu.Lock()
	h := c.src
	c.srcMu.Unlock()
	if h != nil {
		if cl, ok := h.src.(interface{ Close() error }); ok {
			return cl.Close()
		}
	}
	return nil
}

// Enabled reports whether the center is armed. When false, PolicyFor returns a
// transparent pass-through policy and Register arms clients with a zero Policy.
func (c *center) enabled() bool {
	if cfg := c.cfg.Load(); cfg != nil {
		return cfg.Enabled
	}
	return false
}

// Driver returns the configured resilience driver name, defaulting to "default"
// when unset. Clients use it to resolve the Executor backend once, centrally,
// rather than each reading its own ${...driver} knob.
func (c *center) driver() string {
	if cfg := c.cfg.Load(); cfg != nil && cfg.Driver != "" {
		return cfg.Driver
	}
	return resilienceDefaultDriver
}

const resilienceDefaultDriver = "default"

// PolicyFor returns the resolved policy for label: the first Rule whose
// Resources contains label, otherwise Default. When the center is disabled it
// returns a zero Policy so an executor armed from it is a transparent
// pass-through. The read is lock-free (atomic pointer load), so the hot path —
// every protected call's caller reads nothing here, only the executor setup
// does — never contends.
func (c *center) policyFor(label string) resilience.Policy {
	cfg := c.cfg.Load()
	if cfg == nil || !cfg.Enabled {
		return resilience.Policy{}
	}
	for _, r := range cfg.Rules {
		if slices.Contains(r.Resources, label) {
			return r.Policy()
		}
	}
	return cfg.Default.Policy()
}

// Register subscribes cb to policy changes for label and arms it immediately
// with the current resolved policy. cb is then invoked whenever [refresh]
// produces a different policy for label. This is how a client replaces its
// per-bean OnChanged handler: one Register per resource, all driven by the
// center's single source.
//
// cb is always called outside the center's lock, so a callback that itself
// calls into the Center cannot self-deadlock; it MUST be safe for concurrent
// invocation, since a Refresh may fire concurrently with this immediate call.
// resilience.Executor.Refresh satisfies that.
//
// The returned [Subscription] carries the policy cb was armed with and detaches
// the registration on [Subscription.Cancel].
func (c *center) register(label string, cb func(resilience.Policy)) Subscription {
	cur := c.policyFor(label)
	s := &subscriber{last: cur, cb: cb}
	c.mu.Lock()
	c.subs[label] = append(c.subs[label], s)
	c.mu.Unlock()
	cb(cur)
	return Subscription{Policy: cur, cancel: func() { c.unregister(label, s) }}
}

// unregister detaches s from label. It marks s cancelled before removing it, so
// a Refresh that already collected s into its pending list (under mu, before
// this call) still skips it at delivery time. Removing the last subscriber for
// a label drops the map entry, so a repeatedly-rebuilt client leaves no residue
// — not even an empty slice.
func (c *center) unregister(label string, s *subscriber) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s.cancelled = true
	list := c.subs[label]
	for i, e := range list {
		if e == s {
			list = append(list[:i: i], list[i+1:]...)
			break
		}
	}
	if len(list) == 0 {
		delete(c.subs, label)
		return
	}
	c.subs[label] = list
}

// Refresh adopts cfg as the new governance config and notifies every registered
// subscriber whose resolved policy for its label changed. It is the single
// entry point starter-govern calls from its one OnChanged handler — one call
// fans out to all resources, which is the whole point of centralizing.
//
// Notifications are collected under the lock (so last is updated consistently)
// but delivered outside it. A subscriber whose policy is unchanged is not
// notified, keeping a localized change from churning unrelated executors.
func (c *center) refresh(cfg Config) {
	c.cfg.Store(&cfg)
	type pending struct {
		cb func(resilience.Policy)
		p  resilience.Policy
	}
	var todo []pending
	c.mu.Lock()
	for label, list := range c.subs {
		next := c.policyFor(label)
		for _, s := range list {
			// A subscriber cancelled since this refresh began must not be
			// notified: its owner is already tearing down what cb drives.
			if s.cancelled {
				continue
			}
			if !policyEqual(s.last, next) {
				s.last = next
				todo = append(todo, pending{cb: s.cb, p: next})
			}
		}
	}
	c.mu.Unlock()
	for _, p := range todo {
		p.cb(p.p)
	}
}

// policyEqual reports whether two center-resolved policies are equivalent for
// the purpose of change detection. resilience.Policy cannot be compared with ==
// because it carries a RetryPredicate func field; but policies produced by the
// center always come from resilience.Config.Policy(), which leaves
// RetryPredicate nil (funcs cannot be bound from value tags), so reflect.DeepEqual
// is exact here. Even if a caller hand-constructs a Config, DeepEqual treats two
// non-nil funcs as equal only when identical, which is the desired semantic.
func policyEqual(a, b resilience.Policy) bool {
	return reflect.DeepEqual(a, b)
}
