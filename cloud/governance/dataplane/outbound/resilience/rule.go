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
	"fmt"
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

func specOf(a chain.ActionMatch, matchLen int) specificity {
	if isServiceScope(a) {
		return specificity{rank: 0}
	}
	return specificity{rank: 2, exact: a.Exact != "", prefixLen: len(a.Prefix), matchLen: matchLen}
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
			if a.Service != b.Service || specOf(a.Action, len(a.Match)) != specOf(b.Action, len(b.Match)) {
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

// matchCanCoMatch reports whether two metadata matches can both hold for one
// call: they cannot only when they name the same key with conditions no value
// satisfies (overlap decides that pairwise).
func matchCanCoMatch(a, b map[string]matcher.StringMatch) bool {
	for k, ma := range a {
		if mb, ok := b[k]; ok && !overlap(ma, mb) {
			return false
		}
	}
	return true
}

// overlap reports whether some value satisfies both conditions. It is exact
// for every op pair: exact pins the value, while the containment ops can
// always be satisfied together by splicing their operands, so only exact and a
// mismatched exact rule a pair out.
func overlap(a, b matcher.StringMatch) bool {
	switch {
	case a.Exact != "" && b.Exact != "":
		return a.Exact == b.Exact
	case a.Exact != "":
		return holds(b, a.Exact)
	case b.Exact != "":
		return holds(a, b.Exact)
	default:
		return true
	}
}

// holds reports whether v satisfies m, over the exported fields only (the
// fail-closed default lives in the matcher package; here every condition has
// passed validation, so exactly one op is set).
func holds(m matcher.StringMatch, v string) bool {
	switch {
	case m.Exact != "":
		return v == m.Exact
	case m.Prefix != "":
		return strings.HasPrefix(v, m.Prefix)
	case m.Suffix != "":
		return strings.HasSuffix(v, m.Suffix)
	default:
		return strings.Contains(v, m.Contains)
	}
}

// ruleIdentity renders a rule's selector as a stable string for the duplicate
// check. The match renders in key order, so identical selectors compare equal
// however their maps were laid out.
func ruleIdentity(service string, action chain.ActionMatch, match map[string]matcher.StringMatch) string {
	id := service + "\x00" + action.Exact + "\x00" + action.Prefix
	if len(match) == 0 {
		return id
	}
	keys := make([]string, 0, len(match))
	for k := range match {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		id += "\x00" + fmt.Sprintf("%s=%+v", k, match[k])
	}
	return id
}
