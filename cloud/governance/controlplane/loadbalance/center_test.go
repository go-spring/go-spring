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

	"go-spring.org/cloud/governance/capability/discovery"
	kloadbalance "go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/controlplane/source"
	"go-spring.org/stdlib/testing/assert"
)

const doc = `
balancer: round_robin
outlier-threshold: 5
services:
  - service: redis:cache
    balancer: consistent_hash
    consistent-hash:
      replicas: 200
    selector:
      - key: version
        kind: prefix
        value: v2.
`

func TestDocumentBinds(t *testing.T) {
	d, err := source.Parse[kloadbalance.Settings]("loadbalance.yaml", []byte(doc), "")
	assert.Error(t, err).Nil()

	assert.That(t, d.Default.Balancer).Equal(kloadbalance.RoundRobin)
	assert.Number(t, d.Default.OutlierThreshold).Equal(5)
	assert.Number(t, len(d.Rules)).Equal(1)
	assert.That(t, d.Rules[0].Service).Equal("redis:cache")
	assert.That(t, d.Rules[0].Balancer).Equal(kloadbalance.ConsistentHash)
	assert.Number(t, d.Rules[0].ConsistentHash.Replicas).Equal(200)
	sel := d.Rules[0].Selector
	assert.Number(t, len(sel)).Equal(1)
	assert.That(t, sel[0].Key).Equal("version")
	assert.That(t, sel[0].Value).Equal("v2.")
}

// TestDuplicateServicesRejected pins the binding boundary: conf.Bind runs the
// data-plane Validate, so a duplicate label is rejected where the document is
// read. The rule matrix itself lives in capability/loadbalance's own tests.
func TestDuplicateServicesRejected(t *testing.T) {
	_, err := source.Parse[kloadbalance.Settings]("loadbalance.properties", []byte(`
services[0].service=redis:cache
services[0].balancer=round_robin
services[1].service=redis:cache
services[1].balancer=least_conn
`), "")
	assert.Error(t, err).NotNil()
}

func selectionChain(t *testing.T, m *kloadbalance.Manager, label string) *kloadbalance.Chain {
	t.Helper()
	src := func(context.Context) ([]discovery.Endpoint, error) {
		return []discovery.Endpoint{{Addr: "10.0.0.1:8080", Healthy: true, Weight: 1}}, nil
	}
	c, err := m.Chain(context.Background(), src, label)
	if err != nil {
		t.Fatalf("chain for %q: %v", label, err)
	}
	return c
}

func TestCenterAppliesAndKeepsLastGood(t *testing.T) {
	d, err := source.Parse[kloadbalance.Settings]("loadbalance.yaml", []byte(doc), "")
	assert.Error(t, err).Nil()

	mgr := kloadbalance.NewManager()
	src := source.NewPushSource(d)
	if _, err := NewCenter(context.Background(), mgr, src); err != nil {
		t.Fatal(err)
	}

	chain := selectionChain(t, mgr, "redis:cache")
	assert.That(t, chain.Selection().Balancer).Equal(kloadbalance.ConsistentHash)
	assert.Number(t, chain.Selection().ConsistentHash.Replicas).Equal(200)

	// A document naming an unregistered strategy is rejected and the last good
	// selection keeps serving.
	src.Push(context.Background(), kloadbalance.Settings{Rules: []kloadbalance.Rule{{
		Service:   "redis:cache",
		Selection: kloadbalance.Selection{Balancer: "no_such_strategy"},
	}}})
	again := selectionChain(t, mgr, "redis:cache")
	assert.That(t, again.Selection().Balancer).Equal(kloadbalance.ConsistentHash)
	assert.That(t, again.Selection().OutlierSuspendFor).Equal(time.Duration(0))
}

// TestNewCenterRejectsUnservableSnapshot pins the startup contract: a source
// whose initial document cannot serve fails wiring, rather than arming a center
// that would run unconfigured.
func TestNewCenterRejectsUnservableSnapshot(t *testing.T) {
	mgr := kloadbalance.NewManager()
	src := source.NewPushSource(kloadbalance.Settings{
		Rules: []kloadbalance.Rule{{Service: "redis:cache"}},
	})
	if _, err := NewCenter(context.Background(), mgr, src); err == nil {
		t.Fatal("NewCenter must reject an unservable initial document")
	}
}
