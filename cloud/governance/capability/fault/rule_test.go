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
	"go-spring.org/stdlib/testing/assert"
)

// TestActionMatch_Matches pins the one-of convention: the zero value matches
// everything, Exact is equality, Prefix covers a subtree, and Exact wins when
// both are set.
func TestActionMatch_Matches(t *testing.T) {
	cases := []struct {
		m      matcher.ActionMatch
		action string
		want   bool
	}{
		{matcher.ActionMatch{}, "", true},                       // no condition: matches all
		{matcher.ActionMatch{}, "GET /users/1", true},           //
		{matcher.ActionMatch{Exact: "insert"}, "insert", true},  //
		{matcher.ActionMatch{Exact: "insert"}, "update", false}, //
		{matcher.ActionMatch{Prefix: "GET /users"}, "GET /users/1", true},
		{matcher.ActionMatch{Prefix: "GET /users"}, "POST /users", false},
		{matcher.ActionMatch{Exact: "insert", Prefix: "in"}, "insert", true},    // exact wins
		{matcher.ActionMatch{Exact: "insert", Prefix: "in"}, "internal", false}, //
	}
	for _, tc := range cases {
		assert.That(t, tc.m.Matches(tc.action)).Equal(tc.want)
	}
}

// TestRuleCore_Matches pins the three positive axes: Service by exact compare
// (no catch-all), Action by the one-of match, Metadata by subset — a call may
// carry dimensions the rule does not name, and a key the rule names must be
// present with an equal value.
func TestRuleCore_Matches(t *testing.T) {
	base := RuleCore{
		Name:          "r",
		Service:       "svc",
		ActionMatch:   matcher.ActionMatch{Prefix: "GET /users"},
		MetadataMatch: map[string]matcher.StringMatch{"tenant": {Exact: "acme"}},
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

// TestRuleCore_Dose pins the dose semantics: Latency applies to EVERY attempt
// regardless of Rate, and the effect draws at Rate (0 never, 1 always).
func TestRuleCore_Dose(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)

	// Rate 0 + latency: the delay is drawn on every attempt, never an effect.
	r := RuleCore{Name: "slow", Service: "svc", Latency: 10 * time.Millisecond}
	b := &Budget{}
	for range 20 {
		d := r.Dose(b, now)
		assert.That(t, d.Inject).False()
		assert.Number(t, d.Delay).Equal(10 * time.Millisecond)
	}

	// Rate 1: every attempt injects.
	r2 := RuleCore{Name: "dead", Service: "svc", Rate: 1}
	b2 := &Budget{}
	for range 20 {
		d := r2.Dose(b2, now)
		assert.That(t, d.Inject).True()
		assert.Number(t, d.Delay).Equal(time.Duration(0))
	}

	// An unarmed rule draws nothing.
	d := RuleCore{Name: "idle", Service: "svc"}.Dose(&Budget{}, now)
	assert.That(t, d.Inject).False()
	assert.Number(t, d.Delay).Equal(time.Duration(0))
}

// TestRuleCore_StartAfterAndWindows pins the time axis: nothing fires before
// StartAfter, and an On/Off pair alternates eligibility from that anchor.
func TestRuleCore_StartAfterAndWindows(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)

	r := RuleCore{Name: "delayed", Service: "svc", Rate: 1, StartAfter: time.Second}
	b := &Budget{}
	assert.That(t, r.Dose(b, start).Inject).False() // starts the clock, not the fire
	assert.That(t, r.Dose(b, start.Add(500*time.Millisecond)).Inject).False()
	assert.That(t, r.Dose(b, start.Add(time.Second)).Inject).True()

	// On 10s / Off 10s from StartAfter.
	r2 := RuleCore{Name: "flaky", Service: "svc", Rate: 1, On: 10 * time.Second, Off: 10 * time.Second}
	b2 := &Budget{}
	assert.That(t, r2.Dose(b2, start).Inject).True() // phase 0, inside On
	assert.That(t, r2.Dose(b2, start.Add(9*time.Second)).Inject).True()
	assert.That(t, r2.Dose(b2, start.Add(10*time.Second)).Inject).False() // Off
	assert.That(t, r2.Dose(b2, start.Add(19*time.Second)).Inject).False()
	assert.That(t, r2.Dose(b2, start.Add(20*time.Second)).Inject).True() // next On
}

// TestRuleCore_Every pins the deterministic alternative to Rate: every Nth
// matching call fires, counted per rule identity.
func TestRuleCore_Every(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	r := RuleCore{Name: "nth", Service: "svc", Every: 3}
	b := &Budget{}
	want := []bool{false, false, true, false, false, true}
	for i, w := range want {
		if got := r.Dose(b, now).Inject; got != w {
			t.Fatalf("call %d: inject = %v, want %v", i+1, got, w)
		}
	}
}

// TestRuleCore_Validate pins the cross-field constraints: Rate and Every are
// alternatives, and On/Off is a pair.
func TestRuleCore_Validate(t *testing.T) {
	assert.Error(t, (&RuleCore{Name: "both", Rate: 0.5, Every: 2}).Validate()).NotNil()
	assert.Error(t, (&RuleCore{Name: "half", On: time.Second}).Validate()).NotNil()
	assert.Error(t, (&RuleCore{Name: "half", Off: time.Second}).Validate()).NotNil()
	assert.Error(t, (&RuleCore{Name: "ok", Rate: 0.5}).Validate()).Nil()
	assert.Error(t, (&RuleCore{Name: "ok", Every: 2}).Validate()).Nil()
	assert.Error(t, (&RuleCore{Name: "ok", On: time.Second, Off: time.Second}).Validate()).Nil()
}

// TestRuleCore_LatencyJitterBounds: with LatencyJitter J, every draw lands in
// [Latency-J, Latency+J] clamped to ≥ 0; the midpoint over many draws stays
// near Latency.
func TestRuleCore_LatencyJitterBounds(t *testing.T) {
	const base, jitter = 100 * time.Millisecond, 80 * time.Millisecond
	r := RuleCore{Name: "jitter", Latency: base, LatencyJitter: jitter}
	b := &Budget{}
	now := time.Unix(1_700_000_000, 0)
	var sum time.Duration
	for range 200 {
		d := r.Dose(b, now)
		assert.That(t, d.Delay >= base-jitter).True()
		assert.That(t, d.Delay <= base+jitter).True()
		sum += d.Delay
	}
	mid := sum / 200
	assert.That(t, mid > base-40*time.Millisecond && mid < base+40*time.Millisecond).True()
}
