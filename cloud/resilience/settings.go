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
// retry, circuit breaking, bulkhead, rate limiting and per-attempt timeout.
//
// This package is CLIENT-ONLY. Outbound calls run through a [chain.Executor]
// held for the client's lifetime; inbound requests run through
// [go-spring.org/cloud/guard]'s per-instance [guard.Guard], which has its own
// policy vocabulary and engine. There is no inbound manager, no inbound
// [Driver] method and no inbound policy type here — the two directions do not
// share a shape.
//
// The manager is container-free: it imports no IoC concept and holds no global.
// Its dependency on the governance center is inverted into plain types (Apply
// takes the pushed half), so this package never imports cloud/governance and
// the center never reaches into this package's internals. The gs wiring
// (providing the manager as a bean, injecting it into the starters) lives in
// starter.go.
//
// The BACKEND, unlike the policies, is not pushed: it is fixed at construction —
// the [Driver] the container selected from ${spring.resilience.driver}, a
// startup decision, since switching it drops every executor's state.
//
// # A push stops here
//
// Each Apply publishes a new snapshot and that is the whole of it: it does not
// walk the registry and it does not reach into any executor. Each executor holds
// a [PolicySource] — the client manager — and notices on its next call that the
// generation behind its label moved, at which point it rebuilds its materialized
// state wholesale (see [defaultExecutor.snapshot] and [serverEngine.snapshot]).
// The registry is therefore an identity table (one executor per label) and a
// shutdown list, not a change-propagation mechanism.
//
// One label maps to exactly ONE executor per direction. That is not an
// optimization but a semantic requirement: a breaker, limiter and bulkhead are
// per-RESOURCE state, so two objects sharing a label (a rebuilt client, two pools
// for one service name) must share the breaker rather than each getting its own
// and effectively doubling the trip threshold.
package resilience

import (
	"strconv"

	"go-spring.org/cloud/chain"
)

// Settings is the OUTBOUND half of the governance document, PUSHED whole: the
// center hands it over and this package does the matching. The inbound half
// lives in [go-spring.org/cloud/guard.Settings].
//
// It carries policies only, never the backend: which [Driver] builds the
// executors is a startup decision, not part of the tunable document.
//
// The zero Settings configures nothing: every label resolves a zero policy, which
// every caller reads as the pass-through. There is no arming flag — an empty
// document is a pass-through, and so is an absent one.
type Settings struct {
	// Client is the outbound half: what to apply when this process CALLS something.
	Client ClientSettings
}

// ClientSettings is the pushed outbound half of the document: the policy every
// service runs under, plus per-service overrides. See [Settings.Client].
type ClientSettings struct {
	// Default is the policy applied to every service label no [ClientRule]
	// matches. Zero means the pass-through, which is what a deployment that
	// governs only inbound traffic looks like.
	Default ClientPolicy

	// Rules are the per-service overrides: the first Rule whose Service equals the
	// call's and whose Match the call's metadata satisfies wins. An empty Service
	// matches NOTHING (it is not a catch-all — use Default for that). Several
	// rules may share a Service as long as their Match differs; the center rejects
	// a document whose (Service, Match) pairs collide before it reaches here.
	Rules []ClientRule
}

// ClientRule is one per-service outbound override. See [ClientSettings.Rules].
type ClientRule struct {
	// Service is the service label this Rule matches, exact-compare — the identity
	// a client starter passes when resolving an outbound executor.
	Service string

	// Match narrows the rule to calls whose metadata satisfies it (subset-match).
	// Empty matches every call of the service.
	Match chain.Selector

	// Action narrows the rule to calls whose Request.Action it matches (the
	// interface being invoked). Empty matches every action. Two rules may share a
	// Service as long as Match and Action differ; the first whose conditions the
	// call satisfies wins.
	Action chain.ActionMatch

	// ClientPolicy is what the matched service runs under; it fully REPLACES
	// Default rather than merging field-wise.
	ClientPolicy
}

// ruleKey is the identity a matched rule's protection state is keyed by: its
// service plus its index in the rule list, so two rules that share a service but
// differ by Match get separate state. The index is stable within one config
// generation, which is all the key needs to be.
func ruleKey(service string, index int) string {
	return service + "#" + strconv.Itoa(index)
}
