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

package resilience

import (
	"sort"
	"strings"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/errutil"
)

// ServiceScopeAction is the reserved action selector that means "the whole
// service". Every rule names a scope through its Action field: this marker for
// the service level, a real exact/prefix for one interface. A real action is
// never spelled "-", so the marker cannot be confused with one.
const ServiceScopeAction = "-"

// isServiceScope reports whether a selects the service level.
func isServiceScope(a chain.ActionMatch) bool {
	return a.Exact == ServiceScopeAction
}

// specificity orders REAL action selectors: exact beats any prefix, a longer
// prefix beats a shorter one, and a larger metadata selector beats a smaller
// one. Service-scope rules are handled by rank so a real action always wins.
type specificity struct {
	rank      int // 0 = service scope, 2 = real action
	exact     bool
	prefixLen int
	matchLen  int
}

func specOf(a chain.ActionMatch, m matcher.ExactMap) specificity {
	if isServiceScope(a) {
		return specificity{rank: 0}
	}
	return specificity{rank: 2, exact: a.Exact != "", prefixLen: len(a.Prefix), matchLen: len(m)}
}

func (s specificity) betterThan(o specificity) bool {
	if s.rank != o.rank {
		return s.rank > o.rank
	}
	if s.exact != o.exact {
		return s.exact
	}
	if s.prefixLen != o.prefixLen {
		return s.prefixLen > o.prefixLen
	}
	return s.matchLen > o.matchLen
}

// validateSelector checks that a rule's Action names exactly one scope.
func validateSelector(a chain.ActionMatch) error {
	switch {
	case a.Exact != "" && a.Prefix != "":
		return errutil.Explain(nil, "resilience: action selector sets both exact and prefix")
	case a.Exact == "" && a.Prefix == "":
		return errutil.Explain(nil, "resilience: rule without an action selector (use %q for a service scope)", ServiceScopeAction)
	}
	return nil
}

// ValidateLimits reports whether s can serve.
//
// Limits accept both scopes: a [ServiceScopeAction] rule (no Match, may carry a
// label-scoped retry budget) and real-action rules (Match allowed, budget
// rejected — a budget is per label). A real action always wins over the service
// marker.
func ValidateLimits(s LimitsSettings) error {
	seen := make(map[string]struct{}, len(s.Rules))
	var real []LimitsRule
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service == "" {
			return errutil.Explain(nil, "resilience: rule without a service")
		}
		if err := validateSelector(r.Action); err != nil {
			return err
		}
		if isServiceScope(r.Action) {
			if len(r.Match) != 0 {
				return errutil.Explain(nil, "resilience: a service-scope rule for %q must not carry a match selector", r.Service)
			}
		} else if r.Retry.RetryBudget != 0 {
			return errutil.Explain(nil, "resilience: retry-budget is label-scoped and cannot be set on the action rule %q of %q",
				r.Action, r.Service)
		}
		key := ruleIdentity(r.Service, r.Action, r.Match)
		if _, dup := seen[key]; dup {
			return errutil.Explain(nil, "resilience: duplicate rule for service %q and action %+v", r.Service, r.Action)
		}
		seen[key] = struct{}{}
		if !isServiceScope(r.Action) {
			real = append(real, *r)
		}
	}
	return validateNoAmbiguity(real)
}

// ValidateBreaker reports whether every breaker policy in s can be materialized.
//
// Breakers accept both scopes. A service rule WINS OUTRIGHT over the
// interface-level rules of its service, so the two never compete; interface
// rules are consulted only when the service has no service rule.
func ValidateBreaker(s BreakerSettings) error {
	if err := validateBreakerPolicy(s.Default); err != nil {
		return errutil.Explain(err, "resilience: default breaker")
	}
	seen := make(map[string]struct{}, len(s.Rules))
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service == "" {
			return errutil.Explain(nil, "resilience: rule without a service")
		}
		if err := validateSelector(r.Action); err != nil {
			return err
		}
		if err := validateBreakerPolicy(r.BreakerPolicy); err != nil {
			return errutil.Explain(err, "resilience: breaker rule %q of %q", r.Action, r.Service)
		}
		key := ruleIdentity(r.Service, r.Action, nil)
		if _, dup := seen[key]; dup {
			return errutil.Explain(nil, "resilience: duplicate breaker rule for service %q and action %+v", r.Service, r.Action)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// ValidateRateLimit reports whether s can serve. Rate limiting is
// INTERFACE-scoped only: the service marker is rejected.
func ValidateRateLimit(s RateLimitSettings) error {
	seen := make(map[string]struct{}, len(s.Rules))
	for i := range s.Rules {
		r := &s.Rules[i]
		if r.Service == "" {
			return errutil.Explain(nil, "resilience: rule without a service")
		}
		if isServiceScope(r.Action) {
			return errutil.Explain(nil, "resilience: rate limiting is interface-scoped; the service marker %q is not accepted", ServiceScopeAction)
		}
		if err := validateSelector(r.Action); err != nil {
			return err
		}
		key := ruleIdentity(r.Service, r.Action, nil)
		if _, dup := seen[key]; dup {
			return errutil.Explain(nil, "resilience: duplicate rate-limit rule for service %q and action %+v", r.Service, r.Action)
		}
		seen[key] = struct{}{}
	}
	return nil
}

// validateNoAmbiguity rejects two real-action rules of one service that tie on
// specificity and can match the same call: which one wins would otherwise depend
// on list order.
func validateNoAmbiguity(rules []LimitsRule) error {
	for i := range rules {
		a := rules[i]
		for j := 0; j < i; j++ {
			b := rules[j]
			if a.Service != b.Service || specOf(a.Action, a.Match) != specOf(b.Action, b.Match) {
				continue
			}
			if actionsCanCoMatch(a.Action, b.Action) && matchCanCoMatch(a.Match, b.Match) {
				return errutil.Explain(nil,
					"resilience: ambiguous rules for service %q with equal specificity (action %+v vs %+v)",
					a.Service, b.Action, a.Action)
			}
		}
	}
	return nil
}

func actionsCanCoMatch(a, b chain.ActionMatch) bool {
	switch {
	case a.Exact != "" && b.Exact != "":
		return a.Exact == b.Exact
	case a.Exact != "":
		return strings.HasPrefix(a.Exact, b.Prefix)
	case b.Exact != "":
		return strings.HasPrefix(b.Exact, a.Prefix)
	case a.Prefix != "" && b.Prefix != "":
		return strings.HasPrefix(a.Prefix, b.Prefix) || strings.HasPrefix(b.Prefix, a.Prefix)
	default:
		return true
	}
}

func matchCanCoMatch(a, b matcher.ExactMap) bool {
	for k, v := range a {
		if bv, ok := b[k]; ok && bv != v {
			return false
		}
	}
	return true
}

// ruleIdentity renders a rule's selector as a stable string for the duplicate
// check.
func ruleIdentity(service string, action chain.ActionMatch, match matcher.ExactMap) string {
	return service + "\x00" + action.Exact + "\x00" + action.Prefix + "\x00" + canonicalSelector(match)
}

// canonicalSelector renders a selector in key order, so two selectors that name
// the same keys and values compare equal however their maps were laid out.
func canonicalSelector(s matcher.ExactMap) string {
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
