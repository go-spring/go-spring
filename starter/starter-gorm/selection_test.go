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
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

// armedManager is a selection manager armed the way the governance center arms
// it: one resolver the test owns, and an Apply per push. A test therefore drives
// the exact manager instance it handed to the builder, with no process-wide state
// involved.
type armedManager struct {
	m *loadbalance.Manager

	mu      sync.Mutex
	current loadbalance.Selection
}

func newArmedManager(t *testing.T, sel loadbalance.Selection) *armedManager {
	t.Helper()
	m, err := loadbalance.NewManager(nil)
	assert.Error(t, err).Nil()
	a := &armedManager{m: m, current: sel}
	a.m.Apply(loadbalance.Settings{Enabled: true, Resolve: a.resolve})
	return a
}

func (a *armedManager) resolve(string) loadbalance.Selection {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.current
}

// push adopts sel and re-evaluates every bound pool, as a governance source
// push does.
func (a *armedManager) push(sel loadbalance.Selection) {
	a.mu.Lock()
	a.current = sel
	a.mu.Unlock()
	a.m.Apply(loadbalance.Settings{Enabled: true, Resolve: a.resolve})
}

// TestNewPickPoolBindsSelection covers the gorm half of endpoint selection: the
// shared pool builder attaches a suspension tracker (without one the thresholds
// land on nothing) and binds the pool to the entry's service label through the
// injected manager, so one spring.governance.client.rules[N] rule — the same label that drives the
// entry's protection executor — also governs its balancing strategy. The returned
// stop detaches.
func TestNewPickPoolBindsSelection(t *testing.T) {
	a := newArmedManager(t, loadbalance.Selection{
		Balancer: loadbalance.LeastConn, OutlierThreshold: 3, OutlierSuspendFor: time.Minute,
	})

	c := Common{Addressing: discovery.Addressing{ServiceName: "user-db"}, Scheme: "tcp"}
	backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:3306", Healthy: true, Weight: 1})

	const label = "gorm:mysql:user-db"
	pool, resolver, stop, err := c.NewPickPool(context.Background(), backend, label, a.m)
	assert.Error(t, err).Nil()
	if resolver == nil || pool == nil {
		t.Fatal("service-name set with a backend bean should build a discovery pool")
	}

	// The selection in force at bind time lands before the first Pick.
	assert.That(t, pool.Tracker()).NotNil()
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(3)

	// A later push reaches the pool's selection in place, without a rebuild.
	a.push(loadbalance.Selection{Balancer: loadbalance.Weighted, OutlierThreshold: 5, OutlierSuspendFor: 2 * time.Minute})
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.Weighted)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(5)

	// The returned stop detaches the binding: a further push no longer lands.
	stop()
	a.push(loadbalance.Selection{Balancer: loadbalance.LeastConn, OutlierThreshold: 9})
	assert.That(t, pool.Selection().Balancer).Equal(loadbalance.Weighted)
	assert.Number(t, pool.Tracker().Config().Threshold).Equal(5)
}

// TestNewPickPoolAnUnarmedManagerIsPassThrough pins the "governance off" case: an
// unarmed manager (no rules in the process) leaves the pool on the strategy it
// was built with, and a stop from it is safe to call anyway. The manager is
// REQUIRED — the package that owns it registers it, so the container
// always has one; "governance off" is an unarmed manager, never a nil one.
func TestNewPickPoolAnUnarmedManagerIsPassThrough(t *testing.T) {
	unarmed, err := loadbalance.NewManager(nil)
	assert.Error(t, err).Nil()
	for _, mgr := range []*loadbalance.Manager{unarmed} {
		c := Common{Addressing: discovery.Addressing{ServiceName: "user-db"}, Scheme: "tcp"}
		backend := discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:3306", Healthy: true, Weight: 1})

		pool, resolver, stop, err := c.NewPickPool(context.Background(), backend, "gorm:mysql:user-db", mgr)
		assert.Error(t, err).Nil()
		if resolver == nil || pool == nil {
			t.Fatal("service-name set with a backend bean should build a discovery pool")
		}
		// Nothing was ever pushed: the pool keeps the strategy it was built
		// with, and its tracker is left at the zero config.
		assert.That(t, pool.Selection()).Equal(loadbalance.Selection{})
		assert.Number(t, pool.Tracker().Config().Threshold).Equal(0)
		ep, perr := pool.Pick(loadbalance.PickInfo{})
		assert.Error(t, perr).Nil()
		assert.That(t, ep.Addr).Equal("127.0.0.1:3306")

		stop()
	}
}

// TestNewPickPoolDirectAddrBindsNothing pins that a client dialing a fixed
// address has no candidate set to choose from: no pool, no resolver, and a stop
// that is safe to call anyway.
func TestNewPickPoolDirectAddrBindsNothing(t *testing.T) {
	a := newArmedManager(t, loadbalance.Selection{Balancer: loadbalance.LeastConn})

	c := Common{}
	pool, resolver, stop, err := c.NewPickPool(context.Background(), nil, "gorm:mysql:127.0.0.1:3306", a.m)
	assert.Error(t, err).Nil()
	assert.That(t, pool == nil).True()
	assert.That(t, resolver == nil).True()

	stop()
}
