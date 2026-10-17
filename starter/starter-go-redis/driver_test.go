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

package StarterGoRedis

import (
	"context"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// testCenter builds a governance center over res and lb with an unarmed fault
// injector and an empty document: everything a driver reads, with no wiring.
func testCenter(t *testing.T, resClient *resilience.ClientManager, lb *loadbalance.Manager) *governance.Center {
	t.Helper()
	ctr, err := governance.NewCenter(resClient, lb, fault.NewClientManager(nil), fault.NewServerManager(nil), nil,
		governance.NewPushSource(governance.Config{}))
	if err != nil {
		t.Fatal(err)
	}
	return ctr
}

// poolDriver is a stand-in for a company Driver whose topology needs an
// endpoint-selection chain: it builds one over its endpoint source, hands it to
// NewClient along with the governance bundle, and returns the wrapper — exactly
// as the Driver contract asks. This test exists to pin that a chain built by a
// driver is governed from construction (it registers itself, see
// [loadbalance.Manager.Chain]) and that NewClient needs no binding step.
type poolDriver struct {
	endpoints []discovery.Endpoint
	// returned records the chain the driver handed over, so the test can assert
	// governance landed on THAT chain rather than on some other.
	returned *loadbalance.Chain
}

func (d *poolDriver) CreateClient(ctx context.Context, c Config, center *governance.Center) (*Client, error) {
	src := func() ([]discovery.Endpoint, error) { return d.endpoints, nil }
	d.returned = center.Loadbalance().Chain(ctx, src, serviceLabel(c))
	// A real *redis.Client is not needed: this driver builds no dialer.
	return NewClient(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), c, d.returned, center)
}

// TestDriverChainIsGovernedFromConstruction pins the split the Driver interface
// encodes: the driver builds the chain over its own endpoint source and hands it
// over — that build IS the governance registration, so one rule reaches the
// chain the driver returned without NewClient doing anything. A driver that
// builds a chain must get governance for free, and one that builds none must not
// be forced to care.
func TestDriverChainIsGovernedFromConstruction(t *testing.T) {
	drv := &poolDriver{endpoints: []discovery.Endpoint{
		{Addr: "10.0.0.1:6379", Healthy: true},
		{Addr: "10.0.0.2:6379", Healthy: true},
	}}

	// The manager is applied to the bundle the driver receives; a chain built
	// against an unarmed manager is rebuilt when the manager goes live — standing
	// in for a center that pushes later.
	mgr := loadbalance.NewManager()
	cfg := Config{Addr: "10.0.0.1:6379"}
	w, err := drv.CreateClient(context.Background(), cfg,
		testCenter(t, resilience.NewClientManager(resilience.NewDefaultDriver(nil)), mgr))
	assert.Error(t, err).Nil()

	// The rule gives this service a strategy the chain was not built with, and
	// targets the very label the driver registered under.
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{{
		Service: serviceLabel(cfg),
		Selection: loadbalance.Selection{
			Balancer:          loadbalance.ConsistentHash,
			OutlierThreshold:  3,
			OutlierSuspendFor: time.Second,
		},
	}}})

	// The rule landed on the chain the driver returned, as a rebuild — a fresh
	// generation, not the base one retuned.
	assert.That(t, drv.returned).NotNil()
	assert.Number(t, drv.returned.Generation()).Equal(uint64(2))
	assert.That(t, drv.returned.Selection().Balancer).Equal(loadbalance.ConsistentHash)
	assert.Number(t, drv.returned.Tracker().Config().Threshold).Equal(3)
	// Destroy still releases the client.
	assert.Error(t, w.Destroy()).Nil()
}

// TestDriverChainNilIsFine pins the other half: a driver whose topology needs no
// pick chain passes nil and NewClient stores nothing — no nil dereference, no
// forced governance awareness.
func TestDriverChainNilIsFine(t *testing.T) {
	lb := loadbalance.NewManager()
	w, err := NewClient(redis.NewClient(&redis.Options{Addr: "127.0.0.1:0"}), Config{Addr: "10.0.0.1:6379"}, nil,
		testCenter(t, resilience.NewClientManager(resilience.NewDefaultDriver(nil)), lb))
	assert.Error(t, err).Nil()
	assert.That(t, w.lbChain).Nil()
}
