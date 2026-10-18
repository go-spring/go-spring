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

package StarterMongoDB

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// pickChain builds the construction-seam output under test: service-name routing
// against a static backend holding one live instance. Scheme is left unset, so
// the lookup is not narrowed by transport and the plain endpoint below matches.
func pickChain(t *testing.T, mgr *loadbalance.Manager) (*loadbalance.Chain, Config) {
	t.Helper()
	c := Config{Addressing: discovery.Addressing{ServiceName: "orders"}}
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:27017", Healthy: true, Weight: 1})
	chain, err := newPickChain(context.Background(), c, backend, mgr)
	assert.Error(t, err).Nil()
	if chain == nil {
		t.Fatal("service-name set with a backend bean should build a discovery chain")
	}
	return chain, c
}

// TestNewPickChainRebuildsFromScratch covers this starter's half of endpoint
// selection: the chain is registered under the service's governance label as it
// is built, the base generation serves before any rule arrives, and a rule that
// moves the label's selection REBUILDS the chain — a new pool, not the old one
// retuned.
func TestNewPickChainRebuildsFromScratch(t *testing.T) {
	mgr := loadbalance.NewManager()
	chain, c := pickChain(t, mgr)

	// The base generation: round-robin, suspension disabled, live instance picked.
	assert.Number(t, chain.Generation()).Equal(uint64(1))
	assert.That(t, chain.Selection()).Equal(loadbalance.Selection{})
	ep, err := chain.Pick(loadbalance.PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr).Equal("127.0.0.1:27017")

	// A push that targets THIS service's governance label moves the chain...
	moved := loadbalance.Selection{Balancer: loadbalance.LeastConn, OutlierThreshold: 3, OutlierSuspendFor: time.Minute}
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: serviceLabel(c), Selection: moved},
	}})

	assert.Number(t, chain.Generation()).Equal(uint64(2))
	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(3)

	// ...while a push for another label leaves it alone, even though the document
	// as a whole changed.
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: serviceLabel(c), Selection: moved},
		{Service: "mongodb:other", Selection: loadbalance.Selection{Balancer: loadbalance.Weighted}},
	}})
	assert.Number(t, chain.Generation()).Equal(uint64(2))
}

// TestNewPickChainUnarmedManagerIsPassThrough pins the "governance off" case: an
// unarmed manager (no rules in the process) leaves the chain on its base
// configuration, and it still serves.
func TestNewPickChainUnarmedManagerIsPassThrough(t *testing.T) {
	chain, _ := pickChain(t, loadbalance.NewManager())

	assert.Number(t, chain.Generation()).Equal(uint64(1))
	assert.That(t, chain.Selection()).Equal(loadbalance.Selection{})

	ep, err := chain.Pick(loadbalance.PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr).Equal("127.0.0.1:27017")
}

// TestNewPickChainDirectInstanceHasNoChain pins that a client citing no
// service-name has no candidate set to choose from: newPickChain returns a nil
// chain and the caller dials the URI hosts directly.
func TestNewPickChainDirectInstanceHasNoChain(t *testing.T) {
	chain, err := newPickChain(context.Background(), Config{}, nil, loadbalance.NewManager())
	assert.Error(t, err).Nil()
	assert.That(t, chain == nil).True()
}

// TestNewPickChainMissingBackendFails pins the loud failure: a service-name with
// no discovery backend cited by the ${discovery} label is a construction error,
// not a silently direct dial.
func TestNewPickChainMissingBackendFails(t *testing.T) {
	c := Config{Addressing: discovery.Addressing{ServiceName: "orders"}}
	_, err := newPickChain(context.Background(), c, nil, nil)
	assert.Error(t, err).NotNil()
}
