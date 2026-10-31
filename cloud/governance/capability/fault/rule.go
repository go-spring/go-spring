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

package fault

import (
	"math/rand/v2"
	"strings"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
)

// rule.go is the CONFIGURATION vocabulary: what an operator writes. A rule
// states its target ([Rule]: service, metadata, action — the call's traffic
// class being one metadata dimension) and its effect/dose (rate, latency, error,
// max-affected, max-duration); [Config] is just one direction's list of them.
//
// Everything a rule itself computes lives here too, next to the fields it reads:
// whether it governs a call ([Rule.Matches]), what dose it draws for one
// attempt ([Rule.roll]) and the [Result] that roll produces. Both
// computations are local to the rule — they take a [Target] and draw randomness,
// nothing else. What they do NOT do is sequence anything: the kernel, the
// guardrails and the sleep live in manager.go, and the call's [Target] is built at
// call time, not configured.

// ActionMatch is a rule's match condition over a call's [Target.Action] — the
// interface being invoked ("insert", "GET /users/{id}"). Its zero value matches
// every action, so a rule that declares no action condition governs the whole
// service.
//
// Exact is equality; Prefix matches a leading string, which is how one rule
// covers a REST subtree whose concrete paths vary ("GET /users" covers
// "GET /users/{id}"). They are mutually exclusive in practice — Exact wins when
// both are set — so a rule states one intent.
//
// WHY SO SHALLOW: fault injection is not routing. A router needs rich matching
// because forwarding to the wrong backend is an incident; a fire needs matching
// an operator can predict, because the first question during a drill is "what
// exactly did we just hit". The mainstream chaos tools agree: Chaos Mesh's
// HTTPChaos matches on method, path (with wildcards) and a header map; ChaosBlade
// matches on uri/method and exposes regex as a SEPARATE opt-in parameter; the
// rich match grammars (Envoy StringMatcher, Istio StringMatch, Gateway API,
// Traefik, APISIX vars) belong to traffic management. The axis a fire usually
// needs beyond this is the dose — Rate, MaxAffected, MaxDuration — not a richer
// predicate. (Traffic class is not an axis here: it is one metadata entry, see
// [Rule.MetadataMatch].)
//
// EXTENSION: operators are added as NEW FIELDS under the same one-of convention,
// never by reinterpreting an existing field, so every config written today keeps
// its meaning. A regex operator would earn its place only when a real case needs
// to split one service by path pattern that Prefix cannot express; it would then
// be added here (compiled once per distinct pattern, not per call), not as a
// "match strategy" enum on an existing field. Case-insensitivity, if ever
// needed, is spelled " (?i)" in the pattern the way Traefik and Spring do it.
//
// It is fault's own type, structurally identical to [chain.ActionMatch]: the
// shared [go-spring.org/cloud/governance/capability/matcher] vocabulary
// covers metadata maps, not the single-value "which action" question, and fault
// must not import chain, whose matcher pairs with chain.Operation — the
// client-shaped call description a rule has no business reading (see [Target]).
type ActionMatch struct {
	Exact  string `value:"${exact:=}"`
	Prefix string `value:"${prefix:=}"`
}

// Matches reports whether action satisfies m. The first non-empty operator wins.
func (m ActionMatch) Matches(action string) bool {
	switch {
	case m.Exact != "":
		return action == m.Exact
	case m.Prefix != "":
		return strings.HasPrefix(action, m.Prefix)
	default:
		return true
	}
}

// Rule is one fault rule: it names one service (optionally one interface
// and one traffic slice of it) and the dose to apply. See [Config.Rules].
//
// There is deliberately NO per-replica (instance/host/pod) selector. Targeting
// one replica is a DELIVERY decision, not a property of the rule: mainstream
// chaos platforms select the target in their control plane and act only there
// (Chaos Mesh's daemon on the target node, ChaosBlade's per-host agent), and
// config centers gray-publish per instance (Nacos beta by IP, Apollo by
// IP/label). A rule-level field would instead ship the selector to EVERY process
// and make each re-decide, which (a) duplicates selection the platform already
// does, (b) can only ever work inbound — an outbound call cannot know which peer
// the balancer will pick — and (c) makes the document non-portable: the same file
// could no longer be replayed anywhere with the same result. If per-replica
// drills are needed, put that identity in the SOURCE layer (the adapter names
// this process when it fetches its document), never here. This axis was removed
// once; do not reintroduce it.
type Rule struct {
	// Name identifies the rule; the expr tag requires it non-empty. Together
	// with [Config.RunID] it forms the identity a config push reconciles against
	// — (run id, name) — so a rule keeps its MaxDuration/MaxAffected counters as
	// long as BOTH are unchanged: reordering
	// the list, editing another rule or re-pushing the same document inside one
	// run never resets a live rule, while a NEW run id starts every rule from
	// zero. That is what lets a scheduled controller re-push idempotently and a
	// new drill begin with a fresh blast-radius budget.
	//
	// Changing Service, MetadataMatch, ActionMatch or the dose knobs keeps the
	// counters (same run, same rule); renaming, removing-and-re-adding, or
	// bumping the run id starts fresh. It is also the name a [Result] reports
	// and the key telemetry joins on, within one run.
	Name string `value:"${name:=}" expr:"$ != ''"`

	// Service is the target this rule matches, exact-compare — the label a
	// starter names its calls by ("redis", "gorm:mysql", "http:user-svc", "gin").
	//
	// There is deliberately NO catch-all: a fire must name its target. Outbound,
	// an empty match would set EVERY downstream on fire at once, which is an
	// incident, not a drill; inbound, naming the one entrance ("gin") costs five
	// characters. The mainstream chaos tools agree — every Chaos Mesh experiment
	// has a selector, every ChaosBlade command a target; none offers a
	// "fire everything" mode. A rule that fails to name a service is a rule that
	// was written wrong, and the expr tag rejects it at parse time.
	Service string `value:"${service:=}" expr:"$ != ''"`

	// MetadataMatch narrows the rule to calls whose [Target.Metadata] satisfies
	// it: subset-match — every key it names must be present in the call's
	// metadata with an equal value (a call may carry dimensions the rule does not
	// name). Empty matches every call of the service. Several rules may share a
	// Service as long as their metadata differs; the first whose Service equals
	// the call's and whose metadata satisfies it wins — how one service is set on
	// fire differently per tenant, region or canary.
	//
	// The call's TRAFFIC CLASS is one of these dimensions, not a knob of its own:
	// the seams record the context's traffic kind under the ordinary key
	// "traffic" while they build the operation, so
	//
	//	match.traffic=loadtest   fires only synthetic load-test traffic
	//	match.traffic=real       fires only production traffic
	//	(no traffic condition)   fires every class
	//
	// There is no fault-specific "scope" field and no second matching path: the
	// class is just another dimension the subset rule already handles, and an
	// unknown class ("stress", "shadow") needs no code change.
	//
	// Value equality is deliberate (see [ActionMatch] for the why): chaos
	// targeting is mostly "this tenant / this canary", which is a label lookup,
	// not a pattern language. Chaos Mesh's request_headers and ChaosBlade's
	// business parameters are the same shape.
	//
	// It is [matcher.ExactMap], the exact-only shape of the shared matching
	// vocabulary: the map spelling is kept because a rule config reads better as
	// `match.tenant=acme` than as an indexed term list, and its semantics are
	// exactly the [matcher.Exact] terms of a [matcher.Selector] — subset-match
	// with the named key required to be present. Bind via indexed properties,
	// e.g. spring.fault.client.rules[0].match.tenant=acme.
	//
	// EXTENSION: richer metadata matching (prefix/suffix/contains) arrives as a
	// SIBLING field — MetadataMatchers matcher.Selector, the {key, kind, value}
	// list Envoy/Istio/Sentinel use — that ANDs with this map. This field's
	// spelling never changes, so a config written today keeps working forever.
	MetadataMatch matcher.ExactMap `value:"${match:=}"`

	// ActionMatch narrows the rule to calls whose [Target.Action] it matches —
	// the interface being invoked ("insert", "GET /users/{id}"). Empty matches
	// every action, so a rule without one fires the whole service. This is what
	// makes INTERFACE-level fault injection possible: fire one endpoint, one
	// method. See [ActionMatch].
	ActionMatch ActionMatch `value:"${action:=}"`

	// Rate is the per-call probability (0..1) of injecting the configured Error.
	// 0 means never inject; 1 means every call fails. Latency, when set, is
	// applied to every call regardless of Rate — use it to model a uniformly
	// slow downstream without forcing errors.
	Rate float64 `value:"${rate:=0}" expr:"$ >= 0 && $ <= 1"`

	// Latency is injected before every ATTEMPT (and before the error decision):
	// the outbound seam fires once per retry, so a call that retried N times
	// sleeps N x Latency and spends N budget units against the guardrails. That
	// matches how a network-layer delay behaves against a retrying client. Use it
	// to exercise slow-call and per-attempt timeout paths. 0 disables.
	Latency time.Duration `value:"${latency:=0}" expr:"int($) >= 0"`

	// LatencyJitter randomizes the injected latency: each attempt sleeps Latency
	// plus a uniform draw in [-LatencyJitter, +LatencyJitter], clamped to ≥ 0.
	// A fixed Latency can resonate with a fixed retry backoff; jitter spreads
	// the slow-down the way real network variance does. 0 (the default) keeps
	// the sleep exactly Latency.
	LatencyJitter time.Duration `value:"${latency-jitter:=0}" expr:"int($) >= 0"`

	// Error selects the injected failure kind, applied at Rate:
	//   [ErrorGeneric]   — a retryable injected error ([ErrInjected]), the default
	//   [ErrorTimeout]   — wraps context.DeadlineExceeded
	//   [ErrorReset]     — wraps syscall.ECONNRESET
	//   [ErrorRefused]   — wraps syscall.ECONNREFUSED
	// Rate 0 means no errors are injected (a latency-only rule).
	Error string `value:"${error:=generic}" expr:"$ in ['generic', 'timeout', 'reset', 'refused']"`

	// Class is how the caller's resilience stack classifies the injected failure:
	//   [ClassTransient] (default) — retried, counted by the circuit breaker
	//   [ClassPermanent]            — NOT retried, still counted
	//   [ClassBusiness]             — NOT retried, NOT counted
	//
	// It is the process-internal counterpart of the status code a wire-level
	// chaos tool can inject (HTTPChaos's replace.code, gRPCChaos's code): what the
	// resilience stack actually consumes is the classification, not the number.
	// Use "permanent"/"business" to exercise the paths a retryable injection
	// cannot reach — "this failure must not be retried", "this failure must not
	// open the breaker".
	//
	// WHY THE CLASS RIDES ON THE RULE (not on some caller-side list): this
	// framework's classification contract is IDENTITY-DRIVEN — [resilience.Retryable]
	// and [resilience.FailureClass] are interfaces an error implements to answer
	// for itself (a non-idempotent write opts out of retry the same way), in the
	// Hystrix typed-exception tradition. There is no resilience4j-style
	// ignore/record exception list for a synthetic error to fall through to, so
	// a class an injection cannot carry is a class nothing else could express:
	// dropping this field would make the permanent/business paths unreachable,
	// with no alternative entrance. [InjectedError] implementing those
	// interfaces is therefore not the injection overriding the framework — it
	// is the injected error obeying the same contract every real error obeys.
	Class string `value:"${class:=transient}" expr:"$ in ['transient', 'permanent', 'business']"`

	// MaxDuration is this rule's safety auto-off: after this much time
	// elapses since ITS first affected call, the rule stops applying any fault
	// (latency or error). It lets an operator arm a fire and walk away — a
	// forgotten rule self-heals rather than running until the next deploy.
	// 0 (the default) means no time bound. The window and its clock are PER
	// PROCESS: in a fleet of N replicas each one runs its own copy of the rule.
	// Hot-reload note: shortening it takes effect promptly; lengthening it does
	// not un-trip an already-expired fire — the first-affected stamp is kept per
	// rule across config pushes, so re-arming the window takes a process restart
	// (or a rule identity change).
	MaxDuration time.Duration `value:"${max-duration:=0}" expr:"int($) >= 0"`

	// MaxAffected is this rule's blast-radius cap: once this many calls
	// have received any fault effect (latency or error) FROM THIS RULE, it stops.
	// Use it to bound the impact of a high-Rate fire on production traffic. 0
	// (the default) means no count bound. The counter is PER PROCESS, so a fleet
	// of N replicas can affect up to N x MaxAffected calls; this is a per-instance
	// blast-radius cap, not a cluster-wide one. Per rule on purpose — one
	// rule's budget must not silently switch another one off; the counters
	// are keyed by the rule's identity.
	MaxAffected int64 `value:"${max-affected:=0}" expr:"$ >= 0"`
}

// Config is the set of rules ARMED in one traffic direction right now — an
// experiment's projection onto this process, not the experiment (the plan, its
// scheduling and its stop conditions live in the platform). One push may carry
// many rules; they all belong to the same [Config.RunID]. The SAME type serves
// both traffic directions: the outbound and inbound fault Managers each hold one,
// installed with their own Refresh. Both are fed from the fault root's
// document (the [Document] type here) — the client side from
// spring.fault.client.*, the server side from spring.fault.server.*. Fault is a
// peer of governance, not part of its document, and there is no per-starter
// fault key either.
//
// There is deliberately NO global default and NO catch-all: a call is set on
// fire only when a rule that NAMES its service matches it, and every rule must
// name one ([Rule.Service] has no empty case). A "fire everything" mode would
// be the most dangerous thing a chaos tool could carry — an operator aiming at
// one service would have to remember to disarm the rest of the process, and
// outbound an empty match would set every downstream on fire at once, which is
// an incident, not a drill. The mainstream chaos tools agree: every Chaos Mesh
// experiment names a selector and a mode, every ChaosBlade command names a
// target; neither has a "fire everything" state.
//
// An empty Config injects nothing, so a side built from it is a transparent
// no-op.
type Config struct {
	// RunID labels ONE DRILL EXECUTION of this document — the id a control plane
	// mints when an operator starts a drill and every replica shares. It is
	// document metadata, not a rule and not an effect: the rules-only invariant
	// (no top-level default) is untouched, because a run id changes nothing about
	// what fires.
	//
	// It exists because [Rule.Name] is the rule's IDENTITY, stable across
	// pushes, while "this particular execution" is a different question: spans,
	// call records and per-rule counters can only be grouped into one drill if
	// they carry a value that changes per run. It rides through
	// [Result.RunID] to the call record and the span
	// (`resilience.injected.run`); aggregation across replicas is the control
	// plane's, which reads each process's document and counters.
	//
	// It is NOT a configuration field: there is no binding tag, so an operator
	// cannot set it per direction and it can never disagree with the document's.
	// On the control-plane path the center assigns it from [Document.RunID]
	// BEFORE it commits; a caller that drives [Manager.Refresh] directly sets it
	// in code.
	// Empty is legal on that direct path and means "no drill id" — identity then
	// falls back to [Rule.Name] alone.
	RunID string

	// Rules are an ORDERED list. The FIRST rule whose conditions match a call
	// governs it — evaluation stops there, so a call is never affected by more
	// than one rule. Order is precedence among rules on the SAME service (see
	// [evaluate]). Bind via indexed properties — the same shape on either side,
	// under that side's block, e.g.:
	//
	//	spring.fault.client.rules[0].name=redis-errors
	//	spring.fault.client.rules[0].service=redis:cache
	//	spring.fault.client.rules[0].rate=0.5
	//	spring.fault.server.rules[0].service=grpc
	//	spring.fault.server.rules[0].rate=0.2
	Rules []Rule `value:"${rules:=}"`
}

// Config has NO Validate method: every constraint a rule carries is a
// single-field rule that lives on the field's `expr` tag, judged by conf.Bind
// when the document enters. The cross-field and cross-rule candidates were
// each examined and declined — a duplicate name only makes the later rule
// unreachable (selection stops at the first match), and a LatencyJitter with
// no Latency is silently inert (see [Rule.roll]), both typos to notice in
// review rather than states to defend against.

// Matches reports whether r governs a call: its Service equals the call's
// service, its Action matches the call's, and the call's metadata satisfies
// its Match.
func (r Rule) Matches(t Target) bool {
	if r.Service != t.Service {
		return false
	}
	if !r.ActionMatch.Matches(t.Action) {
		return false
	}
	return r.MetadataMatch.Matches(t.Metadata)
}

// roll draws this rule's dose for one attempt into a [Result]: the latency
// first (always, when Latency > 0), then the error at Rate. Latency and error
// are independent knobs; either one counts as affected for the guardrails. One
// rule produces one result, and [evaluate] stops at the first matching rule, so
// a call is governed by exactly one. [Result.RunID] is left empty — the kernel,
// not the rule, knows which document it published.
//
// The rule is the WHOLE declaration — there is no global effect to fall back to,
// so whatever this rule leaves zero is zero for the calls it governs.
func (r Rule) roll() Result {
	o := Result{Name: r.Name}
	if r.Latency > 0 {
		o.Delay = r.Latency
		if r.LatencyJitter > 0 {
			// Uniform draw in [-jitter, +jitter], clamped at 0: latency stays a
			// lower bound, and a jitter larger than latency cannot make it
			// negative. Jitter breaks the resonance of a fixed sleep with a
			// fixed retry backoff.
			o.Delay += time.Duration((rand.Float64()*2 - 1) * float64(r.LatencyJitter))
			if o.Delay < 0 {
				o.Delay = 0
			}
		}
	}
	if r.Rate > 0 && rand.Float64() < r.Rate {
		o.Inject = true
		o.Error = r.Error
		o.Class = r.Class
	}
	return o
}
