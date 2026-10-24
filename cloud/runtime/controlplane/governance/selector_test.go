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

package governance

import (
	"strings"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	"go-spring.org/cloud/runtime/dataplane/kernel/matcher"
	"go-spring.org/stdlib/testing/assert"
)

// selectorDoc is a full document whose only outbound rule narrows its endpoints
// by metadata — the shape an operator publishes to move traffic between
// versions without a redeploy.
const selectorDoc = `
spring.governance:
  client:
    rules:
      - service: redis:cache
        timeout: 100ms
        selector:
          - key: version
            kind: prefix
            value: v2.
          - key: zone
            kind: exact
            value: cn-north-1a
`

// TestParseSelector pins that a published document binds into the typed
// selector: the dynamic rule travels as plain data, so it needs no Go code on
// the receiving side.
func TestParseSelector(t *testing.T) {
	cfg, err := Parse("governance.yaml", []byte(selectorDoc), "")
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Client.Rules) != 1 {
		t.Fatalf("rules = %+v, want one", cfg.Client.Rules)
	}
	sel := cfg.Client.Rules[0].Selector
	if len(sel) != 2 {
		t.Fatalf("selector = %+v, want two terms", sel)
	}
	assert.That(t, sel[0].Key).Equal("version")
	assert.That(t, sel[0].Op).Equal(matcher.Prefix)
	assert.That(t, sel[0].Value).Equal("v2.")
	assert.That(t, sel[1].Key).Equal("zone")
	assert.That(t, sel[1].Op).Equal(matcher.Exact)
}

// TestParseSelectorRequiresTermFields pins that every term field is required at
// bind time: there is no unnamed key and no default comparison or operand, so a
// document that omits one is rejected instead of building a term that can never
// match or silently meaning "the key exists".
func TestParseSelectorRequiresTermFields(t *testing.T) {
	const missingKind = `
spring.governance:
  client:
    rules:
      - service: redis:cache
        selector:
          - key: version
            value: v2.
`
	if _, err := Parse("governance.yaml", []byte(missingKind), ""); err == nil {
		t.Fatal("term without kind accepted")
	}

	const emptyKey = `
spring.governance:
  client:
    rules:
      - service: redis:cache
        selector:
          - key: ""
            kind: prefix
            value: v2.
`
	if _, err := Parse("governance.yaml", []byte(emptyKey), ""); err == nil {
		t.Fatal("term with empty key accepted")
	}

	const badKind = `
spring.governance:
  client:
    rules:
      - service: redis:cache
        selector:
          - key: version
            kind: regex
            value: ".*"
`
	if _, err := Parse("governance.yaml", []byte(badKind), ""); err == nil {
		t.Fatal("term with unknown kind accepted")
	}

	const missingValue = `
spring.governance:
  client:
    rules:
      - service: redis:cache
        selector:
          - key: version
            kind: exact
`
	if _, err := Parse("governance.yaml", []byte(missingValue), ""); err == nil {
		t.Fatal("term without value accepted")
	}

	// An empty operand is meaningful only for exact, and must be written out
	// instead of arrived at by omission.
	const explicitEmpty = `
spring.governance:
  client:
    rules:
      - service: redis:cache
        selector:
          - key: version
            kind: exact
            value: ""
`
	cfg, err := Parse("governance.yaml", []byte(explicitEmpty), "")
	if err != nil {
		t.Fatalf("explicit empty value rejected: %v", err)
	}
	if got := cfg.Client.Rules[0].Selector[0].Value; got != "" {
		t.Fatalf("value = %q, want empty", got)
	}
}

// policyExprDoc exercises every expr tag on the policy/fault vocabulary. The
// expressions live in struct tags, so the compiler cannot check them; this test
// binds a representative document to catch a malformed expression (e.g. a
// duration compared to an int, or a named type compared to a string literal)
// before it reaches an operator's config.
const policyExprDoc = `
spring.governance:
  client:
    default:
      timeout: 100ms
      retry:
        max-retries: 2
        initial-interval: 10ms
        multiplier: 2
        max-interval: 1s
        randomization-factor: 0.5
        retry-budget: 10
      rate-limit: 10
      burst: 2
      rate-limit-max-wait: 1ms
      algorithm: sliding-window
      window: 1s
      rate-limit-warm-up-duration: 1s
      warm-up-cold-factor: 2
      breaker-strategy: error-rate
      error-rate-threshold: 0.5
      min-requests: 10
      breaker-window: 10s
      slow-call-duration-threshold: 1s
      slow-call-rate-threshold: 0.5
      half-open-requests: 1
      half-open-timeout: 1s
      open-duration: 1s
      error-threshold: 5
    fault:
      rules:
        - rate: 0.5
          latency: 10ms
          latency-jitter: 1ms
          error: timeout
          scope: loadtest
          max-duration: 1m
          max-affected: 100
    rules:
      - service: redis:cache
        timeout: 1s
  server:
    default:
      rate-limit: 10
      burst: 2
      rate-limit-max-wait: 1ms
      algorithm: token-bucket
      window: 1s
      rate-limit-warm-up-duration: 1s
      warm-up-cold-factor: 2
      max-concurrent: 4
      bulkhead-queue-duration: 1ms
      timeout: 1s
`

// TestParse_PolicyExprTags pins that the documented values of the policy and
// fault vocabulary bind and satisfy their expr tags, and that an out-of-domain
// value is rejected at bind time.
func TestParse_PolicyExprTags(t *testing.T) {
	cfg, err := Parse("governance.yaml", []byte(policyExprDoc), "")
	if err != nil {
		t.Fatalf("documented policy rejected: %v", err)
	}
	if got := cfg.Client.Default.Retry.RandomizationFactor; got != 0.5 {
		t.Fatalf("randomization-factor = %v, want 0.5", got)
	}
	if got := cfg.Client.Fault.Rules[0].Scope; got != "loadtest" {
		t.Fatalf("scope = %q, want loadtest", got)
	}

	// An unknown enum is a silent-fallback hazard: scope would otherwise be
	// read as "all traffic" and inject faults into real traffic.
	bad := strings.Replace(policyExprDoc, "scope: loadtest", "scope: loadtes", 1)
	if _, err := Parse("governance.yaml", []byte(bad), ""); err == nil {
		t.Fatal("unknown scope accepted")
	}
}

// TestDispatchPushesSelector pins that the parsed rule reaches the endpoint
// selection authority: the label resolves to a selection carrying the selector,
// which is what makes the chain rebuild and narrow.
func TestDispatchPushesSelector(t *testing.T) {
	cfg, err := Parse("governance.yaml", []byte(selectorDoc), "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, lb, _, _ := newTestCenterWith(cfg)

	got, _ := lb.ResolveSelection("redis:cache")
	assert.Number(t, len(got.Selector)).Equal(2)
	assert.That(t, got.Selector[0].Value).Equal("v2.")

	// An unrelated label resolves the default, which carries no selector.
	other, _ := lb.ResolveSelection("gorm:mysql:primary")
	assert.Number(t, len(other.Selector)).Equal(0)
}

// TestValidateRejectsBadSelector pins that a malformed pushed rule is rejected
// whole — the fail-static contract — instead of reaching the request path.
func TestValidateRejectsBadSelector(t *testing.T) {
	c := newTestCenter(Config{})

	// The malformation is the cross-field rule Validate owns; key/op typos are
	// rejected earlier, by the field tags (see TestParseSelectorRequiresTermFields).
	bad := Config{Client: ClientConfig{Rules: []ClientRule{{
		Service: "redis:cache",
		Selection: loadbalance.Selection{Selector: matcher.Selector{
			{Key: "version", Op: matcher.Prefix, Value: ""},
		}},
	}}}}
	assert.Error(t, c.Validate(bad)).NotNil()

	good := Config{Client: ClientConfig{Rules: []ClientRule{{
		Service: "redis:cache",
		Selection: loadbalance.Selection{Selector: matcher.Selector{
			{Key: "version", Op: matcher.Prefix, Value: "v2."},
		}},
	}}}}
	assert.Error(t, c.Validate(good)).Nil()
}
