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
// The division of labour is deliberate. This package owns the DOCUMENT: the rule
// list, the source handling, the duplicate-label rejection. The module packages
// own the label MATCHING and what to DO with a matched policy: the executor
// cache, the driver directory, the pool subscriptions, the injector state. A
// module never imports this package — the center PUSHES each one its own data
// (its own rule type plus a default, see [Center.dispatch]) and the module
// matches labels itself. Every authority is driven the same way, fault included;
// the three translators in dispatch are the only place that knows both
// vocabularies. That is what keeps a module usable — and testable — without a
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
// needs ([Center.ResilienceClient], [Center.Loadbalance], [Center.FaultClient], [Center.FaultServer],
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
	"fmt"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/guard"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
)

// CenterOption customizes the authorities a [Center] composes.
type CenterOption func(*centerOptions)

type centerOptions struct {
	limiter guard.ClusterLimiter
}

// WithClusterLimiter makes the inbound guard charge a shared rate-limit store
// instead of its per-instance budget. It is the inbound counterpart of the
// limiter the resilience Driver receives.
func WithClusterLimiter(l guard.ClusterLimiter) CenterOption {
	return func(o *centerOptions) { o.limiter = l }
}

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

	// res, lb, injClient, injServer and disc are the module authorities this
	// center distributes to. They are injected rather than constructed here,
	// because each is registered as a bean by the package that owns it, and the
	// starter must hand the same instances on.
	//
	// The two fault directions are held as TWO independent authorities, exactly
	// as fault exposes them: the center hands each one out unchanged and pushes
	// only that direction's document. It deliberately does not pair them into a
	// third type — the client and server seams are free to evolve apart.
	resClient   *resilience.ClientManager
	guardServer *guard.Manager
	lb          *loadbalance.Manager
	injClient   *fault.ClientManager
	injServer   *fault.ServerManager
	disc        *discovery.Manager

	// src is the bound [Source], set once at construction (the container's
	// contribution) and never replaced — Close reads it on teardown. With no
	// replacement there is no stale-callback race to guard.
	src Source
}

// NewCenter builds a Center over the module authorities and arms it in the same
// breath: the source's snapshot is dispatched into the module authorities right
// here, and a document that cannot serve (a duplicate label, an unusable driver)
// fails the constructor, which is where the wiring can fail startup.
//
// src is bound for life, exactly as the authorities arrive as constructor
// parameters, and it is REQUIRED: the container ALWAYS passes one — the bean's
// constructor declares src required, so a process with no Source bean fails
// startup rather than running with governance silently off. A hand-built center
// (an embedder, a test) passes one too: a caller that wants to hand over a single
// fixed document and never see a push wraps it in a [PushSource], so the initial
// document always comes from src.Snapshot() — there is no second entry point.
//
// The authorities are owned by the caller — the wiring starter, or a test — so
// the same instances can also be registered as beans for clients to inject.
func NewCenter(resClient *resilience.ClientManager,
	lb *loadbalance.Manager, injClient *fault.ClientManager, injServer *fault.ServerManager,
	disc *discovery.Manager, src Source, opts ...CenterOption) (*Center, error) {
	co := centerOptions{}
	for _, o := range opts {
		o(&co)
	}
	var guardOpts []guard.Option
	if co.limiter != nil {
		guardOpts = append(guardOpts, guard.WithClusterLimiter(co.limiter))
	}
	c := &Center{
		resClient:   resClient,
		guardServer: guard.NewManager(guardOpts...),
		lb:          lb,
		injClient:   injClient, injServer: injServer,
		disc: disc, src: src,
	}
	src.Subscribe(func(ctx context.Context, cfg Config) {
		c.adopt(ctx, cfg)
	})
	initial := src.Snapshot()
	if err := c.dispatch(context.Background(), initial); err != nil {
		return nil, err
	}
	logApplied(context.Background(), initial)
	return c, nil
}

// NewProcessCenter builds a Center for a process that has no governance document
// — an embedder or a test that wants the resilience and fault authorities behind
// the one injection point a client already takes, without a rule list to serve.
// The load-balance authority is a fresh empty manager and the discovery directory
// is absent; callers that route by service name must join a [Center] built over a
// real directory instead.
//
// It is [NewCenter] over an empty [PushSource]: the authorities are armed against
// an empty document, so nothing is governed and every label falls through to its
// default — exactly the behaviour of a process configured with no rules.
func NewProcessCenter(resClient *resilience.ClientManager,
	injClient *fault.ClientManager, injServer *fault.ServerManager, opts ...CenterOption) (*Center, error) {
	return NewCenter(resClient, loadbalance.NewManager(), injClient, injServer, nil, NewPushSource(Config{}), opts...)
}

// ResilienceClient returns the OUTBOUND resilience authority the center distributes to. The
// Center is the governance family's sole spokesperson: a client starter injects
// the ONE *Center bean and reads whichever authority it needs from here,
// instead of wiring each authority bean in separately. The bean is registered
// by the cloud/governance package itself and always instantiated (it exports as
// a gs.Rooter), so a successfully started app never holds a nil Center.
//
// A nil *Center reads as nil authorities, exactly as a zero construction bundle
// did before the center was the one injection point: a caller that never routes
// through an authority (a hand-built client, a test) may pass nil and only the
// authority it actually reaches for must be present. The composition helpers
// ([Center.ExecutorFor] / [Center.PolicyFor]) still require a resilience manager
// and panic without one, since a client is never assembled unprotected.
func (c *Center) ResilienceClient() *resilience.ClientManager {
	if c == nil {
		return nil
	}
	return c.resClient
}

// GuardServer returns the INBOUND guard authority — the manager that creates one
// [guard.Guard] per server instance and pushes each the server policy. It is the
// object a server adapter (or a filter library with no bean to inject into)
// passes to [InboundOver].
func (c *Center) GuardServer() *guard.Manager {
	if c == nil {
		return nil
	}
	return c.guardServer
}

// FaultClient returns the OUTBOUND fault authority; see [Center.ResilienceClient].
// It is the object client starters wrap their executors with. A server starter
// has no use for it, and vice versa.
func (c *Center) FaultClient() *fault.ClientManager {
	if c == nil {
		return nil
	}
	return c.injClient
}

// FaultServer returns the INBOUND fault authority; see [Center.ResilienceClient].
// It is the object server adapters create their shell from.
func (c *Center) FaultServer() *fault.ServerManager {
	if c == nil {
		return nil
	}
	return c.injServer
}

// Loadbalance returns the endpoint-selection authority; see [Center.ResilienceClient].
func (c *Center) Loadbalance() *loadbalance.Manager {
	if c == nil {
		return nil
	}
	return c.lb
}

// Discovery returns the directory of named discovery backends; see
// [Center.ResilienceClient]. It is not a policy authority like the other three — the
// center holds it so a client reaches discovery through the same injection point
// it reaches everything else.
func (c *Center) Discovery() *discovery.Manager {
	if c == nil {
		return nil
	}
	return c.disc
}

// ExecutorFor returns the executor a client of the given system, labelled
// service, runs its calls on. It is the ONE place the observe layer and the
// governance layer are composed — the center names observability, the resilience
// authority names neither it nor the center, so the two packages stay
// independent and this composition is all that ties them. It is the composition a
// client's constructor needs in one call, so no driver repeats it, on the one
// object a client already injects.
//
// The layers are stacked fault(observe(resilience-executor)): the observer wraps
// the resilience manager's handle (so a rejected or failed call is still traced,
// measured and logged), and fault injection wraps that. The observe wrapper
// reaches the breaker through the handle's [resilience.BreakerEventListenerSetter],
// so breaker transitions still emit.
//
// "system" is the client's backend name ("memcached", "redigo", ...), which
// clients also declare in their operations; it is a parameter rather than state
// because one center serves every kind of client.
func (c *Center) ExecutorFor(system, label string) chain.Executor {
	exec := observability.WrapClientExecutor(c.resClient.ClientExecutorFor(system, label), system, label)
	if c.injClient == nil {
		// A nil center is the standalone case a caller normalized away: there is no
		// governance, so there is no fault layer either — but observation is
		// unconditional, so the call is still traced and measured.
		return exec
	}
	return c.injClient.WrapClientExecutor(exec)
}

// InboundFor returns the pair of inbound objects one route's middleware drives,
// wired together: the guard's breaker transitions are emitted through the
// observer. It replaces the old ServerExecutorFor — inbound is no longer a chain
// (a governed request has two hook points a chain cannot span), so what a server
// gets is two peer objects, not one executor.
//
// It is the same single composition point the outbound clients reach: the
// guard authority supplies the governance object and this package (the only
// importer of both) supplies the observer, so no middleware wires the observe
// object to the guard itself and no other package needs to know both.
func (c *Center) InboundFor(system, label string) *Inbound {
	return InboundOver(system, label, c.guardServer, c.injServer)
}

// InboundOver builds the inbound pair over an explicit guard authority — the
// seam for a caller that holds the manager rather than a center (a filter library
// with no bean to inject into, a test). mgr MUST be non-nil: an invalid
// construction is refused, never silently wrapped.
//
// inj is optional: when present, the inbound fault stage becomes part of the
// guard (innermost, after the breaker gate), so ordering and breaker accounting
// are fixed in one place. A caller with no injector gets governance without
// fault, exactly as before.
func InboundOver(system, label string, mgr *guard.Manager, inj ...*fault.ServerManager) *Inbound {
	if mgr == nil {
		panic("governance: InboundOver requires a non-nil guard.Manager")
	}
	var opts []guard.Option
	if len(inj) > 0 && inj[0] != nil {
		opts = append(opts, guard.WithFault(&faultStage{
			system: system,
			srv:    inj[0].NewServer(),
		}))
	}
	obs := observability.NewServerObserver(system, label)
	g := mgr.GuardFor(guard.Identity{System: system, Service: label}, opts...)
	if s, ok := g.(guard.BreakerEventListenerSetter); ok {
		s.SetBreakerEventListener(obs)
	}
	return &Inbound{observe: obs, govern: g}
}

// faultStage adapts the fault package's inbound shell to the guard's Fault
// interface. The shell is created once per server instance (not per request) and
// is fed the request's guard-level identity.
type faultStage struct {
	system string
	srv    *fault.Server
}

func (f *faultStage) Fire(ctx context.Context, req guard.Request) (guard.FaultResult, error) {
	d, err := f.srv.Fire(ctx, fault.Target{
		Service:  f.system,
		Action:   req.Action,
		Metadata: req.Metadata,
	})
	return guard.FaultResult{
		Rule:  d.Rule,
		Fired: d.Fired(),
		Kind:  d.Kind,
		Delay: d.Delay,
	}, err
}

// Refusal is a rejected request's error. It carries the guard's neutral verdict
// so an adapter maps the protocol word without importing guard's internals.
type Refusal struct {
	result guard.Result
}

func (r *Refusal) Error() string {
	if err := r.result.Err(); err != nil {
		return err.Error()
	}
	return "guard: " + r.result.Verdict().String()
}

// Unwrap exposes the underlying sentinel for errors.Is.
func (r *Refusal) Unwrap() error { return r.result.Err() }

// Verdict is why the request was refused.
func (r *Refusal) Verdict() guard.Verdict { return r.result.Verdict() }

// RetryAfter is how long the caller is asked to wait, when known.
func (r *Refusal) RetryAfter() time.Duration { return r.result.RetryAfter() }

// Result returns the full guard result.
func (r *Refusal) Result() guard.Result { return r.result }

// Inbound is a middleware's handle on one route's inbound governance and
// observation. It holds NO per-request state and NO domain state — the state
// lives on the [guard.Guard] and on
// [observability.ServerObserver]/[observability.Observation]. Its only job is to
// fix the layer order — observation outermost, admission inside — so an adapter
// cannot get it wrong and a rejected request is still traced.
type Inbound struct {
	observe *observability.ServerObserver
	govern  guard.Guard
}

// Run is the PREFERRED form for a protocol whose handler can be wrapped in one
// closure. It opens the span, admits inside it, and runs fn — the handler, with
// fault innermost — last, then settles. It returns fn's error, or a [Refusal]
// when the request never reached fn.
//
// A handler that must record the late half of its operation (the response status
// code) uses the [Inbound.Before]/[Inbound.After] form instead, whose [Hooks]
// exposes the observation.
func (in *Inbound) Run(ctx context.Context, op observability.ServerOperation, fn func(context.Context) error) error {
	return in.observe.Run(ctx, op, func(ctx context.Context, _ *observability.Observation) error {
		return in.run(ctx, op, fn)
	})
}

// GovernedRun runs the request under governance only, with no observation. It is
// the seam for a server that has turned its own observability off but still
// wants admission control.
func (in *Inbound) GovernedRun(ctx context.Context, op observability.ServerOperation, fn func(context.Context) error) error {
	return in.run(ctx, op, fn)
}

// run is the shared admission + completion path.
func (in *Inbound) run(ctx context.Context, op observability.ServerOperation, fn func(context.Context) error) (err error) {
	r, done := in.govern.Admit(ctx, guard.Request{
		Action:   op.Action,
		Metadata: op.Metadata,
	})
	if !r.Admitted() {
		return &Refusal{result: r}
	}

	ctx, cancel := withBudget(ctx, r.Timeout())
	defer cancel()
	defer func() {
		if p := recover(); p != nil {
			done(guard.OutcomeFailure, fmt.Errorf("guard: handler panicked: %v", p))
			panic(p)
		}
	}()
	err = fn(ctx)
	done(classify(err), err)
	return err
}

// Before is the SPLIT form, for a protocol whose request cannot be wrapped in one
// closure. It opens the span FIRST, then admits — so a rejection is still
// observed — and on rejection closes the span and returns the [Refusal]. On
// admission it returns the [Hooks] to settle with [Inbound.After].
func (in *Inbound) Before(ctx context.Context, op observability.ServerOperation) (*Hooks, error) {
	ctx, ob := in.observe.Begin(ctx, op)
	r, done := in.govern.Admit(ctx, guard.Request{
		Action:   op.Action,
		Metadata: op.Metadata,
	})
	if !r.Admitted() {
		ref := &Refusal{result: r}
		in.observe.End(ob, ref)
		return nil, ref
	}
	runCtx, cancel := withBudget(ctx, r.Timeout())
	return &Hooks{ctx: runCtx, ob: ob, done: done, cancel: cancel}, nil
}

// After settles an attempt begun by [Inbound.Before]: the guard first (so the
// breaker sees the outcome before the span closes), then the observer.
func (in *Inbound) After(h *Hooks, err error) {
	if h == nil {
		return
	}
	h.done(classify(err), err)
	if h.cancel != nil {
		h.cancel()
	}
	in.observe.End(h.ob, err)
}

// Hooks carries one split-form request's tokens: the context the handler runs
// under, the observation and the guard's completion. The state lives on those
// objects, not here.
type Hooks struct {
	ctx    context.Context
	ob     *observability.Observation
	done   guard.Done
	cancel context.CancelFunc
}

// Context is the context the handler should run under — the span and the route's
// handling budget applied.
func (h *Hooks) Context() context.Context { return h.ctx }

// Observation returns the request's observation, for recording the response half.
func (h *Hooks) Observation() *observability.Observation { return h.ob }

// Observe returns the observer alone, for a caller that wants the observation
// signals without governance.
func (in *Inbound) Observe() *observability.ServerObserver { return in.observe }

// withBudget applies the policy's handling budget when one is configured.
func withBudget(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	if d > 0 {
		return context.WithTimeout(ctx, d)
	}
	return ctx, func() {}
}

// classify turns a handler's outcome into the guard's neutral classification.
func classify(err error) guard.Outcome {
	if err != nil {
		return guard.OutcomeFailure
	}
	return guard.OutcomeSuccess
}

// PolicyFor returns the policy governing the labelled service.
//
// A driver needs it for a field the running executor cannot deliver: one baked
// into the client at construction rather than adopted per call, because a pool
// or a transport is built once. Everything else the policy carries is adopted
// by the executor on refresh and needs no lookup here.
//
// No field is construction-bound today, so nothing calls this yet: it is the
// seam such a knob plugs into when one exists.
func (c *Center) PolicyFor(label string) resilience.ClientPolicy {
	return c.resClient.ClientPolicyFor(label)
}

// adopt applies one config from whatever source: it fans resilience and
// selection out via the module authorities AND swaps the fault injector's
// config. It is the single sink for ALL source pushes, which is what keeps the
// fault chain source-agnostic — fault.Config always rides inside [Config], so any
// Source drives it for free.
//
// A push that cannot serve is not fatal — unlike at startup, the process is
// already serving, so the document is rejected and the last good one keeps
// serving in EVERY authority, with the error logged. That is the whole of the
// difference from the constructor: both run the same two-phase [Center.dispatch],
// but a startup failure aborts the process where a push only logs.
//
// The [logApplied] line is printed only on the success path: a rejected document
// took effect nowhere, so claiming "policy applied" would misreport what the
// process is running on.
func (c *Center) adopt(ctx context.Context, cfg Config) {
	if err := c.dispatch(ctx, cfg); err != nil {
		log.Warn(ctx, log.TagAppDef, "governance: document rejected; the last good policy keeps serving",
			log.Err(err))
		return
	}
	logApplied(ctx, cfg)
}

// logApplied is the one Info line marking a document taking effect: what the
// process now runs on, whether it arrived at startup or on a push. It is called
// on the SUCCESS path only, so a rejected document never produces it — the
// sources log only their own failures, not what actually took effect.
func logApplied(ctx context.Context, cfg Config) {
	log.Info(ctx, log.TagAppDef, "governance: policy applied",
		log.Int("client_rules", len(cfg.Client.Rules)),
		log.Int("server_rules", len(cfg.Server.Rules)),
		// Whether a direction's fault fire is armed, derived from the config
		// (there is no enabled flag to read; see fault.Config.Inert).
		log.Bool("client_fault", !cfg.Client.Fault.Inert()),
		log.Bool("server_fault", !cfg.Server.Fault.Inert()))
}

// Validate reports whether cfg could take effect, WITHOUT disturbing anything:
// it is [Center.dispatch]'s first phase, exported for a caller that wants to
// reject a bad document before it reaches the center — a console form checked
// before it is pushed, a rules file vetted before it is handed to a source.
//
// A document fails on any of: a Rule list repeating a service label (a duplicate
// makes which entry wins a matter of list order, an accident the operator cannot
// see), or a Selection naming no registered strategy. Every check runs against
// the authorities this center holds, so the verdict cannot drift from what
// dispatch would do.
//
// It reads only immutable state — the authorities' registries and cfg itself — so
// it takes no lock and is safe to call concurrently, even while a push is being
// dispatched.
func (c *Center) Validate(cfg Config) error {
	if err := validateClientRules(cfg.Client.Rules); err != nil {
		return err
	}
	if err := validateServerRules(cfg.Server.Rules); err != nil {
		return err
	}
	return loadbalance.Validate(selectionSettings(cfg))
}

// dispatch applies cfg in two phases, so that a document either takes effect
// WHOLE or not at all: phase 1 asks [Center.Validate] whether it can serve, and
// only then does phase 2 store the snapshot and hand each module its own slice.
// Nothing in phase 2 can fail, so the snapshot and all three authorities always
// move together — the previous document keeps serving, in every authority, when
// a push is rejected.
//
// Each module receives DATA, not a way to ask: the rules are translated into the
// module's own types here and pushed whole, after which the module owns the
// label matching. That is the same shape fault already had, so all three
// authorities are driven identically — the center owns the document, the modules
// own what to do with it. The translators below are the only place that knows
// both vocabularies.
//
// ctx is threaded to the module authorities so a rejected push logs under the
// source's trace context.
func (c *Center) dispatch(ctx context.Context, cfg Config) error {
	// Phase 1 — validate. A document that cannot serve is rejected before any
	// authority is touched.
	if err := c.Validate(cfg); err != nil {
		return err
	}

	// Phase 2 — commit. Nothing here can fail: the document carries policies only
	// (the backend is a startup choice), so Apply's error is always nil and is
	// discarded on purpose rather than re-checked.
	c.cfg.Store(&cfg)
	settings := resilienceSettings(cfg)
	c.resClient.Apply(settings.Client)
	c.guardServer.Apply(guardSettings(cfg))
	c.lb.Apply(ctx, selectionSettings(cfg))
	c.injClient.Refresh(cfg.Client.Fault)
	c.injServer.Refresh(cfg.Server.Fault)
	return nil
}

// resilienceSettings translates the document into the resilience authority's own
// settings. Both the validity check and the commit build from it, so a document
// cannot be validated against one shape and applied as another.
func resilienceSettings(cfg Config) resilience.Settings {
	return resilience.Settings{
		Client: resilience.ClientSettings{
			Default: cfg.Client.Default.ClientPolicy,
			Rules:   resilienceClientRules(cfg.Client.Rules),
		},
	}
}

// guardSettings is the document's SERVER half in the guard authority's own
// settings. The document and the guard policy share one type, so this is a
// shape-preserving conversion.
func guardSettings(cfg Config) guard.Settings {
	s := guard.Settings{Default: cfg.Server.Default}
	for _, r := range cfg.Server.Rules {
		s.Rules = append(s.Rules, guard.Rule{
			Service: r.Service,
			Policy:  r.Policy,
		})
	}
	return s
}

// selectionSettings is [resilienceSettings] for the endpoint-selection authority:
// the selection half of the outbound rules, which the same source entry carries
// alongside the protection half.
func selectionSettings(cfg Config) loadbalance.Settings {
	return loadbalance.Settings{
		Default: cfg.Client.Default.Selection,
		Rules:   selectionRules(cfg.Client.Rules),
	}
}

// resilienceClientRules splits the outbound rules into the resilience half: the
// protection knobs of each [ClientRule]. The same source entry also carries the
// endpoint-selection half, which selectionRules takes — a governance ClientRule is
// ONE service's configuration consumed by TWO modules.
func resilienceClientRules(in []ClientRule) []resilience.ClientRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]resilience.ClientRule, len(in))
	for i, r := range in {
		out[i] = resilience.ClientRule{Service: r.Service, Action: r.Action, Match: r.Match, ClientPolicy: r.ClientPolicy}
	}
	return out
}

// selectionRules takes the endpoint-selection half of the outbound rules.
func selectionRules(in []ClientRule) []loadbalance.Rule {
	if len(in) == 0 {
		return nil
	}
	// Endpoint selection is resolved per CONNECTION at wiring time, not per call,
	// so it cannot honour a per-call Match — a service is balanced one way however
	// its calls are governed. Keep the FIRST rule per service and drop the rest, so
	// relaxing the client-rule uniqueness check (which Match made necessary) leaves
	// this authority with the unique labels it requires.
	out := make([]loadbalance.Rule, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, r := range in {
		if r.Service == "" {
			continue
		}
		if _, dup := seen[r.Service]; dup {
			continue
		}
		seen[r.Service] = struct{}{}
		out = append(out, loadbalance.Rule{Service: r.Service, Selection: r.Selection})
	}
	return out
}

// Close closes the bound source when it happens to be closeable (the [Source]
// contract keeps Close optional — see source.go), else it is a no-op: the center
// itself holds only an atomic snapshot. It is the shutdown counterpart to the
// constructor's arming dispatch, called by the wiring starter on app teardown.
func (c *Center) Close() error {
	if c.guardServer != nil {
		_ = c.guardServer.Close()
	}
	if cl, ok := c.src.(interface{ Close() error }); ok {
		return cl.Close()
	}
	return nil
}
