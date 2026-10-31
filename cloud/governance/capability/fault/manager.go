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

// Package fault is the direction-neutral kernel of in-process fault injection.
// It owns the vocabulary and the one injection sequence both directions run:
// the drill document ([Document]) and its per-direction rule lists
// ([Config]/[Rule]), the call identity a rule matches ([Target]), what one
// attempt decided ([Result]), and the guardrail state model behind a document
// push.
//
// The direction-specific adapters live in the direction trees, not here:
// [go-spring.org/cloud/governance/dataplane/outbound/fault] decides one outbound
// attempt so an injected fault lands INSIDE the resilience executor's retry loop
// (the wrapped fn returns the injected error and the real executor
// retries/breaks around it), which is the whole point;
// [go-spring.org/cloud/governance/dataplane/inbound/fault] decides an inbound call,
// leaving the handler to the adapter. Each owns its own document and kernel, so
// burning inbound requests leaves outbound calls untouched.
//
// What a rule matches is this package's OWN shape, [Target] — not
// chain.Operation (see Target's doc for why). This package depends on nothing
// of cloud's call vocabulary: traffic classification reaches a rule as one more
// [Target.Metadata] entry, recorded by the seam that builds the operation, and
// is matched by the same subset rule as every other targeting dimension.
//
// State follows the family's stable-shell / replaceable-core model (the same
// shape resilience.Manager and loadbalance.Manager use): the [Manager] is the
// stable shell, and the [core] it publishes is what a push replaces. Each
// direction's manager owns one atomic kernel pointer; [Manager.Fire] loads it
// per attempt, so the call path synchronizes with a push only through that one
// atomic load. A refresh RECONCILES
// rather than replaces: it reuses the counters of rules whose identity is
// unchanged, gives new rules fresh ones, and drops the state of rules that are
// gone. Presence in the pushed document is therefore a rule's lifecycle —
// appearing starts it, disappearing ends and releases it — which is why there is
// no separate start/stop call.
//
// The files split by role: manager.go holds the RUNTIME (the manager seam, the
// call/result vocabulary, the build/reconcile logic and the one injection
// sequence), rule.go the CONFIGURATION vocabulary and the two computations a
// rule owns (selection and dose draw), and error.go the outcome vocabulary.

package fault

import (
	"context"
	"sync/atomic"
	"time"

	"go-spring.org/log"
	"go-spring.org/stdlib/timeutil"
)

// Manager is one traffic direction's fault authority AND the entry point its
// adapters decide through. It owns the current document, compiles it into a
// kernel, publishes each new kernel atomically, and evaluates one attempt
// against whichever kernel is current when [Manager.Fire] is called.
//
// One Manager exists per direction: outbound/fault and inbound/fault embed one
// each and expose it under their own authority type, so a client starter can
// only reach the outbound object and a server starter only the inbound one. An
// outbound fire and an inbound fire read different kernels and never trip each
// other.
//
// The kernel pointer is the only shared state. Refresh is UNSYNCHRONIZED, like
// [go-spring.org/cloud/governance/controlplane/source.PushSource]: pushes arrive
// from one serialized source loop (initial snapshot in the constructor, the
// subscribe callback afterwards), so seriality is the control plane's contract,
// not this type's lock. [Manager.Fire] loads the pointer and evaluates against
// that one generation, which is what keeps the call path independent of a push
// landing mid-call.
type Manager struct {
	core atomic.Pointer[core]
}

// NewManager returns an unarmed authority: it carries no rules, so it injects
// nothing until a document is pushed.
//
// It holds no context or protocol vocabulary: the traffic class is one more
// entry in [Target.Metadata], recorded by the seam that builds the operation.
func NewManager() *Manager {
	return &Manager{}
}

// Refresh adopts c as this direction's document, with the reconciliation
// guarantees described on [buildCore], then publishes the new kernel. Callers
// must serialize pushes (the center's source loop already does); an attempt
// reads the kernel when it starts, so a toggle takes effect on the next attempt.
//
// c is ALREADY validated: every expr tag runs where the document enters —
// conf.Bind calls it for every bound field, and a source rejects a document that
// fails it before pushing. This boundary therefore commits what it is given; it
// does not judge it. Callers that construct a Config in code must validate it
// themselves before committing.
func (m *Manager) Refresh(c Config) {
	core := buildCore(c, m.core.Load())
	m.core.Store(core)
}

// Config returns the document this authority currently runs — the cfg inside
// the published kernel, so there is exactly one copy of the document (the zero
// Config when nothing has been pushed, which is a no-op).
func (m *Manager) Config() Config {
	if k := m.core.Load(); k != nil {
		return k.cfg
	}
	return Config{}
}

// Target is one call's identity AS A FAULT RULE SEES IT: what was called, which
// interface of it, and the caller's own targeting metadata. A rule is matched
// against these three, so an adapter that can name them can be put on fire —
// which is what lets the same rule vocabulary serve an outbound client call and
// an inbound request (see outbound/fault's Manager.WrapClientExecutor and
// inbound/fault's Server.Fire).
//
// It deliberately carries no instance identity: which replica receives a
// document is decided when the document is DELIVERED, not when a call is matched
// (see [Rule]'s doc for why that axis must not return).
//
// It is fault's OWN shape, deliberately not [chain.Operation]: that type is a
// CLIENT-shaped declaration of a call — it carries the trace/metric/log
// declarations and the client-edge kind, none of which a fault rule reads — and
// an inbound request has no meaningful Operation to offer. Passing one was how
// the server seam silently lost its Action and its Metadata (every inbound rule
// degraded to a whole-service match); a Target states only what is matched, so
// a caller cannot omit by accident what a rule needs.
//
// It is NOT configuration: nothing here is written by an operator or bound from
// a {value} tag (that is rule.go's vocabulary). An adapter builds it at call
// time from what its protocol knows, which is the only side that does.
type Target struct {
	// Service is what the call is addressed to — the identity a rule's Service
	// matches by exact compare, and a governance label the operator writes rules
	// against: a client names the downstream ("redis:cache", "http:user-svc"), a
	// server names its own entrance ("gin", "grpc"). Its shape is the starter's to
	// choose, as it is on [chain.Operation.Service]; it must be non-empty, since
	// every rule names a service and none can match an empty one.
	Service string

	// Action is the interface being invoked, in the protocol's own format — the
	// second filter after Service, and the one that makes INTERFACE-level fault
	// injection possible: fire one endpoint, one method, instead of a whole
	// service. Empty means "no interface named", which only a rule that names no
	// Action matches.
	//
	// The format is deliberately not fixed, because "the action" differs by
	// protocol: an RPC names its method ("/pkg.Svc/Method", trpc's callee method),
	// an HTTP server names METHOD + path ("GET /users/{id}"), exactly as
	// [chain.Request.Action] does — the string an interface-precise rule needs.
	Action string

	// Metadata is the call's key-value targeting set — the third filter, and the
	// one that separates two calls to the SAME service: a rule's Match must be a
	// subset of this map (see [Rule.MetadataMatch]). It is filled by the seam that
	// builds the operation — the base constructors record the call's traffic
	// class under the ordinary key "traffic" — and is matched, never emitted, so
	// its keys are whatever dimensions the deployment cares about ("tenant",
	// "region", "canary").
	//
	// Nil (the common case) means the call declares nothing, and only rules keyed
	// on service and action alone can match it.
	Metadata map[string]string
}

// Result is what one attempt drew from the rule that governed it: which drill
// ([RunID]), which rule ([Name]), and the dose itself — an injected error
// ([Inject]/[Error]/[Class]) and/or a latency ([Delay]). There is deliberately
// no nested "effect" type: the dose never travels apart from its attribution,
// so one flat value says the whole sentence.
//
// Rules are an ORDERED list: the first one whose conditions match governs the
// attempt and evaluation stops there. Later rules never run, so a result
// carries AT MOST one rule's dose and one call is never affected by more than
// one experiment. Order IS precedence among rules on the same service — put
// the specific ones first.
//
// A governing rule that does not fire (Rate draw missed, guardrail tripped,
// blast radius full) still terminates selection but leaves the attempt untouched:
// the result is then the zero value. Matching decides governance; the draw
// decides whether the fault is applied, and there is no fall-through to the
// next match.
//
// It is delivered PER ATTEMPT, not per call: [evaluate] runs once inside the
// resilience executor's retry loop, so a call that retried produces one Result
// per attempt and nothing in this package aggregates them. A caller that needs
// "what happened to this call" must read the rounds (which is what the client
// seam writes each of these into, and what the observe kit reads back); a caller
// that captures only the LAST Result will miss a fire that happened on an
// earlier attempt and lost nothing on the final one.
//
// It carries no error: whether the call may proceed is the ERROR [evaluate]
// returns, so there is exactly one place to look and no second copy that could
// disagree.
type Result struct {
	// RunID is the document's [Config.RunID] — the drill execution this result
	// belongs to, empty when the document carries none. It is what lets a span, a
	// call record or a counter snapshot be grouped into "this drill" across
	// replicas, since [Rule.Name] is the rule's stable identity, not a run.
	RunID string

	// Name is the governing rule's configured [Rule.Name] — the rule's identity
	// and the key a platform joins telemetry on. Empty on the zero value.
	Name string

	// Inject reports whether the governing rule drew the error for this attempt.
	Inject bool

	// Error is the injected error's kind ([ErrorGeneric], [ErrorTimeout],
	// [ErrorReset], [ErrorRefused]) — the word the call record and the observe
	// tags carry. Empty unless Inject.
	Error string

	// Class is the failure class the rule declared ([ClassTransient],
	// [ClassPermanent], [ClassBusiness]); empty means [ClassTransient].
	Class string

	// Delay is the latency the rule applied, already including the jitter draw.
	// It is the sleep that was ASKED for: when the attempt's context is cancelled
	// mid-sleep, the result still reports the requested delay and [evaluate]
	// returns the context error.
	Delay time.Duration
}

// Fired reports whether this attempt was actually affected — an error injected
// or a latency applied. A cancelled latency still counts as fired: the fire was
// set, the call was affected, and the call record carries it.
func (r Result) Fired() bool { return r.Inject || r.Delay > 0 }

// Business reports whether the injected failure is the CALLER's mistake (the
// "business" class) rather than evidence against the downstream. Inbound
// adapters map it to their protocol's 4xx-equivalent and everything else to a
// downstream-failure shape, so the declared classification survives onto the
// wire. Meaningful only on a fired [Result] — the zero value's empty Class
// reads as not-business.
func (r Result) Business() bool { return r.Class == ClassBusiness }

func (m *Manager) Fire(ctx context.Context, t Target) (Result, error) {
	return evaluate(ctx, m.core.Load(), t)
}

// core is one direction's replaceable kernel — what a push swaps in the
// stable-shell pattern (the same shape resilience.Manager and
// loadbalance.Manager use): the document as pushed, whose Rules order IS
// precedence, plus the per-rule guardrail budgets reached by Name. It is built
// on every push and never mutated afterwards except through the budgets'
// atomics, so a caller that loaded it can use it for the attempt's whole
// duration without racing a push.
type core struct {
	cfg     Config             // the document as pushed; its Rules order IS precedence
	budgets map[string]*budget // by rule Name; the only mutable state, via atomics
}

// buildCore turns a pushed Config into a core. It RECONCILES against prev by
// rule NAME, but only when the run id is unchanged: a rule with the same name
// reuses prev's [budget], so its guardrail counters survive the push. A NEW
// run id reconciles against nothing — every rule starts from zero — because the
// platform re-arms the same rule names for every run, and a spent MaxAffected
// from the previous drill must not silently disable the next one. That is what
// makes a re-push idempotent WITHIN a run (a controller may push the same
// document repeatedly) and a new run fresh.
//
// A rule that no longer exists is dropped with the old core — which is the
// whole of the state cleanup. Presence in the pushed config is therefore a
// rule's lifecycle: appearing starts it, disappearing ends and releases it.
// Documents reaching [Manager.Refresh] are already validated (conf.Bind runs
// their expr tags when a document enters), so every name is present.
func buildCore(c Config, prev *core) *core {
	nc := &core{cfg: c, budgets: make(map[string]*budget, len(c.Rules))}
	// The run gate: only the SAME run id may inherit budgets (see above).
	var prevBudgets map[string]*budget
	if prev != nil && prev.cfg.RunID == c.RunID {
		prevBudgets = prev.budgets
	}
	for _, r := range c.Rules {
		b := prevBudgets[r.Name] // nil unless the same run already armed this name
		if b == nil {
			b = &budget{}
		}
		nc.budgets[r.Name] = b
	}
	return nc
}

// evaluate runs one attempt's roll-and-delay sequence against the kernel the
// caller loaded, and reports what it decided. It does NOT call the operation: the
// returned error is the whole "may I proceed?" signal — nil means the caller
// should run the real fn, anything else means it should return that error instead
// (an injected failure, or the context error when the injected latency was
// cancelled mid-sleep).
//
// It is the ONE shared fault sequence — both directions run through it
// ([Client.Fire] and [Server.Fire]) — so matching, guardrails and
// cancellation semantics cannot drift apart. Splitting the callback out is
// deliberate: whether to run the handler, and what a failure looks like on the
// wire, is protocol knowledge the adapters own. The outbound adapter calls fn
// and records the decision on the call; a server adapter returns a 503, a gRPC
// status or a failing RPC result, choosing its own shape without having run the
// handler first.
//
// The kernel is a PARAMETER rather than a receiver: the manager loaded it from
// its atomic pointer, and this function reaches for nothing else — no
// manager, no lock, no call state. That is what keeps the hot path to one atomic
// load plus the rule scan.
//
// t is the call being fired at, as the adapter named it: a rule is matched
// against its Service, Action and Metadata ([Rule.Matches]) — a rule targets a
// whole service by naming it, or one interface of it with an Action, or one
// slice of its traffic with a Match. The FIRST matching rule governs the attempt
// and evaluation stops there (see [Result]). Since every rule names a service,
// a call that names none matches nothing: a target's Service comes from the
// seam that built it, and this package never guesses one.
//
// The latency sleep is cancellable via ctx; on cancel it returns the context
// error (with the decision intact) so the caller's own timeout/budget logic
// reacts rather than retrying blindly. evaluate does NOT retry — retry is a
// client concern; for the full retry/timeout/breaker treatment on a client call
// use the outbound fault Manager's WrapClientExecutor with a chain.Executor.
//
// It records nothing anywhere. A fire is meant to be visible to every other
// layer — the observe kit tags a span and the access line with it — and that is
// the outbound adapter's job: it writes the returned decision onto the call's
// [chain.Recorder], which is what the other layers read (see
// the outbound fault Manager's WrapClientExecutor). Keeping the write there is
// what leaves this
// sequence free of any call state.
func evaluate(ctx context.Context, c *core, t Target) (Result, error) {
	if c == nil {
		return Result{}, nil
	}
	// Rules are an ORDERED list and the FIRST one whose conditions match governs
	// this attempt. Evaluation stops there: later rules never run, so one call is
	// never affected by more than one experiment. Order is therefore precedence —
	// specific rules go before a broader one.
	//
	// A matched rule that does not fire (Rate draw missed, guardrail tripped,
	// blast radius full) leaves the attempt untouched and STILL terminates
	// selection: matching decides governance, the draw decides whether the fault
	// is applied. There is no fall-through to the next matching rule.
	for _, r := range c.cfg.Rules {
		if !r.Matches(t) {
			continue
		}
		b := c.budgets[r.Name]
		// The governing rule's own guardrails: a forgotten fire self-heals instead
		// of running until the next deploy.
		if b.durationExhausted(r) {
			b.reportExhaustion(ctx, r, "max-duration")
			return Result{}, nil
		}
		o := r.roll()
		if !o.Inject && o.Delay == 0 {
			return Result{}, nil // the draw missed, or the rule carries no effect
		}
		// Spend one unit of the blast-radius budget only after the rule actually
		// drew a dose, so the cap is exact even under concurrency.
		if !b.spend(r) {
			b.reportExhaustion(ctx, r, "max-affected")
			return Result{}, nil
		}
		o.RunID = c.cfg.RunID
		if o.Delay > 0 && !timeutil.Sleep(ctx, o.Delay) {
			return o, ctx.Err()
		}
		if o.Inject {
			return o, newInjectedError(o.Error, o.Class)
		}
		return o, nil
	}
	return Result{}, nil
}

// budget is one rule identity's guardrail state across pushes: when its
// first affected call happened (the MaxDuration window), and how many calls it
// has touched (the MaxAffected budget). It is shared by every core the identity
// has had, so a push neither resets a rule's budget nor re-arms one that
// already expired; dropping the identity drops its last reference.
type budget struct {
	firstAt  atomic.Int64 // unix-nanos of the rule's first affected call; 0 = none
	affected atomic.Int64 // calls this rule actually touched
	notified atomic.Bool  // the exhaustion transition was reported
}

// reportExhaustion logs the ONE transition into exhaustion for this rule
// identity, so an operator sees why a fire stopped without polling per-process
// counters. Repeated no-op calls after the trip stay silent.
func (b *budget) reportExhaustion(ctx context.Context, r Rule, reason string) {
	if b.notified.CompareAndSwap(false, true) {
		log.Info(ctx, log.TagAppDef, "fault: rule exhausted; it stops firing for this run",
			log.String("rule", r.Name), log.String("reason", reason))
	}
}

// durationExhausted reports whether the MaxDuration window has elapsed. It is
// the time half of the guardrails, checked on the hot path before the draw (the
// count half is enforced by [budget.spend] when a roll actually fires).
func (b *budget) durationExhausted(r Rule) bool {
	if r.MaxDuration <= 0 {
		return false
	}
	first := b.firstAt.Load()
	return first != 0 && time.Since(time.Unix(0, first)) > r.MaxDuration
}

// spend books one affected call against the MaxAffected budget, or reports
// that the budget is already spent. It is called AFTER a rule rolled a dose and
// only then, so the budget is an exact cap: a concurrent batch of in-flight
// attempts cannot overshoot it (the CAS loop admits at most MaxAffected
// winners), and an attempt whose rate did not hit never spends one. The first
// winner also starts the MaxDuration window; the CAS makes that race-safe.
func (b *budget) spend(r Rule) bool {
	if r.MaxAffected > 0 {
		for {
			n := b.affected.Load()
			if n >= r.MaxAffected {
				return false
			}
			if b.affected.CompareAndSwap(n, n+1) {
				b.firstAt.CompareAndSwap(0, time.Now().UnixNano())
				return true
			}
		}
	}
	b.affected.Add(1)
	b.firstAt.CompareAndSwap(0, time.Now().UnixNano())
	return true
}
