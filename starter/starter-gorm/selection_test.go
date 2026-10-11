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

package gormcore

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// armedManager is a selection manager armed the way the governance center arms
// it: the selection is PUSHED as the document's default, and re-pushed on every
// change. A test therefore drives the exact manager instance it handed to the
// builder, with no process-wide state involved.
type armedManager struct {
	m *loadbalance.Manager
}

func newArmedManager(t *testing.T, sel loadbalance.Selection) *armedManager {
	t.Helper()
	m := loadbalance.NewManager()
	a := &armedManager{m: m}
	a.push(sel)
	return a
}

// push adopts sel and re-evaluates every registered chain, as a governance
// source push does.
func (a *armedManager) push(sel loadbalance.Selection) {
	a.m.Apply(context.Background(), loadbalance.Settings{Default: sel})
}

// TestNewPickPoolRebuildsOnChange covers the gorm half of endpoint selection: the
// shared builder attaches a suspension tracker and registers the chain under the
// entry's service label, so one spring.governance.client.rules[N] rule — the same
// label that drives the entry's protection executor — also governs its balancing
// strategy. A rule that moves that selection REBUILDS the chain rather than
// retuning it.
func TestNewPickPoolRebuildsOnChange(t *testing.T) {
	a := newArmedManager(t, loadbalance.Selection{
		Balancer: loadbalance.LeastConn, OutlierThreshold: 3, OutlierSuspendFor: time.Minute,
	})

	c := Common{Addressing: discovery.Addressing{ServiceName: "user-db"}, Scheme: "tcp"}
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:3306", Healthy: true, Weight: 1})

	const label = "gorm:mysql:user-db"
	chain, resolver, err := c.NewPickPool(context.Background(), backend, label, a.m)
	assert.Error(t, err).Nil()
	if resolver == nil || chain == nil {
		t.Fatal("service-name set with a backend bean should build a discovery chain")
	}

	// The selection in force at build time lands on the first generation, which
	// already picks from the discovered endpoint.
	assert.Number(t, chain.Generation()).Equal(uint64(1))
	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(3)
	ep, perr := chain.Pick(loadbalance.PickInfo{})
	assert.Error(t, perr).Nil()
	assert.That(t, ep.Addr).Equal("127.0.0.1:3306")

	// A later push rebuilds the chain whole: a fresh pool, not the old one retuned.
	a.push(loadbalance.Selection{Balancer: loadbalance.Weighted, OutlierThreshold: 5, OutlierSuspendFor: 2 * time.Minute})

	assert.Number(t, chain.Generation()).Equal(uint64(2))
	assert.That(t, chain.Selection().Balancer).Equal(loadbalance.Weighted)
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(5)

	// A rule scoped to ANOTHER entry's label does not reach this one, even though
	// the document as a whole changed.
	a.m.Apply(context.Background(), loadbalance.Settings{
		Default: loadbalance.Selection{Balancer: loadbalance.Weighted, OutlierThreshold: 5, OutlierSuspendFor: 2 * time.Minute},
		Rules: []loadbalance.Rule{
			{Service: "gorm:mysql:other-db", Selection: loadbalance.Selection{Balancer: loadbalance.P2C}},
		},
	})
	assert.Number(t, chain.Generation()).Equal(uint64(2))
}

// TestNewPickPoolAnUnarmedManagerIsPassThrough pins the "governance off" case: an
// unarmed manager (no rules in the process) leaves the chain on its base
// configuration, and a stop from it is safe to call anyway. The manager must be
// non-nil — the container always supplies one.
func TestNewPickPoolAnUnarmedManagerIsPassThrough(t *testing.T) {
	unarmed := loadbalance.NewManager()
	c := Common{Addressing: discovery.Addressing{ServiceName: "user-db"}, Scheme: "tcp"}
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:3306", Healthy: true, Weight: 1})

	chain, resolver, err := c.NewPickPool(context.Background(), backend, "gorm:mysql:user-db", unarmed)
	assert.Error(t, err).Nil()
	if resolver == nil || chain == nil {
		t.Fatal("service-name set with a backend bean should build a discovery chain")
	}
	// Nothing was ever pushed: the base generation serves, with no strategy
	// recorded and the tracker left at the zero config.
	assert.That(t, chain.Selection()).Equal(loadbalance.Selection{})
	assert.Number(t, chain.Tracker().Config().Threshold).Equal(0)
	ep, perr := chain.Pick(loadbalance.PickInfo{})
	assert.Error(t, perr).Nil()
	assert.That(t, ep.Addr).Equal("127.0.0.1:3306")
}

// TestNewPickPoolDirectAddrBindsNothing pins that a client dialing a fixed
// address has no candidate set to choose from: no chain and no resolver.
func TestNewPickPoolDirectAddrBindsNothing(t *testing.T) {
	a := newArmedManager(t, loadbalance.Selection{Balancer: loadbalance.LeastConn})

	c := Common{}
	chain, resolver, err := c.NewPickPool(context.Background(), nil, "gorm:mysql:127.0.0.1:3306", a.m)
	assert.Error(t, err).Nil()
	assert.That(t, chain == nil).True()
	assert.That(t, resolver == nil).True()
}
