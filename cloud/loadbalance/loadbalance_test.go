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

package loadbalance

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestRegistryBuildsBuiltins(t *testing.T) {
	for _, name := range []string{RoundRobin, LeastConn, ConsistentHash, Weighted, ZoneAware, Random, P2C} {
		b, err := New(name, NewParams(nil))
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}

	_, err := New("does-not-exist", NewParams(nil))
	assert.Error(t, err).Matches("no strategy registered")

	assert.That(t, Names()).Equal([]string{
		ConsistentHash, LeastConn, P2C, Random, RoundRobin, Weighted, ZoneAware,
	})
}

// TestRegisterRejectsBadContributions pins the guards every strategy file's init
// relies on: an empty name, a nil factory, and a name already taken are all
// programming errors — at init there is no error channel, and a duplicate would
// mean two packages claiming one strategy name.
//
// Only the REJECTION paths are tested: registering a new name would write the
// process-wide table that [TestRegistryBuildsBuiltins] pins exactly.
func TestRegisterRejectsBadContributions(t *testing.T) {
	assert.Panic(t, func() { Register("", buildRoundRobin) }, "empty name")
	assert.Panic(t, func() { Register("nil", nil) }, "nil factory")
	assert.Panic(t, func() { Register(RoundRobin, buildRoundRobin) }, "already registered")
}

// TestParamsPartition covers the strict-partition guarantee carried over from
// the typed Config: a parameter aimed at another strategy is rejected at
// construction, not silently dropped.
func TestParamsPartition(t *testing.T) {
	_, err := New(LeastConn, NewParams(map[string]string{ParamReplicas: "200"}))
	assert.Error(t, err).Matches("not understood by this strategy")

	_, err = New(ConsistentHash, NewParams(map[string]string{ParamZoneKey: "zone"}))
	assert.Error(t, err).Matches("not understood by this strategy")

	// A strategy's own parameters pass; the empty bag passes everywhere.
	if _, err := New(ConsistentHash, NewParams(map[string]string{ParamReplicas: "200"})); err != nil {
		t.Fatal(err)
	}
	if _, err := New(ZoneAware, NewParams(map[string]string{ParamZoneKey: "zone", ParamDelegate: LeastConn})); err != nil {
		t.Fatal(err)
	}

	// A present-but-unparsable value is an error, never a silent default.
	_, err = New(ConsistentHash, NewParams(map[string]string{ParamReplicas: "many"}))
	assert.Error(t, err).Matches("not an integer")

	// zone_aware delegate errors: unknown name, and self-delegation.
	_, err = New(ZoneAware, NewParams(map[string]string{ParamDelegate: "nope"}))
	assert.Error(t, err).Matches("zone_aware delegate")
	_, err = New(ZoneAware, NewParams(map[string]string{ParamDelegate: ZoneAware}))
	assert.Error(t, err).Matches("cannot delegate to itself")
}

// TestValidateCoversDefaultAndEveryRule pins the dry-run entry point: it reports
// what a chain rebuild would reject — an unregistered name, or parameters the
// strategy refuses — for the default and for every rule, and passes settings a
// rebuild would accept.
func TestValidateCoversDefaultAndEveryRule(t *testing.T) {
	// A well-formed settings: an empty default (the base strategy) plus rules that
	// name real strategies with the parameters those strategies own.
	ok := Settings{
		Default: Selection{},
		Rules: []Rule{
			{Service: "redis:cache", Selection: Selection{Balancer: LeastConn}},
			{Service: "gorm:db", Selection: Selection{
				Balancer: ConsistentHash,
				Params:   map[string]string{ParamReplicas: "200"},
			}},
		},
	}
	assert.Error(t, Validate(ok)).Nil()

	// An unregistered name in the DEFAULT.
	badDefault := Settings{Default: Selection{Balancer: "no_such_strategy"}}
	assert.Error(t, Validate(badDefault)).Matches("no strategy registered")

	// An unregistered name in a RULE — the default being fine must not mask it.
	badRule := Settings{Rules: []Rule{{Service: "redis:cache", Selection: Selection{Balancer: "no_such_strategy"}}}}
	assert.Error(t, Validate(badRule)).Matches("no strategy registered")

	// A registered name whose parameters the strategy rejects: caught here rather
	// than at the next rebuild.
	badParams := Settings{Rules: []Rule{{Service: "redis:cache", Selection: Selection{
		Balancer: ConsistentHash,
		Params:   map[string]string{ParamReplicas: "many"},
	}}}}
	assert.Error(t, Validate(badParams)).Matches("not an integer")

	// An empty Balancer is the base strategy, not an error.
	assert.Error(t, Validate(Settings{Rules: []Rule{{Service: "x"}}})).Nil()
}
