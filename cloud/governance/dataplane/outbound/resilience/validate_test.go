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
	"testing"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

// TestValidateLimitsScopeRules pins the document constraints that are not
// single-field expr tags: the Action chooses the scope, and the scope decides
// which companion fields are legal.
func TestValidateLimitsScopeRules(t *testing.T) {
	// A service-scope rule covers the whole label, so a metadata match is
	// meaningless.
	assert.Error(t, ValidateLimits(LimitsSettings{Rules: []LimitsRule{{
		Service: "svc",
		Action:  chain.ActionMatch{Exact: ServiceScopeAction},
		Match:   matcher.ExactMap{"tenant": "core"},
	}}})).Matches("must not carry a match selector")

	// The retry budget is label-scoped: it cannot sit on one interface.
	assert.Error(t, ValidateLimits(LimitsSettings{Rules: []LimitsRule{{
		Service: "svc",
		Action:  chain.ActionMatch{Prefix: "GET /"},
		Limits:  Limits{Retry: chain.RetryPolicy{RetryBudget: 4}},
	}}})).Matches("retry-budget is label-scoped")

	// An action selector is required.
	assert.Error(t, ValidateLimits(LimitsSettings{Rules: []LimitsRule{{Service: "svc"}}})).NotNil()
}

// TestValidateRateLimitInterfaceScoped pins the rate-limit contract: only an
// interface is limited, so the service marker is rejected.
func TestValidateRateLimitInterfaceScoped(t *testing.T) {
	assert.Error(t, ValidateRateLimit(RateLimitSettings{Rules: []RateLimitRule{{
		Service: "svc",
		Action:  chain.ActionMatch{Exact: ServiceScopeAction},
	}}})).Matches("interface-scoped")
}

// TestValidateBreakerRuleList pins the breaker document checks that are not
// single-field expr tags.
func TestValidateBreakerRuleList(t *testing.T) {
	// An action selector is required.
	assert.Error(t, ValidateBreaker(BreakerSettings{Rules: []BreakerRule{{Service: "svc"}}})).NotNil()

	// Two rules selecting the same calls would make list order decide the winner.
	assert.Error(t, ValidateBreaker(BreakerSettings{Rules: []BreakerRule{
		{Service: "svc", Action: chain.ActionMatch{Exact: ServiceScopeAction}},
		{Service: "svc", Action: chain.ActionMatch{Exact: ServiceScopeAction}},
	}})).Matches("duplicate breaker rule")
}
