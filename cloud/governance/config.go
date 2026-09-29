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
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/loadbalance"
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
//   - [Config.Server] (inbound) — [resilience.ServerPolicy]: rate limit, concurrency
//     cap, inbound breaker, handling budget. This is the protection this process
//     wants against the requests it RECEIVES. No retry (a handler that already
//     produced side effects cannot be replayed) and no endpoint selection (there
//     is no endpoint to choose) — those are not disabled here, they have no
//     representation in the model at all.
//
// [Config.Enabled] and [Config.Driver] stay at the top because they are the two
// process-wide decisions: one on/off switch, and one resilience backend. The
// backend is deliberately NOT per direction, because [Driver] answers for both —
// one name still switches the whole process.
//
// The direction of a RULE is expressed by which block it lives in, and the rule's
// service label is what runtime lookups match. The two label sets are disjoint by
// convention (an inbound label names the server, e.g. "gin:0.0.0.0:8080"; an
// outbound one names the dependency, e.g. "redis:cache"), so a starter's label
// resolves in exactly one block.
type Config struct {
	// Enabled gates the whole center — BOTH directions. When false, every module
	// is disarmed: [Center.serviceFor] returns a zero Policy and
	// [Center.serverPolicyFor] a zero ServerPolicy (transparent pass-throughs)
	// regardless of Default/Rules, and neither fault side injects. Importing
	// starter-governance with no configured source is therefore a no-op.
	Enabled bool `value:"${enabled:=false}"`

	// Driver names the resilience backend all services use ("default" or
	// "sentinel"). Centralizing the driver means one place switches the backend
	// for every service, instead of each starter's own ${...driver}. It is one key
	// for both directions because a [resilience.Driver] implements both: the
	// backend that builds an outbound executor also builds the inbound admission
	// executor, so there is nothing to select twice.
	Driver string `value:"${driver:=default}"`

	// Client is the outbound half of the document. Bind via govern.client.*.
	Client ClientConfig `value:"${client:=}"`

	// Server is the inbound half of the document. Bind via govern.server.*.
	Server ServerConfig `value:"${server:=}"`
}

// ClientConfig is the outbound half of the governance document: what this process
// needs in its role as a CALLER.
type ClientConfig struct {
	// Default is the outbound policy applied to every service that no Rule
	// matches. Most deployments set only this and let every service share it;
	// per-service exceptions go under Rules. It carries only policy knobs — NOT
	// the on/off switch or the backend name, which are process-wide at the top of
	// the document ([Config.Enabled], [Config.Driver]) and deliberately NOT
	// re-bindable per service. Bind via govern.client.default.* (e.g.
	// govern.client.default.attempt-timeout=500ms,
	// govern.client.default.balancer=least_conn).
	Default ClientDefaultPolicy `value:"${default:=}"`

	// Rules are per-service outbound policy entries — one service per Rule, no
	// grouping several labels into one entry (they would silently share a
	// policy). [Center.policyFor] returns the Rule whose Service equals the
	// label; when no Rule matches it returns Default. Each Rule's embedded
	// halves fully replace Default for the matched service — not a field-wise
	// merge: a resilience.ClientPolicy field of 0 means "disabled", so a partial
	// merge could not distinguish "explicitly set to 0" from "left unset".
	// Bind via indexed properties:
	//
	//	govern.client.rules[0].service=redigo:cache
	//	govern.client.rules[0].attempt-timeout=100ms
	//	govern.client.rules[1].service=gorm:mysql:orders
	//	govern.client.rules[1].attempt-timeout=3s
	//
	// The service label (with its colons) lives in a value, not a key, so it
	// is dot-safe and needs no escaping in .properties or YAML — unlike a
	// map keyed by label, where the colon would have to appear in the key.
	Rules []ClientRule `value:"${rules:=}"`

	// Fault is this direction's fault-injection config (chaos engineering). The
	// client side injects INSIDE the resilience executor's retry loop (see
	// [fault.WrapClientExecutor]), so a fire here exercises this process's own
	// protection stack. The server side has its own, independent config under
	// [ServerConfig.Fault] — one direction's fire never affects the other, which
	// is what lets an operator burn outbound calls without failing inbound
	// requests (and vice versa). Bind via govern.client.fault.*.
	Fault fault.Config `value:"${fault:=}"`
}

// ServerConfig is the inbound half of the governance document: what this process
// needs in its role as a CALLEE.
type ServerConfig struct {
	// Default is the inbound admission model applied to every route that no Rule
	// matches. Bind via govern.server.default.* (e.g.
	// govern.server.default.rate-limit=1000,
	// govern.server.default.max-concurrent=32).
	Default resilience.ServerPolicy `value:"${default:=}"`

	// Rules are per-route admission entries, matched by the same exact label the
	// inbound middleware passes to [resilience.Manager.ServerExecutorFor]. They let one
	// process run several servers (or routes) under different limits — a public
	// endpoint and an internal one do not share a budget. A matched Rule fully
	// replaces Default, by the same no-field-wise-merge rule as [ClientConfig].
	// Bind via indexed properties:
	//
	//	govern.server.rules[0].service=gin:0.0.0.0:8080
	//	govern.server.rules[0].rate-limit=500
	//	govern.server.rules[1].service=grpc:inventory.Inventory/Get
	//	govern.server.rules[1].max-concurrent=64
	Rules []ServerRule `value:"${rules:=}"`

	// Fault is this direction's fault-injection config. The server side injects
	// at the inbound seam (see [fault.ApplyServer]), so a fire here tests this
	// process's error paths, its observe classification and its inbound breaker,
	// and lets upstream callers' retry/breaker behaviour be verified without
	// touching a real dependency. Independent of [ClientConfig.Fault], including
	// the guardrails: each direction counts its own MaxDuration/MaxAffected, so a
	// self-healed client fire does not disarm a server one. Bind via
	// govern.server.fault.*.
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
// this struct's own level (govern.client.default.attempt-timeout,
// govern.client.default.balancer, ...).
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
// Like ClientDefaultPolicy it carries no Enabled/Driver: those are process-wide, not
// per-service.
type ClientRule struct {
	// Service is the service label this Rule matches, exact-compare — the identity
	// a client starter passes when resolving an outbound executor, e.g.
	// "redis:cache", "gorm:mysql:primary", "http:user-svc". Label matching is plain
	// equality, so the label must be spelled exactly as the starter spells it.
	// Empty matches nothing (use Default instead).
	Service string `value:"${service:=}"`
	resilience.ClientPolicy
	loadbalance.Selection
}

// ServerRule is the inbound counterpart of [ClientRule]: one per-entrance
// admission entry. It carries no Selection — there is no endpoint to choose
// inbound — and no retry, because [resilience.ServerPolicy] cannot express one.
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
	// identity the admission middleware passes, which is THIS process's entrance:
	// the listening address ("gin:0.0.0.0:8080", "http-server::9090") or a label
	// the caller supplies (thrift). It is not the name of a callee. Matching is
	// plain equality, so it must be spelled exactly as the starter spells it.
	Service string `value:"${service:=}"`
	resilience.ServerPolicy
}

// validateClientRules rejects a client rule list that names the same service label
// twice. Labels are unique by contract (one Rule per service); a duplicate would
// silently resolve to whichever entry comes first, so it is a document error, not
// an ordering hint. Empty labels are skipped: they match nothing (use Default), and
// several empty ones are harmless.
func validateClientRules(rules []ClientRule) error {
	labels := make([]string, 0, len(rules))
	for _, r := range rules {
		labels = append(labels, r.Service)
	}
	return validateUniqueServices(labels)
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
