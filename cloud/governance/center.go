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
	"sync/atomic"

	"go-spring.org/cloud/chain"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
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
func NewCenter(res *resilience.Manager, lb *loadbalance.Manager,
	inj *fault.Injector, disc *discovery.Manager, src Source) (*Center, error) {
	c := &Center{res: res, lb: lb, inj: inj, disc: disc, src: src}
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
func NewProcessCenter(res *resilience.Manager, inj *fault.Injector) (*Center, error) {
	return NewCenter(res, loadbalance.NewManager(), inj, nil, NewPushSource(Config{}))
}

// Resilience returns the resilience authority the center distributes to. The
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
func (c *Center) Resilience() *resilience.Manager {
	if c == nil {
		return nil
	}
	return c.res
}

// Fault returns the fault-injection authority; see [Center.Resilience].
func (c *Center) Fault() *fault.Injector {
	if c == nil {
		return nil
	}
	return c.inj
}

// Loadbalance returns the endpoint-selection authority; see [Center.Resilience].
func (c *Center) Loadbalance() *loadbalance.Manager {
	if c == nil {
		return nil
	}
	return c.lb
}

// Discovery returns the directory of named discovery backends; see
// [Center.Resilience]. It is not a policy authority like the other three — the
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
	exec := observability.WrapClientExecutor(c.resilienceOrUnarmed().ClientExecutorFor(system, label), system, label)
	if c == nil || c.inj == nil {
		// A nil center is the standalone case a caller normalized away: there is no
		// governance, so there is no fault layer either — but observation is
		// unconditional, so the call is still traced and measured.
		return exec
	}
	return fault.WrapClientExecutor(exec, label, c.inj)
}

// ServerExecutorFor is [Center.ExecutorFor] for the inbound direction: the
// inbound executor an endpoint of the given system, labelled route, runs its
// requests under, wrapped for observation. There is no fault layer inbound (fault
// injection is outbound only), so the stack is observe(resilience-inbound).
//
// It exists so an inbound middleware reaches the same one composition point the
// outbound clients do, instead of wiring the observe wrapper itself; the
// resilience authority's [resilience.Manager.ServerExecutorFor] returns the bare
// handle, which this wraps.
func (c *Center) ServerExecutorFor(system, label string) chain.Executor {
	return observability.WrapServerExecutor(c.resilienceOrUnarmed().ServerExecutorFor(system, label), system, label)
}

// resilienceOrUnarmed returns the resilience authority, or a fresh unarmed one
// when the center (or its authority) is absent. It normalizes the standalone case
// the same way the module accessors do: a transparent pass-through executor keeps
// the caller's code shape uniform, and the observe wrapper above still applies.
func (c *Center) resilienceOrUnarmed() *resilience.Manager {
	if c == nil || c.res == nil {
		return resilience.NewManager(nil)
	}
	return c.res
}

// PolicyFor returns the policy governing the labelled service.
//
// A driver needs it for the one policy field that cannot reach a running
// executor: [resilience.ClientPolicy.MaxConns] sizes the connection pool, and a
// pool is built once, when the client is constructed. Everything else the policy
// carries is adopted by the executor on refresh and needs no lookup here.
func (c *Center) PolicyFor(label string) resilience.ClientPolicy {
	return c.res.ClientPolicyFor(label)
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
	c.res.Apply(resilienceSettings(cfg))
	c.lb.Apply(ctx, selectionSettings(cfg))
	c.inj.SetConfig(fault.Configs{
		Client: cfg.Client.Fault,
		Server: cfg.Server.Fault,
	})
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
		Server: resilience.ServerSettings{
			Default: cfg.Server.Default,
			Rules:   resilienceServerRules(cfg.Server.Rules),
		},
	}
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
		out[i] = resilience.ClientRule{Service: r.Service, ClientPolicy: r.ClientPolicy}
	}
	return out
}

// resilienceServerRules is [resilienceClientRules] for the inbound block, which
// carries no selection half.
func resilienceServerRules(in []ServerRule) []resilience.ServerRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]resilience.ServerRule, len(in))
	for i, r := range in {
		out[i] = resilience.ServerRule{Service: r.Service, ServerPolicy: r.ServerPolicy}
	}
	return out
}

// selectionRules takes the endpoint-selection half of the outbound rules.
func selectionRules(in []ClientRule) []loadbalance.Rule {
	if len(in) == 0 {
		return nil
	}
	out := make([]loadbalance.Rule, len(in))
	for i, r := range in {
		out[i] = loadbalance.Rule{Service: r.Service, Selection: r.Selection}
	}
	return out
}

// Close closes the bound source when it happens to be closeable (the [Source]
// contract keeps Close optional — see source.go), else it is a no-op: the center
// itself holds only an atomic snapshot. It is the shutdown counterpart to the
// constructor's arming dispatch, called by the wiring starter on app teardown.
func (c *Center) Close() error {
	if cl, ok := c.src.(interface{ Close() error }); ok {
		return cl.Close()
	}
	return nil
}
