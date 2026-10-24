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

// Package governance is the service-governance CONTROL PLANE: it owns ONE
// refreshable [Config] document and ONE source, and distributes them to the
// module authorities that do the actual work — resilience, endpoint selection
// and fault injection.
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
// beans, the injected [Source], the injected module managers) lives in the
// runtime root. Reference/config awareness is contract-first: the center
// consumes its own [Source] — a snapshot plus a change subscription — and
// nothing else, so any pusher (a governance console stream, a config-center
// listener, a static injection) can drive the whole family by implementing two
// methods.
//
// It is a PLANE, not the process's injection point. A client or server starter
// injects [go-spring.org/cloud/runtime.Runtime], the composition root that owns
// this center and every authority it drives; the center exposes none of them.
// There is no process-wide facade and no global state.
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

	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/runtime/dataplane/inbound/fault"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	clientfault "go-spring.org/cloud/runtime/dataplane/outbound/fault"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
	"go-spring.org/log"
)

// Center is the governance CONTROL PLANE. It holds an atomic snapshot of the
// current [Config] and, on each adopt, distributes it to the module authorities
// — one call fans out to every service, which is the whole point of
// centralizing.
//
// It owns the DOCUMENT, not the request path. It holds the authorities only to
// push each its own slice; it exposes none of them and executes nothing. The
// process's single injection point — the object a client or server starter
// injects — is [go-spring.org/cloud/runtime.Runtime], the composition root that
// owns this center and every authority it drives.
//
// It is container-free: it knows sources only through the [Source] contract and
// modules only through the authorities handed to it, so it imports no IoC
// concept. The gs wiring (the beans, binding the injected source, arming the
// authorities) lives in the runtime root, which builds one [Center] over the
// module beans it also exports for clients to inject.
//
// Safe for concurrent use.
type Center struct {
	cfg atomic.Pointer[Config]

	// res, guardServer, lb, injClient and injServer are the POLICY authorities
	// this center distributes to. They are injected rather than constructed here:
	// the runtime composition root owns them and hands the same instances on,
	// which is what routes a push to the clients and servers that follow it. This
	// center never exposes them.
	//
	// Discovery is deliberately absent: no rule applies to it, so the control
	// plane has nothing to push. The runtime owns the directory and hands it out.
	//
	// The two fault directions are held as TWO independent authorities, exactly
	// as fault exposes them: the center pushes only that direction's document. It
	// deliberately does not pair them into a third type — the client and server
	// seams are free to evolve apart.
	resClient   *resilience.Manager
	guardServer *admission.Manager
	lb          *loadbalance.Manager
	injClient   *clientfault.ClientManager
	injServer   *serverfault.ServerManager

	// src is the bound [Source], set once at construction (the container's
	// contribution) and never replaced — Close reads it on teardown. With no
	// replacement there is no stale-callback race to admission.
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
func NewCenter(resClient *resilience.Manager, guardServer *admission.Manager,
	lb *loadbalance.Manager, injClient *clientfault.ClientManager, injServer *serverfault.ServerManager,
	src Source) (*Center, error) {
	c := &Center{
		resClient:   resClient,
		guardServer: guardServer,
		lb:          lb,
		injClient:   injClient, injServer: injServer,
		src: src,
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
// see), a server policy that cannot be built (a negative max-concurrent), or a
// Selection naming no registered strategy. Every check runs against the
// authorities this center holds, so the verdict cannot drift from what dispatch
// would do.
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
	if err := admission.ValidateSettings(guardSettings(cfg)); err != nil {
		return err
	}
	if err := resilience.ValidateSettings(resilienceSettings(cfg)); err != nil {
		return err
	}
	return loadbalance.Validate(selectionSettings(cfg))
}

// dispatch applies cfg in two phases, so that a document either takes effect
// WHOLE or not at all: phase 1 asks [Center.Validate] whether it can serve, and
// phase 2 hands the inbound authority its slice first — it is the one that can
// still reject a policy it cannot build, and it rejects before the snapshot or
// any other authority moves. Nothing after it can fail, so the snapshot and all
// three authorities always move together — the previous document keeps serving,
// in every authority, when a push is rejected.
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

	// Phase 2 — commit. Both policy authorities re-check the document and can
	// still reject it (an unknown breaker strategy, a threshold the strategy does
	// not read, ...); they run before the snapshot or the other authorities
	// move, so a rejected document leaves the previous one serving everywhere.
	if err := c.guardServer.Apply(guardSettings(cfg)); err != nil {
		return err
	}
	if err := c.resClient.Apply(resilienceSettings(cfg)); err != nil {
		return err
	}
	c.cfg.Store(&cfg)
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
		Default: cfg.Client.Default.Policy,
		Rules:   resilienceClientRules(cfg.Client.Rules),
	}
}

// guardSettings is the document's SERVER half in the guard authority's own
// settings. The document and the guard policy share one type, so this is a
// shape-preserving conversion.
func guardSettings(cfg Config) admission.Settings {
	s := admission.Settings{Default: cfg.Server.Default}
	for _, r := range cfg.Server.Rules {
		s.Rules = append(s.Rules, admission.Rule{
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
func resilienceClientRules(in []ClientRule) []resilience.Rule {
	if len(in) == 0 {
		return nil
	}
	out := make([]resilience.Rule, len(in))
	for i, r := range in {
		out[i] = resilience.Rule{Service: r.Service, Action: r.Action, Match: r.Match, Policy: r.Policy}
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
// constructor's arming dispatch, called by [go-spring.org/cloud/runtime.Runtime]
// on app teardown. The authorities are owned by the runtime and have their own
// teardown; this closes only the document's source.
func (c *Center) Close() error {
	if cl, ok := c.src.(interface{ Close() error }); ok {
		return cl.Close()
	}
	return nil
}
