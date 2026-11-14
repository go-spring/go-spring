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
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/stdlib/errutil"
)

// rule.go is the SHARED rule vocabulary: the targeting axes and dose knobs every
// fault rule carries, whichever direction it governs. The direction packages
// embed [RuleCore] in their own rule type and add their effect — an outbound
// error kind and class, an inbound response status — so a configuration can
// never express an effect its direction cannot produce.

// RuleCore is the direction-neutral half of a rule: whom it targets and how much
// it applies. A direction's rule type embeds it and adds the effect; the shared
// kernel ([Kernel]) reads only this part for matching, guardrails and dose.
type RuleCore struct {
	// Name identifies the rule; the expr tag requires it non-empty. Together
	// with the direction document's RunID it forms the identity a config push
	// reconciles against — (run id, name) — so a rule keeps its
	// MaxDuration/MaxAffected counters as long as BOTH are unchanged: reordering
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
	//	match.traffic.exact=loadtest   fires only synthetic load-test traffic
	//	match.traffic.exact=real       fires only production traffic
	//	(no traffic condition)   fires every class
	//
	// There is no fault-specific "scope" field and no second matching path: the
	// class is just another dimension the subset rule already handles, and an
	// unknown class ("stress", "shadow") needs no code change.
	//
	// Value equality is deliberate (see [matcher.ActionMatch] for the why): chaos
	// targeting is mostly "this tenant / this canary", which is a label lookup,
	// not a pattern language. Chaos Mesh's request_headers and ChaosBlade's
	// business parameters are the same shape.
	//
	// It is the map spelling of the shared matching vocabulary, one
	// [matcher.StringMatch] per key — the op always spelled out
	// (`match.tenant.exact=acme`, `match.version.prefix=v2`), one condition per
	// key, documents being platform-generated rather than hand-written, and
	// evaluated as bound with no translation.
	MetadataMatch map[string]matcher.StringMatch `value:"${match:=}"`

	// ActionMatch narrows the rule to calls whose [Target.Action] it matches —
	// the interface being invoked ("insert", "GET /users/{id}"). Empty matches
	// every action, so a rule without one fires the whole service. This is what
	// makes INTERFACE-level fault injection possible: fire one endpoint, one
	// method. See [matcher.ActionMatch]. The config key stays `action.` (e.g.
	// action.exact=GET /users/{id}).
	ActionMatch matcher.ActionMatch `value:"${action:=}"`

	// Rate is the probability (0..1) of applying the rule's effect. 0 means never
	// inject; 1 means every attempt fails. Latency, when set, is applied to every
	// attempt regardless of Rate — use it to model a uniformly slow downstream
	// without forcing errors.
	//
	// It counts ATTEMPTS, not logical calls. An outbound call that retries is N
	// separate requests to the downstream, and each is judged independently —
	// the same per-request model Envoy and Istio use. For independent draws the
	// call-level failure probability is therefore 1-(1-Rate)^N; retries mask
	// part of the injected rate, which is exactly what a real flaky downstream
	// does.
	//
	// Rate and [Every] are two ways to decide the same thing and are mutually
	// exclusive ([RuleCore.Validate] rejects both).
	Rate float64 `value:"${rate:=0}" expr:"$ >= 0 && $ <= 1"`

	// Every fires the effect on every Nth matching ATTEMPT (1 = every attempt),
	// a deterministic alternative to [Rate]. Like Rate it counts attempts, so a
	// retried outbound call consumes more than one sequence number. The counter
	// is per rule identity and survives re-pushes within a run, like the
	// guardrail counters. 0 disables it; it is mutually exclusive with Rate.
	Every int `value:"${every:=0}" expr:"$ >= 0"`

	// StartAfter delays the whole rule: no effect (and no latency eligibility)
	// until this much time has passed since the rule's FIRST matching call. It
	// is what expresses "arm now, start the outage in 10s"; the window rides the
	// rule identity, so a re-push within a run does not restart it.
	StartAfter time.Duration `value:"${start-after:=0}" expr:"int($) >= 0"`

	// On and Off make the rule INTERMITTENT: from StartAfter onward it is
	// eligible for On, then silent for Off, repeating. Within an On window the
	// Rate/Every decision still applies, so "5s of 50% failures" is expressible.
	// The pair must be set together ([RuleCore.Validate]); 0/0 means always
	// eligible. This is how a drill produces "bad for a while, then recover" —
	// the shape a breaker's half-open probe and a balancer's re-admission need.
	On  time.Duration `value:"${on:=0}" expr:"int($) >= 0"`
	Off time.Duration `value:"${off:=0}" expr:"int($) >= 0"`

	// Latency is injected before every ATTEMPT (and before the effect decision):
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

	// MaxDuration is this rule's safety auto-off: after this much time
	// elapses since ITS first affected call, the rule stops applying any fault
	// (latency or effect). It lets an operator arm a fire and walk away — a
	// forgotten rule self-heals rather than running until the next deploy.
	// 0 (the default) means no time bound. The window and its clock are PER
	// PROCESS: in a fleet of N replicas each one runs its own copy of the rule.
	// Hot-reload note: shortening it takes effect promptly; lengthening it does
	// not un-trip an already-expired fire — the first-affected stamp is kept per
	// rule across config pushes, so re-arming the window takes a process restart
	// (or a rule identity change).
	MaxDuration time.Duration `value:"${max-duration:=0}" expr:"int($) >= 0"`

	// MaxAffected is this rule's blast-radius cap: once this many ATTEMPTS
	// have received any fault effect (latency or injection) FROM THIS RULE, it
	// stops. It counts attempts on purpose: every affected attempt is one real
	// corrupted request, so a retried call can spend more than one unit and a
	// call-level count would understate the blast radius. Use it to bound the
	// impact of a high-Rate fire on production traffic. 0 (the default) means
	// no count bound. The counter is PER PROCESS,
	// so a fleet of N replicas can affect up to N x MaxAffected calls; this is a
	// per-instance blast-radius cap, not a cluster-wide one. Per rule on purpose
	// — one rule's budget must not silently switch another one off; the counters
	// are keyed by the rule's identity.
	MaxAffected int64 `value:"${max-affected:=0}" expr:"$ >= 0"`
}

// Matches reports whether r governs a call: its Service equals the call's
// service, its Action matches the call's, and the call's metadata satisfies
// its Match.
func (r RuleCore) Matches(t Target) bool {
	if r.Service != t.Service {
		return false
	}
	if !r.ActionMatch.Matches(t.Action) {
		return false
	}
	return matcher.MetadataMatches(r.MetadataMatch, t.Metadata)
}

// Validate rejects a contradiction the field-level expr tags cannot see: Rate
// and Every both decide the same thing, and On/Off is a pair. It runs when a
// rule is bound (the binder validates every embedded RuleCore); a directly
// constructed configuration must call it itself.
func (r *RuleCore) Validate() error {
	if r.Every > 0 && r.Rate > 0 {
		return errutil.Explain(nil, "fault: rule %q sets both rate and every; use one", r.Name)
	}
	if (r.On > 0) != (r.Off > 0) {
		return errutil.Explain(nil, "fault: rule %q must set on and off together", r.Name)
	}
	return nil
}

// Dose is what the shared kernel decided for one matching call: whether the
// rule's effect applies and how long the attempt sleeps. The kernel applies the
// time pattern, the Rate/Every draw and the latency; the direction rule turns
// `Inject` into its own effect.
type Dose struct {
	Inject bool
	Delay  time.Duration
}

// Dose decides one matching call against the rule's time pattern and dose knobs.
// The kernel calls it for the first matching rule and nothing else.
func (r RuleCore) Dose(b *Budget, now time.Time) Dose {
	n := b.noteMatch(now)
	if !b.inWindow(r, now) {
		return Dose{}
	}
	d := Dose{Delay: r.drawLatency()}
	switch {
	case r.Every > 0:
		d.Inject = n%uint64(r.Every) == 0
	case r.Rate > 0 && rand.Float64() < r.Rate:
		d.Inject = true
	}
	return d
}

// drawLatency draws the per-attempt latency, including jitter.
func (r RuleCore) drawLatency() time.Duration {
	if r.Latency <= 0 {
		return 0
	}
	delay := r.Latency
	if r.LatencyJitter > 0 {
		// Uniform draw in [-jitter, +jitter], clamped at 0: latency stays a
		// lower bound, and a jitter larger than latency cannot make it
		// negative. Jitter breaks the resonance of a fixed sleep with a
		// fixed retry backoff.
		delay += time.Duration((rand.Float64()*2 - 1) * float64(r.LatencyJitter))
		if delay < 0 {
			delay = 0
		}
	}
	return delay
}
