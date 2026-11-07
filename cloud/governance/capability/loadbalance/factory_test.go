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

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestFactoryBuildsBuiltins(t *testing.T) {
	factories := testFactories()
	for _, name := range []string{RoundRobin, LeastConn, Weighted, Random, P2C} {
		b, err := factories.Create(Selection{Balancer: name})
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}

	// consistent_hash has one parameter: a rule that omits it gets the tag's
	// default, and a code-built selection that omits it gets the same default
	// when the ring is built.
	b, err := factories.Create(Selection{Balancer: ConsistentHash, ConsistentHashReplicas: 100})
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()

	b, err = factories.Create(Selection{Balancer: ConsistentHash})
	assert.Error(t, err).Nil()
	ep, err := b.Pick(eps("a"), PickInfo{HashKey: "user-42"})
	assert.Error(t, err).Nil()
	assert.String(t, ep.Addr).Equal("a")

	// The zone-aware switch wraps whichever strategy was named, so the rule states
	// its balancer once.
	b, err = factories.Create(Selection{Balancer: RoundRobin, ZoneAware: true, ZoneKey: discovery.MetaKeyZone})
	assert.Error(t, err).Nil()
	_, zoneAware := b.(*ZoneAwareBalancer)
	assert.That(t, zoneAware).True()

	// An empty name is a construction error: the base strategy is stated by the
	// settings document, never implied here.
	_, err = factories.Create(Selection{})
	assert.Error(t, err).Matches("balancer name is required")

	_, err = factories.Create(Selection{Balancer: "does-not-exist"})
	assert.Error(t, err).Matches("no strategy registered")

	assert.That(t, factories.strategyNames()).Equal([]string{
		ConsistentHash, LeastConn, P2C, Random, RoundRobin, Weighted,
	})
}

// duplicateStrategyProvider exports a name another provider already owns.
type duplicateStrategyProvider struct{ name string }

func (f duplicateStrategyProvider) Strategies() []string { return []string{f.name} }

func (duplicateStrategyProvider) New(Selection) (Balancer, error) { return nil, nil }

func TestFactoryRejectsDuplicateStrategies(t *testing.T) {
	assert.Panic(t, func() {
		NewFactory([]Provider{
			NewBuiltinProvider(),
			duplicateStrategyProvider{name: RoundRobin},
		})
	}, "exported by two providers")

	assert.Panic(t, func() {
		NewFactory([]Provider{
			duplicateStrategyProvider{name: "same"},
			duplicateStrategyProvider{name: "same"},
		})
	}, "exported by two providers")
}

// repeatingStrategyProvider exports one name twice; only two providers owning
// the name is a collision.
type repeatingStrategyProvider struct{ name string }

func (p repeatingStrategyProvider) Strategies() []string { return []string{p.name, p.name} }

func (repeatingStrategyProvider) New(Selection) (Balancer, error) {
	return &RoundRobinBalancer{}, nil
}

func TestFactoryToleratesOneProvidersRepeats(t *testing.T) {
	f := NewFactory([]Provider{repeatingStrategyProvider{name: RoundRobin}})
	_, err := f.Create(Selection{Balancer: RoundRobin})
	assert.Error(t, err).Nil()
}

// TestBuiltinProviderExportsEveryStrategy pins the built-in provider's set: it
// answers every built-in name and builds each of them.
func TestBuiltinProviderExportsEveryStrategy(t *testing.T) {
	p := NewBuiltinProvider()
	assert.That(t, len(p.Strategies())).Equal(6)
	for _, name := range p.Strategies() {
		selection := Selection{Balancer: name}
		if name == ConsistentHash {
			selection.ConsistentHashReplicas = 100
		}
		b, err := p.New(selection)
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}
}

// TestForeignGroupsIgnored pins that a built-in reads only its own parameter
// group: a group aimed at another strategy is ignored rather than failing the
// build.
func TestForeignGroupsIgnored(t *testing.T) {
	_, err := testFactories().Create(Selection{
		Balancer: LeastConn, ConsistentHashReplicas: 200,
	})
	assert.Error(t, err).Nil()

	// The zone-aware switch is not a group: it applies to every strategy.
	_, err = testFactories().Create(Selection{
		Balancer: ConsistentHash, ZoneAware: true, ZoneKey: "az",
		ConsistentHashReplicas: 100,
	})
	assert.Error(t, err).Nil()

	// Loose params are the contributed seam; a built-in leaves them alone too.
	b, err := testFactories().Create(Selection{
		Balancer: RoundRobin, Params: map[string]string{"zone": "cn-1"},
	})
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()

	// A strategy's own group passes.
	_, err = testFactories().Create(Selection{
		Balancer: ConsistentHash, ConsistentHashReplicas: 200,
	})
	assert.Error(t, err).Nil()

	// A zone key the selection does not state defaults to the shared zone key
	// when the switch is on.
	b, err = testFactories().Create(Selection{Balancer: RoundRobin, ZoneAware: true})
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()
}
