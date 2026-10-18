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
	"strings"
	"time"
)

// config.go is the CONFIGURATION vocabulary: what an operator writes. A rule
// states its target ([Rule]: service, metadata, action, scope) and its
// effect/dose (rate, latency, error, max-affected, max-duration); [Config] is
// just one direction's list of them. Nothing here decides or runs anything —
// the sequence that reads these lives in core.go/shell.go, and what it reads them
// against (the call's [Target]) is built at call time, not configured.

// ActionMatcher is a rule's match condition over a call's [Target.Action] — the
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
// needs beyond this is the dose — Rate, MaxAffected, MaxDuration, Scope — not a
// richer predicate.
//
// EXTENSION: operators are added as NEW FIELDS under the same one-of convention,
// never by reinterpreting an existing field, so every config written today keeps
// its meaning. A regex operator would earn its place only when a real case needs
// to split one service by path pattern that Prefix cannot express; it would then
// be added here (compiled once per distinct pattern, not per call), not as a
// "match strategy" enum on an existing field. Case-insensitivity, if ever
// needed, is spelled " (?i)" in the pattern the way Traefik and Spring do it.
//
// It is fault's own type, structurally identical to [chain.ActionMatch] and
// derived from nothing — chain's matcher pairs with chain.Selector, whose
// Matches takes a chain.Operation, which is the client-shaped call description a
// rule has no business reading (see [Target]).
type ActionMatcher struct {
	Exact  string `value:"${exact:=}"`
	Prefix string `value:"${prefix:=}"`
}

// Matches reports whether action satisfies m. The first non-empty operator wins.
func (m ActionMatcher) Matches(action string) bool {
	switch {
	case m.Exact != "":
		return action == m.Exact
	case m.Prefix != "":
		return strings.HasPrefix(action, m.Prefix)
	default:
		return true
	}
}

// Rule is one per-service fault rule. See [Config.Rules].
type Rule struct {
	// Service is the target this rule matches, exact-compare — the label a
	// starter names its calls by ("redis", "gorm:mysql", "http:user-svc", "gin").
	// Empty means catch-all (matches every call).
	Service string `value:"${service:=}"`

	// MetadataMatch narrows the rule to calls whose [Target.Metadata] satisfies
	// it: subset-match — every key it names must be present in the call's
	// metadata with an equal value (a call may carry dimensions the rule does not
	// name). Empty matches every call of the service. Several rules may share a
	// Service as long as their metadata differs; the first whose Service equals
	// the call's and whose metadata satisfies it wins — how one service is set on
	// fire differently per tenant, region or canary.
	//
	// Value equality is deliberate (see [ActionMatcher] for the why): chaos
	// targeting is mostly "this tenant / this canary", which is a label lookup,
	// not a pattern language. Chaos Mesh's request_headers and ChaosBlade's
	// business parameters are the same shape.
	//
	// It is a plain map, not a named type: the check is a few lines in
	// [Rule.matchesRule], and a type would add an exported symbol plus a method
	// for callers who only ever write this in a config file. Bind via indexed
	// properties, e.g.
	// spring.governance.client.fault.rules[0].match.tenant=acme.
	//
	// EXTENSION: richer metadata matching (prefix/regex/presence) will arrive as
	// a SIBLING field — MetadataMatchers []MetadataMatcher, the {key, operator,
	// value} list Envoy/Istio/Sentinel use — that ANDs with this map. This map's
	// type and config spelling must never change, so a config written today keeps
	// working forever.
	MetadataMatch map[string]string `value:"${match:=}"`

	// ActionMatch narrows the rule to calls whose [Target.Action] it matches —
	// the interface being invoked ("insert", "GET /users/{id}"). Empty matches
	// every action, so a rule without one fires the whole service. This is what
	// makes INTERFACE-level fault injection possible: fire one endpoint, one
	// method. See [ActionMatcher].
	ActionMatch ActionMatcher `value:"${action:=}"`

	// Scope restricts this fire to a class of traffic identified by the
	// load-test marker carried in the call's context (the injected
	// [traffic.Propagator]'s IsLoadTest):
	//   ""        — inject into all traffic
	//   "real"    — inject only into real traffic, skipping load-test requests
	//               (so synthetic load from cloud/experimental/loadtest does not get faulted)
	//   "loadtest"— inject only into load-test traffic (so faults exercise the
	//               resilience stack under synthetic load without disturbing
	//               production traffic)
	// An unknown value behaves as "". It is per-rule because it is a targeting
	// dimension like the others: one experiment may burn real traffic while
	// another burns the synthetic load.
	Scope string `value:"${scope:=}"`

	// Rate is the per-call probability (0..1) of injecting the configured Error.
	// 0 means never inject; 1 means every call fails. Latency, when set, is
	// applied to every call regardless of Rate — use it to model a uniformly
	// slow downstream without forcing errors.
	Rate float64 `value:"${rate:=0}"`

	// Latency is injected before each call (and before the error decision). Use
	// it to exercise slow-call and per-attempt timeout paths. 0 disables.
	Latency time.Duration `value:"${latency:=0}"`

	// LatencyJitter randomizes the injected latency: each call sleeps Latency
	// plus a uniform draw in [-LatencyJitter, +LatencyJitter], clamped to ≥ 0.
	// A fixed Latency can resonate with a fixed retry backoff; jitter spreads
	// the slow-down the way real network variance does. 0 (the default) keeps
	// the sleep exactly Latency.
	LatencyJitter time.Duration `value:"${latency-jitter:=0}"`

	// Error selects the injected failure kind, applied at Rate:
	//   "" / "generic" — a retryable injected error ([ErrInjected])
	//   "timeout"      — wraps context.DeadlineExceeded
	//   "reset"        — wraps syscall.ECONNRESET
	//   "refused"      — wraps syscall.ECONNREFUSED
	// Empty plus Rate 0 means no errors are injected.
	Error string `value:"${error:=}"`

	// MaxDuration is this experiment's safety auto-off: after this much time
	// elapses since ITS first affected call, the rule stops applying any fault
	// (latency or error). It lets an operator arm a fire and walk away — a
	// forgotten experiment self-heals rather than running until the next deploy.
	// 0 (the default) means no time bound. Hot-reload note: shortening it takes
	// effect promptly; lengthening it does not un-trip an already-expired fire —
	// the first-affected stamp is kept per rule across config pushes, so
	// re-arming the window takes a process restart (or a rule identity change).
	MaxDuration time.Duration `value:"${max-duration:=0}"`

	// MaxAffected is this experiment's blast-radius cap: once this many calls
	// have received any fault effect (latency or error) FROM THIS RULE, it stops.
	// Use it to bound the impact of a high-Rate fire on production traffic. 0
	// (the default) means no count bound. Per rule on purpose — one experiment's
	// budget must not silently switch another one off; the counters are keyed by
	// the rule's identity.
	MaxAffected int64 `value:"${max-affected:=0}"`
}

// Config is one direction's list of fault EXPERIMENTS. The SAME type serves both
// traffic directions: [ClientManager] and [ServerManager] each hold one,
// installed with their own Refresh and fed from governance.Config's two direction
// blocks — the client side from spring.governance.client.fault.*, the server side
// from spring.governance.server.fault.*. There is no per-starter fault key.
//
// There is deliberately NO global default: a call is set on fire only when a
// rule matches it. Arming everything means writing a catch-all rule (empty
// Service), which shows up in the config diff and in an audit. A default would
// be the most dangerous thing a chaos tool could carry — an operator aiming at
// one service would have to remember to disarm the rest of the process, and
// "default" would also be misleading, since a matched rule does not inherit the
// unset knobs but replaces the effect wholesale. The mainstream chaos tools
// agree: every Chaos Mesh experiment names a selector and a mode, every
// ChaosBlade command names a target; neither has a "fire everything by default"
// state.
//
// An empty Config injects nothing, so a side built from it is a transparent no-op
// (see [Config.Inert]).
type Config struct {
	// Rules are this direction's experiments. A call is fired by the FIRST rule
	// that matches it AND whose Scope applies (see [Rule.matchesRule]); no match
	// means no fire. List specific rules before a catch-all, since matching stops
	// at the first hit. Bind via indexed properties — the same shape on either
	// side, under that side's block, e.g.:
	//
	//	spring.governance.client.fault.rules[0].service=redis:cache
	//	spring.governance.client.fault.rules[0].rate=0.5
	//	spring.governance.server.fault.rules[0].rate=1   # empty service => catch-all inbound
	Rules []Rule `value:"${rules:=}"`
}

// Inert reports whether c can inject nothing at all: no rule carries a rate or a
// latency. It is the derived replacement for a former Enabled switch — Rate and
// Latency are the only knobs that cause an effect (Error needs Rate > 0,
// LatencyJitter needs Latency > 0, and MaxDuration/MaxAffected only ever STOP an
// effect), so "no rule with rate or latency" is exactly "no effect is possible".
// An empty Config is therefore inert, and a side built from it stays a
// transparent no-op.
//
// It is the hot path's early-out (see [evaluate]). Exported so the governance
// center can report whether a side is armed.
func (c Config) Inert() bool {
	for _, r := range c.Rules {
		if r.Rate > 0 || r.Latency > 0 {
			return false
		}
	}
	return true
}

// matchesRule reports whether r governs a call: its Service is empty
// (catch-all) or equals the call's service, its Action matches the call's, AND
// the call's metadata satisfies its Match.
func (r Rule) matchesRule(t Target) bool {
	if r.Service != "" && r.Service != t.Service {
		return false
	}
	if !r.ActionMatch.Matches(t.Action) {
		return false
	}
	// The metadata subset-match, inline: every key the rule names must agree.
	for k, want := range r.MetadataMatch {
		if t.Metadata[k] != want {
			return false
		}
	}
	return true
}
