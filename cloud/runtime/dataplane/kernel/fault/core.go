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
// the rule document ([Config]/[Rule]), the call identity a rule matches
// ([Target]), what one attempt decided ([Decision]), and the guardrail state
// model behind a document push.
//
// The direction-specific shells live in the direction trees, not here:
// [go-spring.org/cloud/runtime/dataplane/outbound/fault] decides one outbound
// attempt so an injected fault lands INSIDE the resilience executor's retry loop
// (the wrapped fn returns the injected error and the real executor
// retries/breaks around it), which is the whole point;
// [go-spring.org/cloud/runtime/dataplane/inbound/fault] decides an inbound call,
// leaving the handler to the adapter. Each owns its own document and kernel, so
// burning inbound requests leaves outbound calls untouched.
//
// What a rule matches is this package's OWN shape, [Target] — not
// chain.Operation (see Target's doc for why). Its other dependencies are cloud's
// vocabulary: traffic for the load-test marker Scope reads, plus the direction
// packages for their host stacks.
//
// State follows the family's stable-shell / replaceable-core model (the same
// shape resilience.Manager and loadbalance.Manager use), with the ownership the
// model requires: each SHELL — created through [Manager.NewShell] and wrapped by
// a direction package — owns its kernel pointer and reads it directly, so the
// call path never asks a manager anything. The manager is the authority: it
// REGISTERS every shell it creates, and a refresh builds the new kernel once and
// PUSHES it into every registered shell under its lock. A refresh RECONCILES
// rather than replaces: it reuses the counters of rules whose identity is
// unchanged, gives new rules fresh ones, and drops the state of rules that are
// gone. Presence in the pushed document is therefore an experiment's lifecycle —
// appearing starts it, disappearing ends and releases it — and a shell's Close
// unregisters it, which is why there is no separate start/stop call.
//
// The files split by role: this one and shell.go hold the SHARED KERNEL (the
// neutral vocabulary, the build/reconcile logic, the injection sequence and the
// direction-neutral manager/shell seam), config.go the CONFIGURATION vocabulary
// and error.go the outcome vocabulary.
package fault

import (
	"context"
	"math/rand/v2"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/traffic"
	"go-spring.org/stdlib/timeutil"
)

// Target is one call's identity AS A FAULT RULE SEES IT: what was called, which
// interface of it, and the caller's own targeting metadata. A rule is matched
// against these three, so an adapter that can name them can be put on fire —
// which is what lets the same rule vocabulary serve an outbound client call and
// an inbound request (see [ClientManager.WrapClientExecutor] and [Server.Fire]).
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
// a {value} tag (that is config.go's vocabulary). An adapter builds it at call
// time from what its protocol knows, which is the only side that does.
type Target struct {
	// Service is what the call is addressed to — the identity a rule's Service
	// matches by exact compare, and a governance label the operator writes rules
	// against: a client names the downstream ("redis:cache", "http:user-svc"), a
	// server names its own entrance ("gin", "grpc"). Its shape is the starter's to
	// choose, as it is on [chain.Operation.Service]; it must be non-empty, since a
	// rule that names a service can never match an empty one.
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

	// Metadata is the caller's own key-value targeting set — the third filter, and
	// the one that separates two calls to the SAME service: a rule's Match must be
	// a subset of this map (see [Rule.MetadataMatch]). It is filled by the caller, or by a
	// middleware reading request headers, and is matched, never emitted, so its
	// keys are whatever dimensions the deployment cares about ("tenant", "region",
	// "canary").
	//
	// Nil (the common case) means the call declares nothing, and only rules keyed
	// on service and action alone can match it.
	Metadata map[string]string
}

// direction is one side's state as the AUTHORITY sees it: the document last
// pushed, the kernel built from it, and the shells registered to receive it. All
// three live under one mutex — registration, refresh and reading the document are
// the authority's slow path, never the call path.
type direction struct {
	mu     sync.Mutex
	cfg    Config
	core   *core
	states map[coreHolder]struct{}
}

// coreHolder is a shell that owns a kernel pointer: [Client] (outbound) and
// [Server] (inbound). It is what a refresh fans out to.
type coreHolder interface {
	setCore(*core)
}

func newDirection() direction {
	return direction{states: map[coreHolder]struct{}{}}
}

// register is the state's own registration call, made by its constructor
// ([NewClient] / [NewServer]) — the shell registers ITSELF with the authority,
// which is what lets the authority perceive every state it must push to. The
// registration seeds the shell with the current kernel under the same lock, so a
// shell created while a refresh is in flight cannot start out with nothing.
func (d *direction) register(st coreHolder) {
	d.mu.Lock()
	if d.core != nil {
		st.setCore(d.core)
	}
	d.states[st] = struct{}{}
	d.mu.Unlock()
}

// unregister is the state's own deregistration, called from its Close: it stops
// receiving refreshes, and the kernel pointer it held is released with it.
func (d *direction) unregister(st coreHolder) {
	d.mu.Lock()
	delete(d.states, st)
	d.mu.Unlock()
}

// refresh builds the next kernel from c (reconciling against the current one) and
// PUSHES it into every registered shell through its [coreHolder.setCore]
// callback. The swap and the fan-out happen under the lock, so a shell can never
// observe a half-applied document.
func (d *direction) refresh(c Config) {
	d.mu.Lock()
	k := buildCore(c, d.core)
	d.cfg = c
	d.core = k
	for st := range d.states {
		st.setCore(k)
	}
	d.mu.Unlock()
}

// config returns the document last pushed to this direction.
func (d *direction) config() Config {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cfg
}

// core is one direction's replaceable kernel: the experiments in matching order,
// each carrying its own guardrail counters. It is built on every config push and
// never mutated afterwards except through the per-rule atomics, so a shell that
// loaded it can use it for the call's whole duration without racing a push.
//
// It is the "mutable core" of the stable-shell pattern the other authorities use
// (resilience.Manager/ServerManager / loadbalance.Manager): the shells live for the process,
// while this is what a push replaces.
type core struct {
	cfg   Config       // the document as pushed, for ClientConfig/ServerConfig
	rules []*ruleState // in matching order
}

// ruleState is one experiment's runtime state: the rule (immutable), its stable
// reconciliation identity, and the guardrail counters that outlive a push when
// the rule is still the same experiment.
type ruleState struct {
	rule  Rule
	id    string // stable across pushes: service[index] (empty service => [index])
	key   string // record key precomputed for a service-bound rule; "" for catch-all
	index int

	firstAt  atomic.Int64 // unix-nanos of the rule's first affected call; 0 = none
	affected atomic.Int64 // calls this rule actually touched
}

// buildCore turns a pushed Config into a core. It RECONCILES against prev: a rule
// whose identity is unchanged keeps its *ruleState (and so its guardrail
// counters), a new rule gets fresh counters, and a rule that no longer exists is
// dropped with the old core — which is the whole of the state cleanup. Presence
// in the pushed config is therefore the experiment's lifecycle: appearing starts
// it, disappearing ends and releases it.
func buildCore(c Config, prev *core) *core {
	nc := &core{cfg: c, rules: make([]*ruleState, 0, len(c.Rules))}
	var prevByID map[string]*ruleState
	if prev != nil {
		prevByID = make(map[string]*ruleState, len(prev.rules))
		for _, old := range prev.rules {
			prevByID[old.id] = old
		}
	}
	for i := range c.Rules {
		r := c.Rules[i]
		id := ruleID(r, i)
		st := prevByID[id]
		if st == nil {
			st = &ruleState{}
		}
		st.rule = r
		st.id = id
		st.index = i
		st.key = ""
		if r.Service != "" {
			st.key = r.Service + "[" + strconv.Itoa(i) + "]"
		}
		nc.rules = append(nc.rules, st)
	}
	return nc
}

// ruleID is the stable identity of one experiment across pushes: its service and
// position. A catch-all has no service, so its position alone identifies it.
// Reordering the list therefore re-arms the affected rules (new identities, fresh
// counters) — the documented price of positional identity.
func ruleID(r Rule, i int) string {
	return r.Service + "[" + strconv.Itoa(i) + "]"
}

// Decision is what the engine decided about ONE attempt — the explicit result
// of the injection sequence, so a caller (or a test) can see what happened
// instead of inferring it from the error or fishing it out of a record.
//
// It is delivered PER ATTEMPT, not per call: [evaluate] runs once inside
// the resilience executor's retry loop, so a call that retried produces one
// Decision per attempt and nothing in this package aggregates them. A caller
// that needs "what happened to this call" must read the rounds (which is what
// the client seam writes each of these into, and what the observe kit reads
// back); a caller that captures only the LAST Decision will miss a fire that
// happened on an earlier attempt and lost nothing on the final one.
//
// It carries no error: whether the call may proceed is the ERROR [evaluate]
// returns, so there is exactly one place to look and no second copy that could
// disagree.
type Decision struct {
	// Rule names the rule that GOVERNED this attempt — the matched rule's
	// identity ("svc[0]", catch-all included). It is filled whether or not the
	// fire took effect, which is what makes "why did my rule not fire?" answerable
	// from the decision: a Rule with no Fired means the rule was selected and
	// either its rate did not hit or its MaxDuration/MaxAffected stopped it. An
	// empty Rule means the sequence never got that far — the direction is inert or
	// no rule matched the call.
	Rule string

	// Inject reports whether an error was injected — in which case the caller
	// must return [evaluate]'s error instead of calling the real operation.
	Inject bool

	// Kind is the injected error's kind ("generic", "timeout", "reset",
	// "refused") — the word the call record and the observe tags carry. Empty
	// unless Inject.
	Kind string

	// Delay is the latency applied before the call, already including the jitter
	// draw. It is the sleep that was ASKED for: when the attempt's context is
	// cancelled mid-sleep, the decision still reports the requested delay and
	// [evaluate] returns the context error.
	Delay time.Duration
}

// Fired reports whether this attempt was actually affected — an error injected
// or a latency applied. It is the question a caller almost always means, and it
// is deliberately distinct from "a rule governed": a rule that was selected but
// whose rate did not hit leaves Fired false (see [Decision.Rule]).
//
// A cancelled latency still counts as fired: the fire was set, the call was
// affected, and the call record carries it.
func (d Decision) Fired() bool { return d.Inject || d.Delay > 0 }

// evaluate runs one attempt's decide-and-delay sequence against the kernel the
// caller loaded, and reports what it decided. It does NOT call the operation: the
// returned error is the whole "may I proceed?" signal — nil means the caller
// should run the real fn, anything else means it should return that error instead
// (an injected failure, or the context error when the injected latency was
// cancelled mid-sleep).
//
// It is the ONE shared fault sequence — both shells run through it ([Client.Fire]
// and [Server.Fire]) — so scope gating, guardrails and cancellation semantics
// cannot drift apart. Splitting the callback out is deliberate: whether to run
// the handler, and what a failure looks like on the wire, is protocol knowledge
// the adapters own. The client shell calls fn and records the decision on the
// call; a server adapter returns a 503, a gRPC status or a failing RPC result,
// choosing its own shape without having run the handler first.
//
// The kernel is a PARAMETER rather than a receiver: the shell loaded it from its
// own atomic pointer, and this function reaches for nothing else — no manager, no
// lock, no call state. That is what keeps the hot path to one atomic load plus
// the rule scan.
//
// t is the call being fired at, as the adapter named it: a rule is matched
// against its Service, Action and Metadata (see [match]) — a rule targets a whole
// service by naming it, or one interface of it with an Action, or one slice of
// its traffic with a Match. Service is the call's own [chain.Operation.Service],
// stated by the client that built the call; this package never guesses one, so a
// call that names no service matches only the rules that name none.
//
// The latency sleep is cancellable via ctx; on cancel it returns the context
// error (with the decision intact) so the caller's own timeout/budget logic
// reacts rather than retrying blindly. evaluate does NOT retry — retry is a
// client concern; for the full retry/timeout/breaker treatment on a client call
// use [ClientManager.WrapClientExecutor] with a chain.Executor.
//
// It records nothing anywhere. A fire is meant to be visible to every other
// layer — the observe kit tags a span and the access line with it — and that is
// the CLIENT shell's job: it writes the returned decision onto the call's
// [chain.Recorder], which is what the other layers read (see
// [ClientManager.WrapClientExecutor]). Keeping the write there is what leaves this
// sequence free of any call state.
func evaluate(c *core, prop traffic.Propagator, t Target, ctx context.Context) (Decision, error) {
	if c == nil || c.cfg.Inert() {
		return Decision{}, nil
	}
	// Pick the experiment: the first rule that matches the call AND whose Scope
	// applies to this traffic. No match means no fire — there is no default.
	st := match(c, prop, t, ctx)
	if st == nil {
		return Decision{}, nil
	}
	// The experiment's own guardrails: a forgotten fire self-heals instead of
	// running until the next deploy, and one experiment's budget cannot switch
	// another one off. A tripped rule still NAMES itself in the decision, which
	// is what makes "the rule matched but stopped firing" diagnosable.
	key := st.key
	if key == "" {
		// A catch-all rule names the call's own service in the record.
		key = t.Service + "[" + strconv.Itoa(st.index) + "]"
	}
	if st.exhausted() {
		return Decision{Rule: key}, nil
	}
	d := st.rule.decide(key)
	if d.Fired() {
		st.stamp()
	}
	if d.Delay > 0 && !timeutil.Sleep(ctx, d.Delay) {
		return d, ctx.Err()
	}
	if d.Inject {
		return d, newInjectedError(d.Kind)
	}
	return d, nil
}

// decide turns a rule's effect knobs into the [Decision] for one attempt: the
// latency first (always, when Latency > 0), then the error at Rate. Latency and
// error are independent knobs; either one counts as affected for the guardrails.
//
// The rule is the WHOLE experiment — there is no global effect to fall back to,
// so whatever this rule leaves zero is zero for the calls it governs.
func (r Rule) decide(key string) Decision {
	d := Decision{Rule: key}
	if r.Latency > 0 {
		d.Delay = r.Latency
		if r.LatencyJitter > 0 {
			// Uniform draw in [-jitter, +jitter], clamped at 0: latency stays a
			// lower bound, and a jitter larger than latency cannot make it
			// negative. Jitter breaks the resonance of a fixed sleep with a
			// fixed retry backoff.
			d.Delay += time.Duration((rand.Float64()*2 - 1) * float64(r.LatencyJitter))
			if d.Delay < 0 {
				d.Delay = 0
			}
		}
	}
	if r.Rate > 0 && rand.Float64() < r.Rate {
		d.Inject = true
		d.Kind = r.Error
	}
	return d
}

// exhausted reports whether this experiment's safety bounds have been spent:
// the MaxDuration window since its first affected call, or its MaxAffected
// budget. Zero disables each bound. The counters live on the ruleState, which a
// push REUSES while the rule identity is unchanged, so an already self-healed
// fire is not re-armed by an unrelated push.
func (st *ruleState) exhausted() bool {
	r := st.rule
	if r.MaxDuration > 0 {
		if first := st.firstAt.Load(); first != 0 && time.Since(time.Unix(0, first)) > r.MaxDuration {
			return true
		}
	}
	return r.MaxAffected > 0 && st.affected.Load() >= r.MaxAffected
}

// stamp records one affected call for the guardrails: the first one starts the
// MaxDuration window (the CAS makes it race-safe), and every one counts toward
// MaxAffected.
func (st *ruleState) stamp() {
	st.firstAt.CompareAndSwap(0, time.Now().UnixNano())
	st.affected.Add(1)
}

// match returns the first experiment in c that governs t — its Service equals the
// call's (catch-all when the rule's Service is empty), its Action matches the
// call's, its metadata satisfies its match condition, AND its Scope applies to
// this traffic — or nil when none match.
//
// A rule whose Scope excludes this call is SKIPPED, not terminal: a
// loadtest-only experiment must not stop a later real-traffic one from matching.
func match(c *core, prop traffic.Propagator, t Target, ctx context.Context) *ruleState {
	for _, st := range c.rules {
		if !st.rule.matchesRule(t) || !scopeApplies(st.rule.Scope, prop, ctx) {
			continue
		}
		return st
	}
	return nil
}

// scopeApplies reports whether a rule applies to a call, given its Scope and the
// call's load-test marker. Scope "" (or unknown) applies to all traffic; "real"
// skips load-test calls; "loadtest" skips real calls. It is per-rule, so the two
// directions and the experiments within them honour the same rule.
func scopeApplies(scope string, prop traffic.Propagator, ctx context.Context) bool {
	switch scope {
	case "real":
		return !prop.IsLoadTest(ctx)
	case "loadtest":
		return prop.IsLoadTest(ctx)
	default:
		return true
	}
}
