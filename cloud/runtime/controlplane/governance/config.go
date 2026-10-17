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

// config.go is the governance DOCUMENT: the rule model ([Config],
// [ClientDefaultPolicy], [Rule], [ServerRule]) every source delivers and the center
// resolves from. The center that consumes it lives in center.go; the contract
// that delivers it lives in source.go.

package governance

import (
	"sort"
	"strings"

	"go-spring.org/cloud/runtime/dataplane/chain"
	"go-spring.org/cloud/runtime/dataplane/fault"
	"go-spring.org/cloud/runtime/dataplane/guard"
	"go-spring.org/cloud/runtime/dataplane/loadbalance"
	"go-spring.org/cloud/runtime/dataplane/resilience"
	"go-spring.org/stdlib/errutil"
)

// Config is the single source of truth for governance. A [Source] delivers it
// (starter-governance binds the bean-injected source); the center resolves a
// policy per service label from it and hands the result to the module
// authorities — so every client reads a resolved policy instead of carrying its
// own resilience config.
//
// The document is laid out by DIRECTION, because one process governs two traffic
// directions whose models differ:
//
//   - [Config.Client] (outbound) — [resilience.ClientPolicy]: retry, per-attempt
//     timeout, downstream breaker, bulkhead, fallback. Plus endpoint selection
//     ([loadbalance.Selection]), which is outbound by nature. This is the
//     protection this process wants when it CALLS something.
//   - [Config.Server] (inbound) — [guard.Policy]: rate limit, concurrency
//     cap, inbound breaker, handling budget. This is the protection this process
//     wants against the requests it RECEIVES. No retry (a handler that already
//     produced side effects cannot be replayed) and no endpoint selection (there
//     is no endpoint to choose) — those are not disabled here, they have no
//     representation in the model at all.
//
// [Config.Driver] stays at the top because it is the one process-wide decision:
// the resilience backend. It is deliberately NOT per direction, because [Driver]
// answers for both — one name still switches the whole process.
//
// There is no on/off switch. A document that configures nothing — empty
// [Config.Client] and [Config.Server] blocks, a zero Default, no rules —
// resolves zero policies for every label, which each module already reads as
// "no configuration": a pass-through executor, an untouched pool. That IS
// "governance off", so an empty document is a valid, inert one.
//
// The backend (which resilience.Driver builds the executors) is NOT part of this
// document: it is a static startup choice read from ${spring.resilience.driver},
// so it lives in the wiring, not in the pushed rules.
//
// The direction of a RULE is expressed by which block it lives in, and the rule's
// service label is what runtime lookups match. The two label sets are disjoint by
// convention (an inbound label names the server, e.g. "gin:0.0.0.0:8080"; an
// outbound one names the dependency, e.g. "redis:cache"), so a starter's label
// resolves in exactly one block.
type Config struct {
	// Client is the outbound half of the document. Bind via spring.governance.client.*.
	Client ClientConfig `value:"${client:=}"`

	// Server is the inbound half of the document. Bind via spring.governance.server.*.
	Server ServerConfig `value:"${server:=}"`
}

// ClientConfig is the outbound half of the governance document: what this process
// needs in its role as a CALLER.
type ClientConfig struct {
	// Default is the outbound policy applied to every service that no Rule
	// matches. Most deployments set only this and let every service share it;
	// per-service exceptions go under Rules. It carries only policy knobs — NOT
	// the backend name, which is a static startup choice
	// (${spring.resilience.driver}) and deliberately NOT re-bindable per service. Bind via
	// spring.governance.client.default.* (e.g.
	// spring.governance.client.default.timeout=500ms,
	// spring.governance.client.default.balancer=least_conn).
	Default ClientDefaultPolicy `value:"${default:=}"`

	// Rules are per-service outbound policy entries — one service per Rule, no
	// grouping several labels into one entry (they would silently share a
	// policy). [Center.clientPolicyFor] returns the Rule whose Service equals the
	// label; when no Rule matches it returns Default. Each Rule's embedded
	// halves fully replace Default for the matched service — not a field-wise
	// merge: a resilience.ClientPolicy field of 0 means "disabled", so a partial
	// merge could not distinguish "explicitly set to 0" from "left unset".
	// Bind via indexed properties:
	//
	//	spring.governance.client.rules[0].service=redigo:cache
	//	spring.governance.client.rules[0].timeout=100ms
	//	spring.governance.client.rules[1].service=gorm:mysql:orders
	//	spring.governance.client.rules[1].timeout=3s
	//
	// The service label (with its colons) lives in a value, not a key, so it
	// is dot-safe and needs no escaping in .properties or YAML — unlike a
	// map keyed by label, where the colon would have to appear in the key.
	Rules []ClientRule `value:"${rules:=}"`

	// Fault is this direction's fault-injection config (chaos engineering). The
	// client side injects INSIDE the resilience executor's retry loop (see
	// [fault.ClientManager.WrapClientExecutor]), so a fire here exercises this process's own
	// protection stack. The server side has its own, independent config under
	// [ServerConfig.Fault] — one direction's fire never affects the other, which
	// is what lets an operator burn outbound calls without failing inbound
	// requests (and vice versa). Bind via spring.governance.client.fault.*.
	Fault fault.Config `value:"${fault:=}"`
}

// ServerConfig is the inbound half of the governance document: what this process
// needs in its role as a CALLEE.
type ServerConfig struct {
	// Default is the inbound model applied to every route that no Rule
	// matches. Bind via spring.governance.server.default.* (e.g.
	// spring.governance.server.default.rate-limit=1000,
	// spring.governance.server.default.max-concurrent=32).
	Default guard.Policy `value:"${default:=}"`

	// Rules are per-instance inbound entries, matched by the same exact label an
	// adapter passes to guard's Identity. They let one
	// process run several servers (or routes) under different limits — a public
	// endpoint and an internal one do not share a budget. A matched Rule fully
	// replaces Default, by the same no-field-wise-merge rule as [ClientConfig].
	// Bind via indexed properties:
	//
	//	spring.governance.server.rules[0].service=gin:0.0.0.0:8080
	//	spring.governance.server.rules[0].rate-limit=500
	//	spring.governance.server.rules[1].service=grpc:inventory.Inventory/Get
	//	spring.governance.server.rules[1].max-concurrent=64
	Rules []ServerRule `value:"${rules:=}"`

	// Fault is this direction's fault-injection config. The server side injects
	// at the inbound seam (see [fault.Server.Fire]), so a fire here tests this
	// process's error paths, its observe classification and its inbound breaker,
	// and lets upstream callers' retry/breaker behaviour be verified without
	// touching a real dependency. Independent of [ClientConfig.Fault], including
	// the guardrails: each direction counts its own MaxDuration/MaxAffected, so a
	// self-healed client fire does not disarm a server one. Bind via
	// spring.governance.server.fault.*.
	Fault fault.Config `value:"${fault:=}"`
}

// ClientDefaultPolicy is the pair of halves applied to every outbound service that no
// [Rule] matches: the protection knobs (retry, breaker, bulkhead, timeout) and the
// endpoint-selection knobs (strategy, outlier suspension).
//
// A service's outbound governance is ONE configuration consumed by TWO modules —
// resilience builds the executor from the first half, loadbalance drives the pool
// from the second — and each module declares its own half's type. This is where a
// deployment composes them, so a service is configured once and both modules
// answer to the same label. Both halves are embedded, so their fields bind at
// this struct's own level (spring.governance.client.default.timeout,
// spring.governance.client.default.balancer, ...).
type ClientDefaultPolicy struct {
	resilience.ClientPolicy
	loadbalance.Selection
}

// Rule is one per-service outbound entry, carrying the same pair as
// [ClientDefaultPolicy]: the embedded resilience.ClientPolicy is this service's protection
// configuration (retry, breaker, bulkhead, timeout), the embedded
// loadbalance.Selection is its endpoint-selection configuration (balancer
// strategy, outlier suspension). Service is the single label it applies to (exact
// match); labels are unique across Rules (a duplicate is rejected at dispatch).
// Like ClientDefaultPolicy it carries no Driver: that is process-wide, not
// per-service.
type ClientRule struct {
	// Service is the call's target this Rule applies to, exact-compare — the
	// label a client starter passes when resolving an outbound executor
	// ("redis:cache", "gorm:mysql:primary", "http:user-svc"). It must be spelled
	// exactly. Empty matches nothing (use Default instead).
	Service string `value:"${service:=}"`

	// Match narrows the rule to calls whose [chain.Request.Metadata] satisfies
	// it (subset-match: every key it names must equal the call's). Empty matches
	// every call of the service, so a rule without a Match behaves exactly as one
	// did before: it governs the whole service. Several rules may share a Service
	// as long as their Match differs — the first rule in the list whose Service
	// equals the call's and whose Match it satisfies wins, which is how one
	// service is governed differently per tenant, region or canary. Bind via
	// indexed properties, e.g. spring.governance.client.rules[0].match.tenant=acme.
	Match chain.Selector `value:"${match:=}"`

	// Action narrows the rule to calls whose [chain.Request.Action] it matches — the
	// interface being invoked ("insert", "GET /users/{id}"). Empty matches every
	// action, so a rule without an Action governs the whole service. This is what
	// makes INTERFACE-level governance possible: only rate-limit one endpoint,
	// only fire one method. See [chain.ActionMatch].
	Action chain.ActionMatch `value:"${action:=}"`

	resilience.ClientPolicy
	loadbalance.Selection
}

// ServerRule is the inbound counterpart of [ClientRule]: one per-entrance
// inbound entry. It carries no Selection — there is no endpoint to choose
// inbound — and no retry, because [guard.Policy] cannot express one.
// Labels are unique across the list (a duplicate is rejected at dispatch);
// an empty label matches nothing (use Default instead).
//
// The label field keeps the family's word "service" even though the value names
// an inbound ENTRANCE, not a downstream service: the client side's labels are no
// more literally services (a "kafka:10.0.0.1:9092" is an address, a
// "gorm:mysql:orders" is a datasource), and one word for "the identity a starter
// passes" is what keeps the two blocks readable as one mechanism.
type ServerRule struct {
	// Service is the inbound label this ServerRule matches, exact-compare — the
	// identity the inbound middleware passes, which is THIS process's entrance:
	// the listening address ("gin:0.0.0.0:8080", "http-server::9090") or a label
	// the caller supplies (thrift). It is not the name of a callee. Matching is
	// plain equality, so it must be spelled exactly as the starter spells it.
	Service string `value:"${service:=}"`
	guard.Policy
}

// validateClientRules rejects a client rule list that names the same
// (Service, Match) pair twice. A Service may now appear in several rules as long
// as their Match differs — that is how one service is governed differently per
// tenant, region or canary — but two rules that select the SAME calls would
// silently resolve to whichever comes first, so that is a document error, not an
// ordering hint. Empty services are skipped: they match nothing (use Default).
func validateClientRules(rules []ClientRule) error {
	seen := make(map[string]struct{}, len(rules))
	for _, r := range rules {
		if r.Service == "" {
			continue
		}
		key := r.Service + "\x00" + canonicalAction(r.Action) + "\x00" + canonicalSelector(r.Match)
		if _, dup := seen[key]; dup {
			return errutil.Explain(nil, "governance: duplicate rule for service %q, action %+v and selector %v", r.Service, r.Action, map[string]string(r.Match))
		}
		seen[key] = struct{}{}
	}
	return nil
}

// canonicalAction renders an action matcher as a stable string for the
// uniqueness check.
func canonicalAction(a chain.ActionMatch) string {
	return a.Exact + "\x01" + a.Prefix
}

// canonicalSelector renders a selector as a stable, order-independent string, so
// two selectors that name the same keys and values compare equal however their
// maps were laid out.
func canonicalSelector(s chain.Selector) string {
	if len(s) == 0 {
		return ""
	}
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(s[k])
		b.WriteByte('&')
	}
	return b.String()
}

// validateServerRules is [validateClientRules] for the inbound rule list. The
// two lists are validated independently: a label may legitimately appear in both
// (nothing prevents a system name being used as both an outbound and an inbound
// label), but not twice within one list.
func validateServerRules(rules []ServerRule) error {
	labels := make([]string, 0, len(rules))
	for _, r := range rules {
		labels = append(labels, r.Service)
	}
	return validateUniqueServices(labels)
}

// validateUniqueServices rejects a label list that names the same service twice.
func validateUniqueServices(labels []string) error {
	seen := make(map[string]struct{}, len(labels))
	for _, label := range labels {
		if label == "" {
			continue
		}
		if _, dup := seen[label]; dup {
			return errutil.Explain(nil, "governance: duplicate rule for service %q", label)
		}
		seen[label] = struct{}{}
	}
	return nil
}
