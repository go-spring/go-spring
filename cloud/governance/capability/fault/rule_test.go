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

package fault

import (
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// TestActionMatch_Matches pins the one-of convention: the zero value matches
// everything, Exact is equality, Prefix covers a subtree, and Exact wins when
// both are set.
func TestActionMatch_Matches(t *testing.T) {
	cases := []struct {
		m      ActionMatch
		action string
		want   bool
	}{
		{ActionMatch{}, "", true},                       // no condition: matches all
		{ActionMatch{}, "GET /users/1", true},           //
		{ActionMatch{Exact: "insert"}, "insert", true},  //
		{ActionMatch{Exact: "insert"}, "update", false}, //
		{ActionMatch{Prefix: "GET /users"}, "GET /users/1", true},
		{ActionMatch{Prefix: "GET /users"}, "POST /users", false},
		{ActionMatch{Exact: "insert", Prefix: "in"}, "insert", true},    // exact wins
		{ActionMatch{Exact: "insert", Prefix: "in"}, "internal", false}, //
	}
	for _, tc := range cases {
		assert.That(t, tc.m.Matches(tc.action)).Equal(tc.want)
	}
}

// TestRule_Matches pins the three positive axes in [Rule.Matches]: Service by
// exact compare (no catch-all), Action by the one-of match, Metadata by
// subset — a call may carry dimensions the rule does not name, and a key the
// rule names must be present with an equal value.
func TestRule_Matches(t *testing.T) {
	base := Rule{
		Name:          "r",
		Service:       "svc",
		ActionMatch:   ActionMatch{Prefix: "GET /users"},
		MetadataMatch: matcher.ExactMap{"tenant": "acme"},
	}

	acme := Target{Service: "svc", Action: "GET /users/1", Metadata: map[string]string{"tenant": "acme"}}
	assert.That(t, base.Matches(acme)).True()

	// The call may carry MORE dimensions than the rule names (subset semantics).
	rich := acme
	rich.Metadata["region"] = "cn"
	assert.That(t, base.Matches(rich)).True()

	// Each axis can veto on its own.
	assert.That(t, base.Matches(Target{Service: "other", Action: "GET /users/1", Metadata: acme.Metadata})).False()
	assert.That(t, base.Matches(Target{Service: "svc", Action: "POST /users", Metadata: acme.Metadata})).False()
	assert.That(t, base.Matches(Target{Service: "svc", Action: "GET /users/1", Metadata: map[string]string{"tenant": "other"}})).False()
	assert.That(t, base.Matches(Target{Service: "svc", Action: "GET /users/1"})).False() // named key absent
}

// TestRule_Roll pins the dose semantics: Latency applies to EVERY attempt
// regardless of Rate, the error draws at Rate (0 never, 1 always) and carries
// the rule's identity, kind and class onto the Result.
func TestRule_Roll(t *testing.T) {
	// Rate 0 + latency: the delay is drawn on every attempt, never an error.
	r := Rule{Name: "slow", Service: "svc", Latency: 10 * time.Millisecond, Error: ErrorReset, Class: ClassBusiness}
	for range 20 {
		o := r.roll()
		assert.That(t, o.Fired()).True()
		assert.That(t, o.Inject).False()
		assert.Number(t, o.Delay).Equal(10 * time.Millisecond)
	}

	// Rate 1: every attempt injects, carrying the identity and the declared kind/class.
	r2 := Rule{Name: "dead", Service: "svc", Rate: 1, Error: ErrorTimeout, Class: ClassPermanent}
	for range 20 {
		o := r2.roll()
		assert.That(t, o.Inject).True()
		assert.String(t, o.Name).Equal("dead")
		assert.String(t, o.Error).Equal(ErrorTimeout)
		assert.String(t, o.Class).Equal(ClassPermanent)
	}

	// An unarmed rule rolls a zero Result: nothing it governs is affected.
	o := Rule{Name: "idle", Service: "svc"}.roll()
	assert.That(t, o.Fired()).False()
}

// TestRule_LatencyJitterBounds: with LatencyJitter J, every draw lands in
// [Latency-J, Latency+J] clamped to ≥ 0; the midpoint over many draws stays
// near Latency.
func TestRule_LatencyJitterBounds(t *testing.T) {
	const base, jitter = 100 * time.Millisecond, 80 * time.Millisecond
	r := Rule{Name: "jitter", Latency: base, LatencyJitter: jitter}
	var sum time.Duration
	for i := 0; i < 200; i++ {
		d := r.roll()
		assert.That(t, d.Delay >= base-jitter).True()
		assert.That(t, d.Delay <= base+jitter).True()
		sum += d.Delay
	}
	mid := sum / 200
	assert.That(t, mid > base-40*time.Millisecond && mid < base+40*time.Millisecond).True()
}

// TestRule_BindsMatchAndAction pins that a rule's Match and Action — fault's own
// types, not chain's — bind from the flattened properties an operator writes.
// Both are reached by reflect kind rather than a registered converter, so what
// matters is the spelling: match.<key> per metadata dimension, action.exact /
// action.prefix for the interface.
func TestRule_BindsMatchAndAction(t *testing.T) {
	storage := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{
		"rules[0].name":          "gin-users",
		"rules[0].service":       "gin",
		"rules[0].action.exact":  "GET /users/{id}",
		"rules[0].match.tenant":  "acme",
		"rules[0].match.region":  "cn",
		"rules[0].rate":          "1",
		"rules[1].name":          "redis",
		"rules[1].service":       "redis:cache",
		"rules[1].action.prefix": "GET /users",
	}))
	var c Config
	assert.Error(t, conf.Bind(storage, &c)).Nil()

	assert.Number(t, len(c.Rules)).Equal(2)
	r0 := c.Rules[0]
	assert.String(t, r0.Service).Equal("gin")
	assert.String(t, r0.ActionMatch.Exact).Equal("GET /users/{id}")
	assert.String(t, r0.ActionMatch.Prefix).Equal("")
	assert.String(t, r0.MetadataMatch["tenant"]).Equal("acme")
	assert.String(t, r0.MetadataMatch["region"]).Equal("cn")
	assert.Number(t, int(c.Rules[1].Rate)).Equal(0)

	// The bound rule governs exactly the call it names: same service, same
	// interface, matching metadata — and nothing else.
	assert.That(t, r0.Matches(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).True()
	assert.That(t, r0.Matches(Target{Service: "gin", Action: "GET /users/{id}",
		Metadata: map[string]string{"tenant": "acme"}})).False() // region missing
	assert.That(t, r0.Matches(Target{Service: "gin", Action: "POST /orders",
		Metadata: map[string]string{"tenant": "acme", "region": "cn"}})).False()
}

// TestRule_ExprRejectsBadValues pins the single-field boundary on the path
// that owns it: the binder evaluates each field's expr tag, so a bad dose, an
// unknown vocabulary word or an empty name never becomes a Rule at all.
func TestRule_ExprRejectsBadValues(t *testing.T) {
	bad := []map[string]any{
		{"rules[0].name": "", "rules[0].rate": "0.5"},
		{"rules[0].name": "r", "rules[0].rate": "1.5"},
		{"rules[0].name": "r", "rules[0].rate": "-0.1"},
		{"rules[0].name": "r", "rules[0].latency": "-1ms"},
		{"rules[0].name": "r", "rules[0].latency-jitter": "-1ms"},
		{"rules[0].name": "r", "rules[0].max-duration": "-1s"},
		{"rules[0].name": "r", "rules[0].max-affected": "-1"},
		{"rules[0].name": "r", "rules[0].error": "boom"},
		{"rules[0].name": "r", "rules[0].error": ""},
		{"rules[0].name": "r", "rules[0].class": "sometimes"},
		{"rules[0].name": "r", "rules[0].class": ""},
	}
	for _, props := range bad {
		var c Config
		assert.Error(t, conf.Bind(flatten.NewPropertiesStorage(flatten.MapProperties(props)), &c)).NotNil()
	}
}
