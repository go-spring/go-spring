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
// process: it owns ONE refreshable [Config] document and ONE source, and
// distributes them to the module authorities that do the actual work —
// resilience, endpoint selection and fault injection. It also holds the
// discovery directory ([discovery.Manager]), which no rule applies to: it is
// carried so a client has one injection point for everything it needs, not a
// fourth policy.
//
// The division of labour is deliberate. This package owns the DOCUMENT: the
// rule list, the label matching, the source handling. The module packages own
// what to DO with a resolved policy: the executor cache, the driver directory,
// the pool subscriptions, the injector state. A module never imports this
// package; the center reaches it through a plain function type each module
// declares for itself ([resilience.Settings.ResolveClientPolicy], [loadbalance.Settings.
// Resolve]) — which is what keeps the modules usable — and testable — without a
// governance center at all.
//
// The center is container-free: it imports no IoC concept. The gs wiring (the
// beans, the injected [Source], the injected module managers) lives in
// starter-governance. Reference/config awareness is contract-first: the center
// consumes its own [Source] — a snapshot plus a change subscription — and
// nothing else, so any pusher (a governance console stream, a config-center
// listener, a static injection) can drive the whole family by implementing two
// methods.
//
// A client injects the ONE center bean and reads from it whichever authority it
// needs ([Center.Resilience], [Center.Loadbalance], [Center.Fault],
// [Center.Discovery]); each authority is held by the center rather than wired in
// separately, and for the three policy authorities it is the same bean the
// center drives, which is what routes a config push to that client. There is no
// process-wide facade and no global state.
//
// Governance scope: govern covers BOTH directions — every outbound call that runs
// through a resilience executor, and every inbound request that runs through an
// inbound executor (gin / echo / http-server / grpc / thrift middleware). The
// two are separate resolutions over separate blocks of the same document
// ([Config.Client] / [Config.Server]), so a push retunes one without disturbing
// the other. dubbo, which has its own URL-param governance model, is adapted
// separately (its timeout/retries are driven from the same center via an
// adapter); dubbo-unique knobs (loadbalance/cluster/serialization) stay in a
// dubbo-specific config section.
package governance

import (
	"context"
	"sync"
	"sync/atomic"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Center is the runtime governance authority. It holds an atomic snapshot of the
// current [Config] and, on each adopt, distributes it to the three injected
// module authorities — one call fans out to every service, which is the whole
// point of centralizing.
//
// It is container-free: it knows sources only through the [Source] contract and
// modules only through the authorities handed to it, so it imports no IoC
// concept. The gs wiring (the beans, binding the injected source, arming the
// authorities) lives in starter-governance, which builds one [Center] over the
// module beans it also exports for clients to inject.
//
// Safe for concurrent use.
type Center struct {
	cfg atomic.Pointer[Config]

	// res, lb, inj and disc are the module authorities this center distributes
	// to. They are injected rather than constructed here, because each is
	// registered as a bean by the package that owns it, and the starter must
	// hand the same instances on — one instance serves both directions.
	res  *resilience.Manager
	lb   *loadbalance.Manager
	inj  *fault.Injector
	disc *discovery.Manager

	// live guards [Center.GoLive] against a second run — gs reaches it through
	// the bean's Init hook and a test may call it directly on top of that — since
	// re-dispatching would re-notify every subscriber for no reason. It also
	// gates [Center.OnReady], which queues callbacks until it flips.
	live atomic.Bool

	// readyMu guards readyCbs, the [Center.OnReady] callback queue. OnReady is a
	// cold path (startup registration), so a plain mutex is fine; it closes the
	// race between a late OnReady and markLive: whichever wins, each callback
	// runs exactly once — either from the queue at GoLive time or immediately.
	readyMu  sync.Mutex
	readyCbs []func()

	// srcMu guards src; the handle indirection is what makes source
	// replacement safe: callbacks from a REPLACED source are dropped by
	// comparing handle pointers (never interface values, which can panic on
	// non-comparable dynamic types), because a replaced source cannot always
	// be unsubscribed — so stale callbacks must no-op instead of retracted.
	srcMu sync.Mutex
	src   *sourceHandle // active source; nil until the constructor or SetSource binds one
}

// sourceHandle tokens the active [Source] so callbacks from a REPLACED source
// can be identified and dropped without comparing interface values (which can
// panic on non-comparable dynamic types).
type sourceHandle struct{ src Source }

// NewCenter builds a Center over the module authorities and, when src is
// non-nil, immediately binds it as the active source: the center is born with
// the source the container contributed, exactly as the authorities arrive as
// constructor parameters.
//
// The container ALWAYS passes one — the bean's constructor declares src
// required, so a process with no Source bean fails startup rather than running
// with governance silently off. The nil branch exists for a hand-built center
// (an embedder, a test): it leaves the center with no source at all, where
// [Center.SetSource] is the only way in and a center with neither stays disabled
// (every client resolves a transparent pass-through).
//
// The cfg is adopted atomically; callers mutate it only via [Center.adopt]. The
// authorities are owned by the caller — the wiring starter, or a test — so the
// same instances can also be registered as beans for clients to inject.
func NewCenter(cfg Config, res *resilience.Manager, lb *loadbalance.Manager,
	inj *fault.Injector, disc *discovery.Manager, src Source) *Center {
	c := &Center{res: res, lb: lb, inj: inj, disc: disc}
	c.cfg.Store(&cfg)
	if src != nil {
		c.bindSource(src)
	}
	return c
}

// Resilience returns the resilience authority the center distributes to. The
// Center is the governance family's sole spokesperson: a client starter injects
// the ONE *Center bean and reads whichever authority it needs from here,
// instead of wiring each authority bean in separately. The bean is registered
// by the cloud/governance package itself and always instantiated (it exports as
// a gs.Rooter), so a successfully started app never holds a nil Center.
func (c *Center) Resilience() *resilience.Manager {
	return c.res
}

// Fault returns the fault-injection authority; see [Center.Resilience].
func (c *Center) Fault() *fault.Injector {
	return c.inj
}

// Loadbalance returns the endpoint-selection authority; see [Center.Resilience].
func (c *Center) Loadbalance() *loadbalance.Manager {
	return c.lb
}

// Discovery returns the directory of named discovery backends; see
// [Center.Resilience]. It is not a policy authority like the other three — the
// center holds it so a client reaches discovery through the same injection point
// it reaches everything else.
func (c *Center) Discovery() *discovery.Manager {
	return c.disc
}

// GoLive completes the center's startup: it distributes the CURRENT snapshot to
// the module authorities and marks the authority live (firing any [OnReady]
// callbacks). Idempotent — a second call is a no-op.
//
// An enabled center whose configured driver resolves to nothing is a wiring
// error, not a runtime one: the name is latched per service at first resolve, so
// reporting it here — where the wiring can fail startup — beats silently serving
// a pass-through center. (A disabled center builds no executor, so it is not
// checked.) On such an error the authority is left NOT live, so [OnReady]
// callbacks do not fire for a center that cannot serve.
func (c *Center) GoLive() error {
	if !c.live.CompareAndSwap(false, true) {
		return nil
	}
	if err := c.dispatch(*c.cfg.Load()); err != nil {
		return err
	}
	c.markLive()
	return nil
}

// markLive fires every callback queued via [Center.OnReady]. The CompareAndSwap
// in [Center.GoLive] guarantees exactly-once firing; callbacks run outside
// readyMu so a callback may itself call OnReady (which now fires immediately)
// without self-deadlocking.
func (c *Center) markLive() {
	c.readyMu.Lock()
	cbs := c.readyCbs
	c.readyCbs = nil
	c.readyMu.Unlock()
	for _, cb := range cbs {
		cb()
	}
}

// Live reports whether the center has completed [Center.GoLive].
func (c *Center) Live() bool { return c.live.Load() }

// OnReady registers cb to fire exactly once when this center goes live. If it is
// already live, cb fires immediately.
//
// It exists because gs wires Rooters before Runners, so a push-based caller (a
// dubbo adapter, a Rooter) may initialize before the wiring starter (also a
// Rooter) has completed GoLive; OnReady guarantees the caller re-runs its work
// once governance is live, without depending on bean order. The callback belongs
// to the center rather than the package because "governance is live" is a
// property of the center that went live, not of the process at large.
func (c *Center) OnReady(cb func()) {
	if c.live.Load() {
		cb()
		return
	}
	c.readyMu.Lock()
	if c.live.Load() { // went live while we waited on the lock: fire now
		c.readyMu.Unlock()
		cb()
		return
	}
	c.readyCbs = append(c.readyCbs, cb)
	c.readyMu.Unlock()
}

// adopt applies one config from whatever source: it fans resilience and
// selection out via the module authorities AND swaps the fault injector's
// config. It is the single sink for ALL source pushes, which is what keeps the
// fault chain source-agnostic — fault.Config always rides inside [Config], so any
// Source drives it for free. A push is a fleet-wide policy change, so it always
// leaves an Info line: the sources log only their own failures, not what actually
// took effect.
//
// A push that names an unusable driver is not fatal — unlike at startup, the
// process is already serving, so the push is adopted and the affected services
// fall back to a pass-through, with the error logged.
func (c *Center) adopt(ctx context.Context, cfg Config) {
	if err := c.dispatch(cfg); err != nil {
		log.Warn(ctx, log.TagAppDef,
			log.Err(err),
			log.Msg("governance: policy applied with an unusable driver; affected services fall back to pass-through"))
	}
	log.Info(ctx, log.TagAppDef,
		log.Bool("enabled", cfg.Enabled),
		log.String("driver", cfg.Driver),
		log.Int("client_rules", len(cfg.Client.Rules)),
		log.Int("server_rules", len(cfg.Server.Rules)),
		log.Bool("client_fault", cfg.Client.Fault.Enabled),
		log.Bool("server_fault", cfg.Server.Fault.Enabled),
		log.Msg("governance: policy applied"))
}

// dispatch stores cfg and hands each module its own slice of the document. A
// document whose Rules repeat a service label is rejected before the store —
// the previous snapshot keeps serving — because a duplicate makes which entry
// wins a matter of list order, an accident the operator cannot see. Otherwise
// the store happens FIRST, so the resolver closures the modules call observe
// the new document by the time the modules re-evaluate their subscribers.
func (c *Center) dispatch(cfg Config) error {
	if err := validateClientRules(cfg.Client.Rules); err != nil {
		return err
	}
	if err := validateServerRules(cfg.Server.Rules); err != nil {
		return err
	}
	c.cfg.Store(&cfg)
	err := c.res.Apply(resilience.Settings{
		Enabled:             cfg.Enabled,
		Driver:              cfg.Driver,
		ResolveClientPolicy: c.clientPolicyFor,
		ResolveServerPolicy: c.serverPolicyFor,
	})
	c.lb.Apply(loadbalance.Settings{
		Enabled: cfg.Enabled,
		Resolve: c.clientSelectionFor,
	})
	c.inj.SetConfig(fault.Configs{
		Client: cfg.Client.Fault,
		Server: cfg.Server.Fault,
	})
	return err
}

// bindSource makes s the active source: install its handle, subscribe with a
// stale guard (callbacks from a previously bound source no-op), then adopt s's
// snapshot. The explicit snapshot adopt matches the [Source] contract that
// Subscribe delivers changes only — Snapshot seeds the present.
func (c *Center) bindSource(s Source) {
	h := &sourceHandle{src: s}
	c.srcMu.Lock()
	c.src = h
	c.srcMu.Unlock()
	s.Subscribe(func(ctx context.Context, cfg Config) {
		if !c.isActive(h) {
			return // replaced source: drop
		}
		c.adopt(ctx, cfg)
	})
	// The priming adopt is the bind, not a change: it has no change context to
	// carry, and its line names what it applied on its own.
	c.adopt(context.Background(), s.Snapshot())
}

// isActive reports whether h is still the bound source handle.
func (c *Center) isActive(h *sourceHandle) bool {
	c.srcMu.Lock()
	defer c.srcMu.Unlock()
	return c.src == h
}

// SetSource binds s as the active source, replacing whatever was bound — the
// source the constructor took, or an earlier SetSource. s.Snapshot() applies
// immediately and later pushes drive the center, while the replaced source's
// callbacks go stale via the handle guard. Binding eagerly (rather than
// remembering a pending value) is what lets the standalone path — a hand-built
// center, a test — work with nothing but this call.
func (c *Center) SetSource(s Source) {
	if s == nil {
		panic(errutil.Explain(nil, "governance: SetSource(nil)"))
	}
	c.bindSource(s)
}

// Close closes the active source when it happens to be closeable (the [Source]
// contract keeps Close optional — see source.go), else it is a no-op: the center
// itself holds only an atomic snapshot. It is the shutdown counterpart to
// [Center.GoLive], called by the wiring starter on app teardown.
func (c *Center) Close() error {
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

// Enabled reports whether the center is switched on. When false, [Center.
// clientPolicyFor] returns a transparent pass-through policy and every module stays
// disarmed. It mirrors [resilience.Manager.Enabled], which is what a client
// normally guards an optional governance path with — this one exists for the
// wiring and for tests, which hold the center rather than a module authority.
func (c *Center) Enabled() bool {
	if cfg := c.cfg.Load(); cfg != nil {
		return cfg.Enabled
	}
	return false
}

// clientServiceFor resolves label's two OUTBOUND halves together: the first Rule in the
// client block whose Service equals the label, otherwise the client Default.
// Resolving them in one place is what guarantees the resilience and loadbalance
// modules answer to the same label, and it keeps the rule walk to one pass. A
// disabled center resolves zero halves, which each module already reads as "no
// configuration" — a pass-through executor, an untouched pool.
//
// The read is lock-free (one atomic pointer load), so a push is visible to the
// next resolve without re-registering anything.
func (c *Center) clientServiceFor(label string) (resilience.ClientPolicy, loadbalance.Selection) {
	cfg := c.cfg.Load()
	if cfg == nil || !cfg.Enabled {
		return resilience.ClientPolicy{}, loadbalance.Selection{}
	}
	for _, r := range cfg.Client.Rules {
		if r.Service == label {
			return r.ClientPolicy, r.Selection
		}
	}
	return cfg.Client.Default.ClientPolicy, cfg.Client.Default.Selection
}

// serverPolicyFor resolves label's inbound inbound model, which the resilience
// module builds the route's inbound executor from. It is the server-side
// counterpart of [Center.clientServiceFor] and reads the OTHER block of the document:
// the client's Policy and the server's ServerPolicy are separate resolutions over
// separate rules, so a label configured on one side has no bearing on the other.
//
// A disabled center resolves a zero ServerPolicy, which the module already reads as
// "no configuration" — a pass-through executor, so the middleware chain stays
// installed but never rejects.
func (c *Center) serverPolicyFor(label string) resilience.ServerPolicy {
	cfg := c.cfg.Load()
	if cfg == nil || !cfg.Enabled {
		return resilience.ServerPolicy{}
	}
	for _, r := range cfg.Server.Rules {
		if r.Service == label {
			return r.ServerPolicy
		}
	}
	return cfg.Server.Default
}

// clientPolicyFor resolves label's outbound protection policy, which the resilience
// module builds its executor from.
func (c *Center) clientPolicyFor(label string) resilience.ClientPolicy {
	p, _ := c.clientServiceFor(label)
	return p
}

// clientSelectionFor resolves label's endpoint-selection policy, which the loadbalance
// module drives its pools with.
func (c *Center) clientSelectionFor(label string) loadbalance.Selection {
	_, s := c.clientServiceFor(label)
	return s
}
