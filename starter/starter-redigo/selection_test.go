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

package StarterRedigo

import (
	"context"
	"testing"
	"time"

	"go-spring.org/cloud/runtime"

	"go-spring.org/cloud/runtime/controlplane/governance"
	serverfault "go-spring.org/cloud/runtime/dataplane/inbound/fault"
	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	clientfault "go-spring.org/cloud/runtime/dataplane/outbound/fault"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestNewPoolChainRebuildsFromScratch covers the redigo half of endpoint
// selection: a service-name entry builds an endpoint-selection chain registered
// under the pool's governance label, the base generation serves before any rule
// arrives, and a rule that moves that label's selection REBUILDS the chain — a
// new pool, not the old one retuned.
func TestNewPoolChainRebuildsFromScratch(t *testing.T) {
	mgr := loadbalance.NewManager()
	disc := discovery.NewManager(map[string]discovery.Discovery{
		"static": discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "127.0.0.1:6379", Healthy: true, Weight: 1}),
	})
	ctr, err := runtime.New(resilience.NewManager(resilience.NewDefaultDriver(nil)), mgr,
		clientfault.NewClientManager(nil), serverfault.NewServerManager(nil), disc, governance.NewPushSource(governance.Config{}))
	assert.Error(t, err).Nil()

	c := Config{Addressing: discovery.Addressing{ServiceName: "cache", Discovery: "static"}}

	p, err := NewPool(context.Background(), c, ctr)
	assert.Error(t, err).Nil()
	if p == nil || p.lbChain == nil {
		t.Fatal("service-name set with a discovery backend should build an endpoint chain")
	}

	// The base generation: round-robin, suspension disabled, live instance picked.
	assert.Number(t, p.lbChain.Generation()).Equal(uint64(1))
	assert.That(t, p.lbChain.Selection()).Equal(loadbalance.Selection{})
	ep, err := p.lbChain.Pick(loadbalance.PickInfo{})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr).Equal("127.0.0.1:6379")

	// A push that targets THIS pool's governance label rebuilds the chain whole.
	moved := loadbalance.Selection{Balancer: loadbalance.LeastConn, OutlierThreshold: 3, OutlierSuspendFor: time.Minute}
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{{Service: p.serviceLabel, Selection: moved}}})

	assert.Number(t, p.lbChain.Generation()).Equal(uint64(2))
	assert.That(t, p.lbChain.Selection().Balancer).Equal(loadbalance.LeastConn)
	assert.Number(t, p.lbChain.Tracker().Config().Threshold).Equal(3)

	// ...while a push for another label leaves it alone, even though the document
	// as a whole changed.
	mgr.Apply(context.Background(), loadbalance.Settings{Rules: []loadbalance.Rule{
		{Service: p.serviceLabel, Selection: moved},
		{Service: "redigo:other", Selection: loadbalance.Selection{Balancer: loadbalance.Weighted}},
	}})
	assert.Number(t, p.lbChain.Generation()).Equal(uint64(2))
}

// TestNewPoolWithoutDiscoveryHasNoChain pins that a fixed-address entry has no
// candidate set to choose from: the pool dials the configured Addr directly and
// carries no chain.
func TestNewPoolWithoutDiscoveryHasNoChain(t *testing.T) {
	ctr, err := runtime.NewProcess(resilience.NewManager(resilience.NewDefaultDriver(nil)), clientfault.NewClientManager(nil), serverfault.NewServerManager(nil))
	assert.Error(t, err).Nil()
	p, err := NewPool(context.Background(), Config{Addr: "127.0.0.1:6379"}, ctr)
	assert.Error(t, err).Nil()
	if p == nil {
		t.Fatal("a fixed-address pool should still assemble")
	}
	assert.That(t, p.lbChain == nil).True()
}
