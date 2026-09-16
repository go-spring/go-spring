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

package StarterRegistryNacos

import (
	"context"
	"go-spring.org/cloud/discovery"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/codes"
)

// lastRegistered / lastUpdated / lastDeregistered expose the fake's most
// recent write of each kind for param-mapping assertions.
func (f *fakeRegistrarClient) lastRegistered() vo.RegisterInstanceParam {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.registered) == 0 {
		return vo.RegisterInstanceParam{}
	}
	return f.registered[len(f.registered)-1]
}

func (f *fakeRegistrarClient) lastDeregistered() vo.DeregisterInstanceParam {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.deregistered) == 0 {
		return vo.DeregisterInstanceParam{}
	}
	return f.deregistered[len(f.deregistered)-1]
}

func (f *fakeRegistrarClient) lastUpdated() vo.UpdateInstanceParam {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.updated) == 0 {
		return vo.UpdateInstanceParam{}
	}
	return f.updated[len(f.updated)-1]
}

// TestRegisterMapsInstanceParams pins the neutral-to-Nacos mapping: the
// advertised addr splits into ip/port, the block's group/cluster scope the
// write, the instance is always ephemeral (the SDK heartbeat contract), and
// metadata rides along. A negative weight clamps to 1 while 0 — the drain
// signal — passes through untouched.
func TestRegisterMapsInstanceParams(t *testing.T) {
	r, client := newTestRegistrar(false)
	meta := map[string]string{"zone": "a"}

	assert.Error(t, r.Register(context.Background(), discovery.Instance{
		ServiceName: "orders", Addr: "10.0.0.5:8080", Weight: -5, Metadata: meta,
	})).Nil()
	p := client.lastRegistered()
	assert.That(t, p.Ip).Equal("10.0.0.5")
	assert.That(t, p.Port).Equal(uint64(8080))
	assert.That(t, p.ServiceName).Equal("orders")
	assert.That(t, p.GroupName).Equal("DEFAULT_GROUP")
	assert.That(t, p.ClusterName).Equal("DEFAULT")
	assert.That(t, p.Ephemeral).True()
	assert.That(t, p.Enable).True()
	assert.That(t, p.Weight).Equal(float64(1)) // -5 clamps to 1
	assert.That(t, p.Metadata["zone"]).Equal("a")

	// Weight 0 is the drain signal and must survive the clamp.
	assert.Error(t, r.Register(context.Background(), discovery.Instance{
		ServiceName: "orders", Addr: "10.0.0.5:8080", Weight: 0,
	})).Nil()
	assert.That(t, client.lastRegistered().Weight).Equal(float64(0))
}

// TestDeregisterMapsInstanceParams pins the deregister mapping: the same
// ip/port split and group scope, ephemeral so the server drops the heartbeat
// with it.
func TestDeregisterMapsInstanceParams(t *testing.T) {
	r, client := newTestRegistrar(false)

	assert.Error(t, r.Deregister(context.Background(), discovery.Instance{
		ServiceName: "orders", Addr: "10.0.0.5:8080",
	})).Nil()
	p := client.lastDeregistered()
	assert.That(t, p.Ip).Equal("10.0.0.5")
	assert.That(t, p.Port).Equal(uint64(8080))
	assert.That(t, p.ServiceName).Equal("orders")
	assert.That(t, p.GroupName).Equal("DEFAULT_GROUP")
	assert.That(t, p.Ephemeral).True()
}

// TestUpdateWeightMapsParams pins the hot-reload mapping: the new weight
// (0 = drain, passed through) replaces the old one while the rest of the
// advertisement — metadata included — stays as registered.
func TestUpdateWeightMapsParams(t *testing.T) {
	r, client := newTestRegistrar(false)
	in := discovery.Instance{ServiceName: "orders", Addr: "10.0.0.5:8080", Weight: 5, Metadata: map[string]string{"zone": "a"}}

	assert.Error(t, r.Register(context.Background(), in)).Nil()
	assert.Error(t, r.UpdateWeight(context.Background(), in, 0)).Nil()

	p := client.lastUpdated()
	assert.That(t, p.Weight).Equal(float64(0))
	assert.That(t, p.Ephemeral).True()
	assert.That(t, p.Metadata["zone"]).Equal("a")
}

// TestRegisterEmitsClientSpan pins the trace half of the instrumentation:
// the write path opens a client span named for the operation, carrying the
// backend's system and the failure it ended with.
func TestRegisterEmitsClientSpan(t *testing.T) {
	testSpans.Reset()
	r, _ := newTestRegistrar(true) // server-side rejection → failed span

	err := r.Register(context.Background(), discovery.Instance{ServiceName: "orders-span", Addr: "1.2.3.4:80", Weight: 1})
	assert.Error(t, err).NotNil()

	for _, s := range testSpans.GetSpans() {
		if s.Name != "register" {
			continue
		}
		attrs := map[string]string{}
		for _, a := range s.Attributes {
			attrs[string(a.Key)] = a.Value.AsString()
		}
		if attrs["registry.system"] != obsSystem || attrs["registry.service"] != "orders-span" {
			continue
		}
		if s.Status.Code == codes.Error {
			return
		}
		t.Fatalf("the rejected register's span must end in error status, got %v", s.Status)
	}
	t.Fatal("no register span with system=nacos was emitted")
}

// TestUpdateWeightEmitsClientSpan pins the same for the weight path: its own
// operation name, no reason attribute (a weight change is never a
// re-register).
func TestUpdateWeightEmitsClientSpan(t *testing.T) {
	testSpans.Reset()
	r, _ := newTestRegistrar(false)
	in := discovery.Instance{ServiceName: "orders-wspan", Addr: "1.2.3.4:80", Weight: 1}

	assert.Error(t, r.UpdateWeight(context.Background(), in, 2)).Nil()

	for _, s := range testSpans.GetSpans() {
		if s.Name != "update_weight" {
			continue
		}
		attrs := map[string]string{}
		for _, a := range s.Attributes {
			attrs[string(a.Key)] = a.Value.AsString()
		}
		if attrs["registry.system"] != obsSystem || attrs["registry.service"] != "orders-wspan" {
			continue
		}
		if _, hasReason := attrs["registry.reason"]; hasReason {
			t.Fatal("a weight-change span must carry no reason attribute")
		}
		return
	}
	t.Fatal("no update_weight span with system=nacos was emitted")
}
