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

package StarterDiscoveryNacos

import (
	"context"
	"sync"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/netutil"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/codes"
)

// fakeRegistryClient adds the registry half to fakeNamingClient: it records
// the write calls and can simulate a server-side rejection (ok=false with no
// transport error), which is a distinct failure mode in the Nacos SDK.
type fakeRegistryClient struct {
	*fakeNamingClient

	mu           sync.Mutex
	registered   []vo.RegisterInstanceParam
	deregistered []vo.DeregisterInstanceParam
	updated      []vo.UpdateInstanceParam
	reject       bool
}

func (f *fakeRegistryClient) RegisterInstance(p vo.RegisterInstanceParam) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registered = append(f.registered, p)
	return !f.reject, nil
}

func (f *fakeRegistryClient) DeregisterInstance(p vo.DeregisterInstanceParam) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deregistered = append(f.deregistered, p)
	return !f.reject, nil
}

func (f *fakeRegistryClient) UpdateInstance(p vo.UpdateInstanceParam) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, p)
	return !f.reject, nil
}

// setReject toggles the server-side rejection the fake reports, so one test can
// publish successfully and then have the follow-up call rejected.
func (f *fakeRegistryClient) setReject(v bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reject = v
}

// counts reports how many writes of each kind the fake received.
func (f *fakeRegistryClient) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.registered), len(f.deregistered), len(f.updated)
}

// lastRegistered / lastUpdated / lastDeregistered expose the fake's most
// recent write of each kind for param-mapping assertions.
func (f *fakeRegistryClient) lastRegistered() vo.RegisterInstanceParam {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.registered) == 0 {
		return vo.RegisterInstanceParam{}
	}
	return f.registered[len(f.registered)-1]
}

func (f *fakeRegistryClient) lastDeregistered() vo.DeregisterInstanceParam {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.deregistered) == 0 {
		return vo.DeregisterInstanceParam{}
	}
	return f.deregistered[len(f.deregistered)-1]
}

func (f *fakeRegistryClient) lastUpdated() vo.UpdateInstanceParam {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.updated) == 0 {
		return vo.UpdateInstanceParam{}
	}
	return f.updated[len(f.updated)-1]
}

// newTestRegistry returns a nacos registry over a fake naming client.
func newTestRegistry(reject bool) (*nacosRegistry, *fakeRegistryClient) {
	client := &fakeRegistryClient{fakeNamingClient: &fakeNamingClient{}, reject: reject}
	return &nacosRegistry{client: client, group: "DEFAULT_GROUP", cluster: "DEFAULT", obs: newTestObserver()}, client
}

func TestNormalizeWeight(t *testing.T) {
	// A misconfigured negative weight clamps to 1.
	assert.That(t, normalizeWeight(-5)).Equal(1)
	// 0 is the drain signal and passes through untouched, on both write paths.
	assert.That(t, normalizeWeight(0)).Equal(0)
	// An explicit positive weight passes through unchanged.
	assert.That(t, normalizeWeight(100)).Equal(100)
}

func TestSplitAddr(t *testing.T) {
	// A well-formed address splits into host and numeric port.
	host, port, err := netutil.SplitHostPort("10.0.0.5:8080")
	assert.Error(t, err).Nil()
	assert.That(t, host).Equal("10.0.0.5")
	assert.That(t, port).Equal(uint64(8080))

	// A missing port fails fast before any Nacos call.
	_, _, err = netutil.SplitHostPort("no-port")
	assert.Error(t, err).Matches("must be host:port")

	// A non-numeric port fails fast too.
	_, _, err = netutil.SplitHostPort("host:abc")
	assert.Error(t, err).Matches("non-numeric port")
}

// TestRegisterMapsInstanceParams pins the neutral-to-Nacos mapping: the
// advertised addr splits into ip/port, the block's group/cluster scope the
// write, the instance is always ephemeral (the SDK heartbeat contract), and
// metadata rides along. A negative weight clamps to 1 while 0 — the drain
// signal — passes through untouched.
func TestRegisterMapsInstanceParams(t *testing.T) {
	r, client := newTestRegistry(false)
	meta := map[string]string{"zone": "a"}

	assert.Error(t, r.Register(context.Background(), discovery.Instance{
		ServiceName: "orders-register-params", Addr: "10.0.0.5:8080", Weight: -5, Metadata: meta,
	})).Nil()
	p := client.lastRegistered()
	assert.That(t, p.Ip).Equal("10.0.0.5")
	assert.That(t, p.Port).Equal(uint64(8080))
	assert.That(t, p.ServiceName).Equal("orders-register-params")
	assert.That(t, p.GroupName).Equal("DEFAULT_GROUP")
	assert.That(t, p.ClusterName).Equal("DEFAULT")
	assert.That(t, p.Ephemeral).True()
	assert.That(t, p.Enable).True()
	assert.That(t, p.Weight).Equal(float64(1)) // -5 clamps to 1
	assert.That(t, p.Metadata["zone"]).Equal("a")

	// Weight 0 is the drain signal and must survive the clamp.
	assert.Error(t, r.Register(context.Background(), discovery.Instance{
		ServiceName: "orders-register-params", Addr: "10.0.0.5:8080", Weight: 0,
	})).Nil()
	assert.That(t, client.lastRegistered().Weight).Equal(float64(0))
}

// TestDeregisterMapsInstanceParams pins the deregister mapping: the same
// ip/port split and group scope, ephemeral so the server drops the heartbeat
// with it.
func TestDeregisterMapsInstanceParams(t *testing.T) {
	r, client := newTestRegistry(false)

	assert.Error(t, r.Deregister(context.Background(), discovery.Instance{
		ServiceName: "orders-deregister-params", Addr: "10.0.0.5:8080",
	})).Nil()
	p := client.lastDeregistered()
	assert.That(t, p.Ip).Equal("10.0.0.5")
	assert.That(t, p.Port).Equal(uint64(8080))
	assert.That(t, p.ServiceName).Equal("orders-deregister-params")
	assert.That(t, p.GroupName).Equal("DEFAULT_GROUP")
	assert.That(t, p.Ephemeral).True()
}

// TestUpdateWeightMapsParams pins the hot-reload mapping: the new weight
// (0 = drain, passed through) replaces the old one while the rest of the
// advertisement — metadata included — stays as registered.
func TestUpdateWeightMapsParams(t *testing.T) {
	r, client := newTestRegistry(false)
	in := discovery.Instance{ServiceName: "orders-update-weight", Addr: "10.0.0.5:8080", Weight: 5, Metadata: map[string]string{"zone": "a"}}

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
	r, _ := newTestRegistry(true) // server-side rejection → failed span

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
		if attrs["discovery.system"] != obsSystem || attrs["discovery.service"] != "orders-span" {
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
	r, _ := newTestRegistry(false)
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
		if attrs["discovery.system"] != obsSystem || attrs["discovery.service"] != "orders-wspan" {
			continue
		}
		if _, hasReason := attrs["discovery.reason"]; hasReason {
			t.Fatal("a weight-change span must carry no reason attribute")
		}
		return
	}
	t.Fatal("no update_weight span with system=nacos was emitted")
}

// A successful publish is counted and marks the instance discoverable.
func TestRegisterSuccessIsReported(t *testing.T) {
	r, client := newTestRegistry(false)
	in := discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}

	assert.Error(t, r.Register(context.Background(), in)).Nil()

	regs, _, _ := client.counts()
	assert.Number(t, regs).Equal(1)
	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
		"reason": discovery.ReasonInitial, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Equal(int64(1))
}

// A server-side rejection (ok=false, no transport error) is a failed publish:
// the Nacos SDK reports it that way, and it must not be mistaken for success.
func TestRegisterServerRejectionIsReported(t *testing.T) {
	r, _ := newTestRegistry(true)
	in := discovery.Instance{ServiceName: "orders-rejected", Addr: "1.2.3.4:80", Weight: 1}

	err := r.Register(context.Background(), in)
	assert.Error(t, err).Matches("rejected by the server")

	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
		"reason": discovery.ReasonInitial, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Zero()
}

// The same rejection rule applies on the way out. A deregister the server
// rejects (ok=false, no transport error) must not be reported as one that
// succeeded: that would flip the published gauge to 0 while the instance is
// still in the registry — a leak hidden behind a gauge saying the opposite.
func TestDeregisterServerRejectionIsReported(t *testing.T) {
	r, client := newTestRegistry(false)
	in := discovery.Instance{ServiceName: "orders-drain-rejected", Addr: "1.2.3.4:80", Weight: 1}
	ctx := context.Background()

	// Publish first, so the gauge has a real published state to wrongly clear.
	assert.Error(t, r.Register(ctx, in)).Nil()
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Equal(int64(1))

	client.setReject(true)
	assert.Error(t, r.Deregister(ctx, in)).Matches("rejected by the server")

	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Equal(int64(1), "a rejected deregister leaves the instance published")
}

// Deregistration clears the published state; a weight change is reported as its
// own operation. Nacos has no self-healing re-register — liveness is the SDK's
// own ephemeral heartbeat — so reason=self_heal never appears for this backend.
func TestDeregisterAndWeightChangeAreReported(t *testing.T) {
	r, client := newTestRegistry(false)
	in := discovery.Instance{ServiceName: "orders-drain", Addr: "1.2.3.4:80", Weight: 1}
	ctx := context.Background()

	assert.Error(t, r.Register(ctx, in)).Nil()
	assert.Error(t, r.UpdateWeight(ctx, in, 0)).Nil()
	_, _, updated := client.counts()
	assert.Number(t, updated).Equal(1)

	assert.Error(t, r.Deregister(ctx, in)).Nil()
	_, deregistered, _ := client.counts()
	assert.Number(t, deregistered).Equal(1)
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Zero("a deregistered instance is no longer published")
}
