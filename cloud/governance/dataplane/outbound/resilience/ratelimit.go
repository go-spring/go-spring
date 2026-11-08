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

// ratelimit.go is the outbound view of the shared rate-limit stage: the manager
// resolves a rule's limiter KIND to a [ratelimit.Provider], and the executor
// keeps one [ratelimit.Limiter] per rate key, built by that factory. The
// counting algorithms, the [ratelimit.Policy] vocabulary and both factory
// families live in go-spring.org/cloud/governance/capability/ratelimit, a leaf
// package shared with inbound admission, so one request is charged the same way
// in both directions.
//
// Rate limiting is a stage of a protected call, not an API of its own: the call
// either gets its budget or comes back as [chain.ErrRateLimited]. By default
// the kind is a process-local algorithm, so the counter lives in the executor;
// naming a shared store's kind (a Redis factory bean) makes the same rule cover
// every replica.

package resilience

import (
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
)

// RateLimitSettings is the rate-limit half of the outbound document. This type
// IS the document — the control-plane rate-limit root binds it and hands it back
// unchanged, so there is no separate binding twin. A malformed selector is not
// rejected here: documents are platform-generated, and a caller that must
// validate one does so in the document-validation layer.
type RateLimitSettings struct {
	// Services groups the rules by the label they govern. Duplicate groups are
	// order-shadowed: the first group naming a label wins. There is no implicit
	// default: a service with no matching rule is not limited at all.
	Services []RateLimitService `value:"${services:=}"`
}

// RateLimitService is one label's policy block: every rule inside it governs
// the same service.
type RateLimitService struct {
	// Service is the label these rules apply to, exact-compare. Required.
	Service string `value:"${service:=}" expr:"$ != ''"`

	// Rules are the limits for this service.
	Rules []RateLimitRule `value:"${rules:=}"`
}

// RateLimitRule is one rule inside a [RateLimitService].
//
// Action is either a real exact/prefix selector, which limits one interface,
// or [matcher.ServiceScopeAction] ("-"), which limits the whole service. A
// service-scope rule is the FALLBACK: it governs a call only when no
// exact/prefix rule of that service matches.
//
// MetadataMatch is deliberately absent. A limit is keyed by the rule (service
// + action); selecting rules by metadata would make several rules share one
// bucket, and giving each value its own bucket needs the value in the key and a
// shared store — a different model, not a matcher field. Fine per-call tuning
// belongs to the timeout/retry and fault rules.
type RateLimitRule struct {
	// ActionMatch selects the scope: [matcher.ServiceScopeAction] for the whole
	// service, or a real exact/prefix for one interface. Required.
	ActionMatch matcher.ActionMatch `value:"${action:=}"`

	ratelimit.Policy
}

// findRule answers the rate-limit half of the call plan ([Executor.planFor]).
//
// Matching follows the timeout/retry precedence: a service-scope rule is the
// FALLBACK, governing only calls no exact/prefix rule of that service matches.
// The breaker deliberately differs — there a service rule wins outright. The
// precedence is per stage; read it from each stage's findRule rather than
// assuming one rule for all.
//
// The second result reports whether any rule matched; false means the stage is
// off for this call. The counter key is derived by the caller from the rule's
// selector (see [ruleKey]).
func (s *RateLimitSettings) findRule(op chain.Operation) (RateLimitRule, bool) {
	for i := range s.Services {
		if s.Services[i].Service == op.Service {
			// The first group naming a label wins; a duplicate group is
			// order-shadowed, not merged.
			return s.Services[i].findRule(op)
		}
	}
	return RateLimitRule{}, false
}

// findRule selects one rule of this service. Specificity order lives in
// [matcher.ActionMatch.BetterThan]: a real action always beats the service
// marker, so no special case is needed here.
func (s *RateLimitService) findRule(op chain.Operation) (RateLimitRule, bool) {
	best := -1
	var bestMatch matcher.ActionMatch
	for i := range s.Rules {
		r := &s.Rules[i]
		if !r.ActionMatch.Matches(op.Request.Action) {
			continue
		}
		if best < 0 || r.ActionMatch.BetterThan(bestMatch) {
			best, bestMatch = i, r.ActionMatch
		}
	}
	if best < 0 {
		return RateLimitRule{}, false
	}
	return s.Rules[best], true
}
