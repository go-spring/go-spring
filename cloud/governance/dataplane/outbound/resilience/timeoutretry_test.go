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
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/source"
	"go-spring.org/stdlib/testing/assert"
)

// TestLimitsServiceRequiredAtBind pins the single-field rule that lives in the
// expr tag rather than in Validate: a rule without a service names nothing, so
// binding rejects it.
func TestLimitsServiceRequiredAtBind(t *testing.T) {
	_, err := source.Parse[TimeoutRetrySettings]("limits.properties", []byte(`
services[0].rules[0].action.exact=GET /x
services[0].rules[0].timeout=1s
`), "")
	assert.Error(t, err).NotNil()

	// The same document with the service named binds.
	_, err = source.Parse[TimeoutRetrySettings]("limits.properties", []byte(`
services[0].service=svc
services[0].rules[0].action.exact=GET /x
services[0].rules[0].timeout=1s
`), "")
	assert.Error(t, err).Nil()
}

// TestValidateLimitsScopeRules pins the document constraints that are not
// single-field expr tags: the Action chooses the scope, and the scope decides
// which companion fields are legal.
func TestValidateLimitsScopeRules(t *testing.T) {
	// A service-scope rule covers the whole label, so a metadata match is
	// meaningless.
	assert.Error(t, (TimeoutRetrySettings{Services: []TimeoutRetryService{{
		Service: "svc",
		Rules: []TimeoutRetryRule{{
			ActionMatch:   matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			MetadataMatch: map[string]matcher.StringMatch{"tenant": {Exact: "core"}},
		}},
	}}}).Validate()).Matches("must not carry a match selector")

	// The retry budget is label-scoped: it cannot sit on one interface.
	assert.Error(t, (TimeoutRetrySettings{Services: []TimeoutRetryService{{
		Service: "svc",
		Rules: []TimeoutRetryRule{{
			ActionMatch:  matcher.ActionMatch{Prefix: "GET /"},
			TimeoutRetry: TimeoutRetry{Retry: Retry{RetryBudget: 4}},
		}},
	}}}).Validate()).Matches("retry-budget is label-scoped")
}

// TestLimitsIsZero pins that a single configured knob is enough to make the
// limits non-zero.
func TestLimitsIsZero(t *testing.T) {
	assert.That(t, (TimeoutRetry{Timeout: time.Second}).IsZero()).False()
}
