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

// Package resilience is the centralized protection authority for the process:
// retry, circuit breaking, rate limiting and timeout.
//
// This package is CLIENT-ONLY. Outbound calls run through a [chain.Executor]
// held for the client's lifetime; inbound requests run through
// [go-spring.org/cloud/governance/dataplane/inbound/admission]'s per-instance [admission.Guard], which has its own
// policy vocabulary and engine. There is no inbound manager, no inbound
// [Driver] method and no inbound policy type here — the two directions do not
// share a shape.
//
// The manager is container-free: it imports no IoC concept and holds no global.
// Its dependency on the governance center is inverted into plain types (Apply
// takes the pushed half), so this package never imports a control-plane root and
// no center reaches into this package's internals. The gs wiring
// (providing the manager as a bean, injecting it into the starters) lives in
// starter.go.
//
// The BACKEND, unlike the policies, is not pushed: it is fixed at construction —
// the [Driver] the container selected from ${spring.resilience.driver}, a
// startup decision, since switching it drops every executor's state.
//
// # A push stops here
//
// Each Apply publishes its capability's snapshot and that is the whole of it: it
// does not walk the registry and it does not reach into any executor. Each
// executor holds the client manager and notices on its next call that the
// generation behind its label moved, at which point it rebuilds only the organ
// whose knobs changed (see [defaultExecutor.snapshot]). The registry is therefore
// an identity table (one executor per label) and a shutdown list, not a
// change-propagation mechanism.
//
// One label maps to exactly ONE executor per direction. That is not an
// optimization but a semantic requirement: a breaker and limiter are
// per-RESOURCE state, so two objects sharing a label (a rebuilt client, two pools
// for one service name) must share the breaker rather than each getting its own
// and effectively doubling the trip threshold.
package resilience

import (
	"strconv"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// LimitsSettings is the limits half of the outbound document, PUSHED whole: the
// center hands it over and this package does the matching. This type IS the
// document — the control-plane limits root binds it and hands it back unchanged
// (see [LimitsSettings.Validate]) — so there is no separate binding twin.
//
// It carries values only, never the backend: which [Driver] builds the executors
// is a startup decision, not part of the tunable document.
//
// The zero settings configure nothing: every label resolves zero limits, which
// every caller reads as "no deadline, no schedule". There is no arming flag — an
// empty document is a pass-through, and so is an absent one.
type LimitsSettings struct {
	// Default is the limits applied to every service label no [LimitsRule]
	// matches. It carries NO value tag: the document has no global level, so a
	// configuration source can never set it; it exists for programmatic callers.
	Default Limits

	// Rules are the per-service overrides: the first rule whose Service equals the
	// call's and whose Match the call's metadata satisfies wins. An empty Service
	// matches NOTHING (it is not a catch-all — use Default for that).
	Rules []LimitsRule `value:"${rules:=}"`
}

// Validate is called by conf.Bind, so an invalid document is rejected at the
// source boundary rather than reaching the center.
func (s LimitsSettings) Validate() error {
	return ValidateLimits(s)
}

// LimitsRule is one limits override.
//
// Action is either [ServiceScopeAction] ("-"), meaning the whole service, or a
// real exact/prefix selector, meaning one interface. A real action always wins
// over the service marker. Match further narrows a real-action rule; the service
// marker does not accept one (a service-level rule already covers the label).
type LimitsRule struct {
	// Service is the service label this rule matches, exact-compare. Required.
	Service string `value:"${service:=}"`

	// Action selects the scope: [ServiceScopeAction] for the whole service, or a
	// real exact/prefix.
	Action chain.ActionMatch `value:"${action:=}"`

	// Match narrows a real-action rule by call metadata (subset-match). Must be
	// empty on a [ServiceScopeAction] rule.
	Match matcher.ExactMap `value:"${match:=}"`

	Limits
}

// BreakerSettings is the circuit-breaker half of the outbound document. This
// type IS the document: the control-plane breaker root binds it and hands it
// back unchanged (see [BreakerSettings.Validate]), so there is no separate
// binding twin.
type BreakerSettings struct {
	// Default is the breaker applied to every service label no [BreakerRule]
	// matches. It carries NO value tag: the document has no global level, so a
	// configuration source can never set it; it exists for programmatic callers.
	Default BreakerPolicy

	// Rules are the overrides. See [BreakerRule] for the scope rules.
	Rules []BreakerRule `value:"${rules:=}"`
}

// Validate is called by conf.Bind, so an invalid document is rejected at the
// source boundary rather than reaching the center.
func (s BreakerSettings) Validate() error {
	return ValidateBreaker(s)
}

// BreakerRule is one breaker override.
//
// Action is either [ServiceScopeAction] ("-") or a real exact/prefix. A service
// rule, when present, WINS OUTRIGHT: the interface-level rules of that service
// are not consulted. With no service rule, the most specific interface rule
// applies.
type BreakerRule struct {
	// Service is the label this rule applies to, exact-compare. Required.
	Service string `value:"${service:=}"`

	// Action selects the scope: "-" for the whole service, or a real
	// exact/prefix for one interface.
	Action chain.ActionMatch `value:"${action:=}"`

	BreakerPolicy
}

// RateLimitSettings is the rate-limit half of the outbound document. This type
// IS the document — the control-plane rate-limit root binds it and hands it back
// unchanged (see [RateLimitSettings.Validate]) — so there is no separate binding
// twin.
type RateLimitSettings struct {
	// Default is the rate limit applied to every service label no [RateLimitRule]
	// matches. It carries NO value tag: the document has no global level, so a
	// configuration source can never set it; it exists for programmatic callers.
	Default RateLimitPolicy

	// Rules are the overrides. See [RateLimitRule].
	Rules []RateLimitRule `value:"${rules:=}"`
}

// Validate is called by conf.Bind, so an invalid document is rejected at the
// source boundary rather than reaching the center.
func (s RateLimitSettings) Validate() error {
	return ValidateRateLimit(s)
}

// RateLimitRule is one rate-limit override.
//
// Rate limiting is INTERFACE-scoped only: Action must be a real exact/prefix
// selector. Neither [ServiceScopeAction] nor Match is accepted — a service-wide
// or metadata-keyed quota would make the accounting impossible to reason about.
type RateLimitRule struct {
	// Service is the label this rule applies to, exact-compare. Required.
	Service string `value:"${service:=}"`

	// Action is the interface being limited. Required and must not be the
	// service marker "-".
	Action chain.ActionMatch `value:"${action:=}"`

	RateLimitPolicy
}

// ruleKey is the identity a matched rule's protection state is keyed by: its
// service plus its index in the rule list, so two rules that share a service but
// differ by Match get separate state. The index is stable within one config
// generation, which is all the key needs to be.
func ruleKey(service string, index int) string {
	return service + "#" + strconv.Itoa(index)
}
