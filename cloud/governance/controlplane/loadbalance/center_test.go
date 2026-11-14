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
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance/capability/source"
	kloadbalance "go-spring.org/cloud/governance/dataplane/outbound/loadbalance"
	balancer "go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
balancer: round_robin
request-failures: 5
zone-aware: true
services:
  - service: redis:cache
    balancer: consistent_hash
    consistent-hash-replicas: 200
    match:
      version:
        prefix: v2.
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[kloadbalance.Settings]("loadbalance.yaml", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.That(t, d.Balancer).Equal(balancer.RoundRobin)
	assert.Number(t, d.RequestFailures).Equal(5)
	// Omitted, the suspension knobs follow Envoy's outlier-detection defaults.
	assert.That(t, d.SuspendDuration).Equal(30 * time.Second)
	assert.That(t, d.MaxSuspendDuration).Equal(300 * time.Second)
	assert.That(t, d.Interval).Equal(10 * time.Second)
	assert.Number(t, d.MaxEjectionPercent).Equal(10)
	// Omitted, the floor follows the document default (Envoy's panic threshold).
	assert.Number(t, d.MinHealthyPercent).Equal(50)
	// The zone-aware switch is a flat selection field, not a strategy.
	assert.That(t, d.ZoneAware).True()
	assert.Number(t, len(d.Rules)).Equal(1)
	assert.That(t, d.Rules[0].Service).Equal("redis:cache")
	assert.That(t, d.Rules[0].Balancer).Equal(balancer.ConsistentHash)
	assert.Number(t, d.Rules[0].ConsistentHashReplicas).Equal(200)
	sel := d.Rules[0].MetadataMatch
	assert.Number(t, len(sel)).Equal(1)
	assert.That(t, sel["version"].Prefix).Equal("v2.")
}

// TestDocumentBoundsRingReplicas pins the resource ceiling on the document path:
// a rule asking for an unbounded ring is rejected at bind time instead of
// allocating it on the first keyed request.
func TestDocumentBoundsRingReplicas(t *testing.T) {
	_, err := source.Parse[kloadbalance.Settings]("loadbalance.yaml", []byte(`
balancer: consistent_hash
consistent-hash-replicas: 1000000
`), "")
	assert.Error(t, err).NotNil()
}

// TestDuplicateServicesFollowListOrder pins the resolution rule for a repeated
// service: the document binds as written and the first matching rule wins — the
// manager does not police the field, it just reads the list in one way. The rule
// matrix itself lives in capability/loadbalance's own tests.
func TestDuplicateServicesFollowListOrder(t *testing.T) {
	d, err := source.Parse[kloadbalance.Settings]("loadbalance.properties", []byte(`
services[0].service=redis:cache
services[0].balancer=round_robin
services[1].service=redis:cache
services[1].balancer=least_conn
`), "")
	assert.Error(t, err).Nil()

	obs, err := kloadbalance.NewOTelObserver()
	assert.Error(t, err).Nil()
	m := kloadbalance.NewManager(kloadbalance.NewFactory([]kloadbalance.Provider{kloadbalance.NewBuiltinProvider()}), obs, kloadbalance.ManagerConfig{})
	assert.Error(t, m.Apply(context.Background(), d)).Nil()
	assert.That(t, d.Rules[0].Balancer).Equal(balancer.RoundRobin)
}

func selectionChain(t *testing.T, m *kloadbalance.Manager, service string) *kloadbalance.Chain {
	t.Helper()
	src := func(context.Context) ([]discovery.Endpoint, error) {
		return []discovery.Endpoint{{Addr: "10.0.0.1:8080", Healthy: true, Weight: 1}}, nil
	}
	c, err := m.Chain(context.Background(), src, service)
	if err != nil {
		t.Fatalf("chain for %q: %v", service, err)
	}
	return c
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[kloadbalance.Settings]("loadbalance.yaml", []byte(doc), "")
	assert.Error(t, err).Nil()

	obs, err := kloadbalance.NewOTelObserver()
	assert.Error(t, err).Nil()
	mgr := kloadbalance.NewManager(kloadbalance.NewFactory([]kloadbalance.Provider{kloadbalance.NewBuiltinProvider()}), obs, kloadbalance.ManagerConfig{})
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	chain := selectionChain(t, mgr, "redis:cache")
	assert.That(t, chain.Selection().Balancer).Equal(balancer.ConsistentHash)
	assert.Number(t, chain.Selection().ConsistentHashReplicas).Equal(200)
	before := chain.Selection()

	// A document naming an unregistered strategy is rejected and the last good
	// selection keeps serving — on the same registered chain, which a push
	// rebuilds in place.
	src.Push(context.Background(), kloadbalance.Settings{Rules: []kloadbalance.Rule{{
		Service:   "redis:cache",
		Selection: kloadbalance.Selection{Balancer: "no_such_strategy"},
	}}})
	assert.That(t, chain.Selection().Balancer).Equal(balancer.ConsistentHash)
	assert.That(t, chain.Selection().SuspendDuration).Equal(before.SuspendDuration)
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot serve a REGISTERED chain fails wiring, rather
// than arming a center that would run that chain unconfigured. Unserved rules
// are just data and would not be built here.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	obs, err := kloadbalance.NewOTelObserver()
	if err != nil {
		t.Fatal(err)
	}
	mgr := kloadbalance.NewManager(kloadbalance.NewFactory([]kloadbalance.Provider{kloadbalance.NewBuiltinProvider()}), obs, kloadbalance.ManagerConfig{})
	selectionChain(t, mgr, "redis:cache")

	src := source.NewPushSource(kloadbalance.Settings{
		Rules: []kloadbalance.Rule{{Service: "redis:cache", Selection: kloadbalance.Selection{Balancer: "no_such_strategy"}}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an initial document that cannot serve a registered chain")
	}
}
