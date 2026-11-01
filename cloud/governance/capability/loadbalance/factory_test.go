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

	// consistent_hash has one parameter with no zero case in code: a Selection
	// built in code states the count, while a bound document gets it from the
	// field's tag.
	b, err := factories.Create(Selection{Balancer: ConsistentHash, ConsistentHash: ConsistentHashParams{Replicas: 100}})
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()

	// zone_aware narrows the candidate set and cannot choose on its own, so it
	// only builds when a delegate is named.
	b, err = factories.Create(Selection{Balancer: ZoneAware, ZoneAware: ZoneAwareParams{ZoneKey: discovery.MetaKeyZone, Delegate: RoundRobin}})
	assert.Error(t, err).Nil()
	assert.That(t, b).NotNil()

	// An empty name is a construction error: the base strategy is stated by the
	// settings document, never implied here.
	_, err = factories.Create(Selection{})
	assert.Error(t, err).Matches("balancer name is required")

	_, err = factories.Create(Selection{Balancer: "does-not-exist"})
	assert.Error(t, err).Matches("no strategy registered")

	assert.That(t, factories.names()).Equal([]string{
		ConsistentHash, LeastConn, P2C, Random, RoundRobin, Weighted, ZoneAware,
	})
}

// duplicateStrategyProvider exports a name another provider already owns.
type duplicateStrategyProvider struct{ name string }

func (f duplicateStrategyProvider) Strategies() []string { return []string{f.name} }

func (duplicateStrategyProvider) New(Selection, Resolver) (Balancer, error) { return nil, nil }

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

func (repeatingStrategyProvider) New(Selection, Resolver) (Balancer, error) {
	return &roundRobin{}, nil
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
	assert.That(t, len(p.Strategies())).Equal(7)
	for _, name := range p.Strategies() {
		selection := Selection{Balancer: name}
		switch name {
		case ConsistentHash:
			selection.ConsistentHash = ConsistentHashParams{Replicas: 100}
		case ZoneAware:
			selection.ZoneAware = ZoneAwareParams{ZoneKey: discovery.MetaKeyZone, Delegate: RoundRobin}
		}
		b, err := p.New(selection, testFactories().Create)
		assert.Error(t, err).Nil()
		assert.That(t, b).NotNil()
	}
}

// TestForeignGroupsIgnored pins that a built-in reads only its own parameter
// group: a group aimed at another strategy is ignored rather than failing the
// build.
func TestForeignGroupsIgnored(t *testing.T) {
	_, err := testFactories().Create(Selection{
		Balancer: LeastConn, ConsistentHash: ConsistentHashParams{Replicas: 200},
	})
	assert.Error(t, err).Nil()

	_, err = testFactories().Create(Selection{
		Balancer: ConsistentHash, ZoneAware: ZoneAwareParams{ZoneKey: "az", Delegate: RoundRobin},
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
		Balancer: ConsistentHash, ConsistentHash: ConsistentHashParams{Replicas: 200},
	})
	assert.Error(t, err).Nil()

	// zone_aware delegate errors: empty, unknown name, and self-delegation.
	_, err = testFactories().Create(Selection{Balancer: ZoneAware})
	assert.Error(t, err).Matches("requires a delegate")
	_, err = testFactories().Create(Selection{Balancer: ZoneAware, ZoneAware: ZoneAwareParams{ZoneKey: discovery.MetaKeyZone, Delegate: "nope"}})
	assert.Error(t, err).Matches("zone_aware delegate")
	_, err = testFactories().Create(Selection{Balancer: ZoneAware, ZoneAware: ZoneAwareParams{ZoneKey: discovery.MetaKeyZone, Delegate: ZoneAware}})
	assert.Error(t, err).Matches("cannot delegate to itself")
}
