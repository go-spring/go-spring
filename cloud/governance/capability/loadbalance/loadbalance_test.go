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
	"context"
	"testing"

	"go-spring.org/cloud/governance/capability/discovery"
	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/stdlib/testing/assert"
)

func TestRegistryBuildsBuiltins(t *testing.T) {
	for _, name := range []string{RoundRobin, LeastConn, Weighted, Random, P2C} {
		b, err := New(Selection{Balancer: name})
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}

	// consistent_hash has one parameter with no zero case in code: a Selection
	// built in code states the count, while a bound document gets it from the
	// field's tag.
	b, err := New(Selection{Balancer: ConsistentHash, ConsistentHash: ConsistentHashParams{Replicas: 100}})
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()

	// zone_aware narrows the candidate set and cannot choose on its own, so it
	// only builds when a delegate is named.
	b, err = New(Selection{Balancer: ZoneAware, ZoneAware: ZoneAwareParams{ZoneKey: discovery.MetaKeyZone, Delegate: RoundRobin}})
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()

	// An empty name is a construction error: the base strategy is stated by the
	// settings document, never implied here.
	_, err = New(Selection{})
	assert.Error(t, err).Matches("balancer name is required")

	_, err = New(Selection{Balancer: "does-not-exist"})
	assert.Error(t, err).Matches("no strategy registered")

	assert.That(t, names()).Equal([]string{
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
	assert.Panic(t, func() { Register("", newRoundRobin) }, "empty name")
	assert.Panic(t, func() { Register("nil", nil) }, "nil factory")
	assert.Panic(t, func() { Register(RoundRobin, newRoundRobin) }, "already registered")
}

// TestParamsPartition covers the strict-partition guarantee of the parameter
// groups: a group set under a strategy that does not read it is rejected by the
// document validation, not silently dropped.
func TestParamsPartition(t *testing.T) {
	// A group aimed at another strategy is a document error.
	err := Validate(Settings{Default: Selection{
		Balancer: LeastConn, ConsistentHash: ConsistentHashParams{Replicas: 200},
	}})
	assert.Error(t, err).Matches("consistent-hash parameters set under")

	err = Validate(Settings{Default: Selection{
		Balancer: ConsistentHash, ZoneAware: ZoneAwareParams{ZoneKey: "az", Delegate: RoundRobin},
	}})
	assert.Error(t, err).Matches("zone-aware parameters set under")

	// Loose params are the contributed seam: a built-in strategy with loose
	// params is a document error, a contributed name may carry them.
	err = Validate(Settings{Default: Selection{
		Balancer: RoundRobin, Params: map[string]string{"zone": "cn-1"},
	}})
	assert.Error(t, err).Matches("contributed strategies only")

	// A strategy's own group passes; the empty selection passes everywhere.
	assert.Error(t, Validate(Settings{Default: Selection{
		Balancer: ConsistentHash, ConsistentHash: ConsistentHashParams{Replicas: 200},
	}})).Nil()

	// zone_aware delegate errors: empty, unknown name, and self-delegation.
	_, err = New(Selection{Balancer: ZoneAware})
	assert.Error(t, err).Matches("requires a delegate")
	_, err = New(Selection{Balancer: ZoneAware, ZoneAware: ZoneAwareParams{ZoneKey: discovery.MetaKeyZone, Delegate: "nope"}})
	assert.Error(t, err).Matches("zone_aware delegate")
	_, err = New(Selection{Balancer: ZoneAware, ZoneAware: ZoneAwareParams{ZoneKey: discovery.MetaKeyZone, Delegate: ZoneAware}})
	assert.Error(t, err).Matches("cannot delegate to itself")
}

// TestValidateCoversDefaultAndEveryRule pins the dry-run entry point: it reports
// what a chain rebuild would reject — an unregistered name, or parameters the
// strategy refuses — for the default and for every rule, and passes settings a
// rebuild would accept.
func TestValidateCoversDefaultAndEveryRule(t *testing.T) {
	// A well-formed settings: the base strategy stated in the default, plus rules
	// that name real strategies with the parameters those strategies own.
	ok := Settings{
		Default: Selection{Balancer: RoundRobin},
		Rules: []Rule{
			{Service: "redis:cache", Selection: Selection{Balancer: LeastConn}},
			{Service: "gorm:db", Selection: Selection{
				Balancer:       ConsistentHash,
				ConsistentHash: ConsistentHashParams{Replicas: 200},
			}},
		},
	}
	assert.Error(t, Validate(ok)).Nil()

	// An unregistered name in the DEFAULT.
	badDefault := Settings{Default: Selection{Balancer: "no_such_strategy"}}
	assert.Error(t, Validate(badDefault)).Matches("no strategy registered")

	// An unregistered name in a RULE — the default being fine must not mask it.
	badRule := Settings{
		Default: Selection{Balancer: RoundRobin},
		Rules:   []Rule{{Service: "redis:cache", Selection: Selection{Balancer: "no_such_strategy"}}},
	}
	assert.Error(t, Validate(badRule)).Matches("no strategy registered")

	// A group aimed at another strategy: caught here rather than at the next
	// rebuild.
	badParams := Settings{
		Default: Selection{Balancer: RoundRobin},
		Rules: []Rule{{Service: "redis:cache", Selection: Selection{
			Balancer:       LeastConn,
			ConsistentHash: ConsistentHashParams{Replicas: 200},
		}}},
	}
	assert.Error(t, Validate(badParams)).Matches("consistent-hash parameters set under")

	// An empty Balancer is an error here too: a rule that leaves it out cannot be
	// built, so the document is rejected whole rather than degrading one label.
	assert.Error(t, Validate(Settings{
		Default: Selection{Balancer: RoundRobin},
		Rules:   []Rule{{Service: "x"}},
	})).Matches("balancer name is required")
}

// TestValidateRejectsBadSelector pins that a malformed selector is rejected by
// the document validation on both the default and a rule, before the strategy
// is even resolved. The malformation used here is the cross-field one Validate
// owns: a non-exact op with an empty operand (the key/op single-field rules live
// in the tags).
func TestValidateRejectsBadSelector(t *testing.T) {
	bad := matcher.Selector{{Key: "version", Op: matcher.Prefix, Value: ""}}

	assert.Error(t, Validate(Settings{Default: Selection{Balancer: RoundRobin, Selector: bad}})).NotNil()
	assert.Error(t, Validate(Settings{Rules: []Rule{{Service: "svc:x", Selection: Selection{Balancer: RoundRobin, Selector: bad}}}})).NotNil()
	assert.Error(t, Validate(Settings{Default: Selection{Balancer: RoundRobin, Selector: versionPrefix("v2.")}})).Nil()
}

// TestParamsPartitionFollowsTheDelegate pins the partition rule for a composed
// strategy: zone_aware's groups belong to its delegate, so they are judged
// against the name the delegate resolves to.
func TestParamsPartitionFollowsTheDelegate(t *testing.T) {
	assert.Error(t, Validate(Settings{Default: Selection{
		Balancer:       ZoneAware,
		ZoneAware:      ZoneAwareParams{ZoneKey: "zone", Delegate: ConsistentHash},
		ConsistentHash: ConsistentHashParams{Replicas: 200},
	}})).Nil()

	assert.Error(t, Validate(Settings{Default: Selection{
		Balancer:       ZoneAware,
		ZoneAware:      ZoneAwareParams{ZoneKey: "zone", Delegate: RoundRobin},
		ConsistentHash: ConsistentHashParams{Replicas: 200},
	}})).Matches("consistent-hash parameters set under")
}

// TestValidateRejectsBadServiceList pins the document-level checks: every rule
// names a service, and a label is unique — a linear scan would otherwise let
// list order decide which entry applies.
func TestValidateRejectsBadServiceList(t *testing.T) {
	assert.Error(t, Validate(Settings{
		Default: Selection{Balancer: RoundRobin},
		Rules:   []Rule{{Service: ""}},
	})).Matches("without a service")

	assert.Error(t, Validate(Settings{
		Default: Selection{Balancer: RoundRobin},
		Rules: []Rule{
			{Service: "redis:cache", Selection: Selection{Balancer: RoundRobin}},
			{Service: "redis:cache", Selection: Selection{Balancer: LeastConn}},
		},
	})).Matches("duplicate entry")
}

// TestEmptyDefaultMeansRoundRobin pins the base default: a document that leaves
// selection alone is valid and arms round-robin, rather than failing because no
// strategy was stated.
func TestEmptyDefaultMeansRoundRobin(t *testing.T) {
	assert.Error(t, Validate(Settings{})).Nil()

	m := newManager(t)
	m.Apply(context.Background(), Settings{})
	assert.That(t, selectionFor(m, "redis:cache").Balancer).Equal(RoundRobin)
}
