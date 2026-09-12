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
	"os"
	"sync"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// testReader collects the instruments this starter emits; testSpans holds its
// spans. Both are installed once in TestMain.
var (
	testReader sdkmetric.Reader
	testSpans  *tracetest.InMemoryExporter
)

// TestMain installs the in-memory OTel providers before any test runs. The OTel
// global meter binds to the FIRST provider set, so installing here keeps these
// assertions independent of which test touches the instrumentation first.
func TestMain(m *testing.M) {
	prevTP := otel.GetTracerProvider()
	prevMP := otel.GetMeterProvider()

	testSpans = tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(testSpans))
	testReader = sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(testReader))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	code := m.Run()
	otel.SetTracerProvider(prevTP)
	otel.SetMeterProvider(prevMP)
	_ = tp.Shutdown(context.Background())
	_ = mp.Shutdown(context.Background())
	os.Exit(code)
}

// fakeRegistrarClient adds the registrar half to fakeNamingClient: it records
// the write calls and can simulate a server-side rejection (ok=false with no
// transport error), which is a distinct failure mode in the Nacos SDK.
type fakeRegistrarClient struct {
	*fakeNamingClient

	mu           sync.Mutex
	registered   []vo.RegisterInstanceParam
	deregistered []vo.DeregisterInstanceParam
	updated      []vo.UpdateInstanceParam
	reject       bool
}

func (f *fakeRegistrarClient) RegisterInstance(p vo.RegisterInstanceParam) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.registered = append(f.registered, p)
	return !f.reject, nil
}

func (f *fakeRegistrarClient) DeregisterInstance(p vo.DeregisterInstanceParam) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deregistered = append(f.deregistered, p)
	return !f.reject, nil
}

func (f *fakeRegistrarClient) UpdateInstance(p vo.UpdateInstanceParam) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = append(f.updated, p)
	return !f.reject, nil
}

// counts reports how many writes of each kind the fake received.
func (f *fakeRegistrarClient) counts() (int, int, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.registered), len(f.deregistered), len(f.updated)
}

// attrsMatch reports whether a datapoint carries every wanted key/value.
func attrsMatch(attrs attribute.Set, want map[string]string) bool {
	for k, v := range want {
		got, ok := attrs.Value(attribute.Key(k))
		if !ok || got.AsString() != v {
			return false
		}
	}
	return true
}

// sumValue returns the counter value carrying want, failing the test when the
// datapoint is absent (a missing series is the wiring bug this file catches).
func sumValue(t *testing.T, name string, want map[string]string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.Error(t, testReader.Collect(context.Background(), &rm)).Nil()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if s, ok := m.Data.(metricdata.Sum[int64]); ok {
				for _, p := range s.DataPoints {
					if attrsMatch(p.Attributes, want) {
						return p.Value
					}
				}
			}
		}
	}
	t.Fatalf("no %s datapoint for %v", name, want)
	return 0
}

// intGaugeValue returns the int64 gauge value carrying want.
func intGaugeValue(t *testing.T, name string, want map[string]string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.Error(t, testReader.Collect(context.Background(), &rm)).Nil()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if g, ok := m.Data.(metricdata.Gauge[int64]); ok {
				for _, p := range g.DataPoints {
					if attrsMatch(p.Attributes, want) {
						return p.Value
					}
				}
			}
		}
	}
	t.Fatalf("no %s gauge for %v", name, want)
	return 0
}

// newTestRegistrar returns a nacos registrar over a fake naming client.
func newTestRegistrar(reject bool) (*nacosRegistrar, *fakeRegistrarClient) {
	client := &fakeRegistrarClient{fakeNamingClient: &fakeNamingClient{}, reject: reject}
	return &nacosRegistrar{client: client, group: "DEFAULT_GROUP", cluster: "DEFAULT"}, client
}

// A successful publish is counted and marks the instance discoverable.
func TestRegisterSuccessIsReported(t *testing.T) {
	r, client := newTestRegistrar(false)
	in := instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1}

	assert.Error(t, r.Register(context.Background(), in)).Nil()

	regs, _, _ := client.counts()
	assert.Number(t, regs).Equal(1)
	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
		"reason": discovery.ReasonInitial, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Equal(int64(1))
}

// A server-side rejection (ok=false, no transport error) is a failed publish:
// the Nacos SDK reports it that way, and it must not be mistaken for success.
func TestRegisterServerRejectionIsReported(t *testing.T) {
	r, _ := newTestRegistrar(true)
	in := instance{ServiceName: "orders-rejected", Addr: "1.2.3.4:80", Weight: 1}

	err := r.Register(context.Background(), in)
	assert.Error(t, err).Matches("rejected by the server")

	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
		"reason": discovery.ReasonInitial, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Zero()
}

// Deregistration clears the published state; a weight change is reported as its
// own operation. Nacos has no self-healing re-register — liveness is the SDK's
// own ephemeral heartbeat — so reason=self_heal never appears for this backend.
func TestDeregisterAndWeightChangeAreReported(t *testing.T) {
	r, client := newTestRegistrar(false)
	in := instance{ServiceName: "orders-drain", Addr: "1.2.3.4:80", Weight: 1}
	ctx := context.Background()

	assert.Error(t, r.Register(ctx, in)).Nil()
	assert.Error(t, r.UpdateWeight(ctx, in, 0)).Nil()
	_, _, updated := client.counts()
	assert.Number(t, updated).Equal(1)

	assert.Error(t, r.Deregister(ctx, in)).Nil()
	_, deregistered, _ := client.counts()
	assert.Number(t, deregistered).Equal(1)
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Zero("a deregistered instance is no longer published")
}

// The tag name is a user-facing config key: USAGE documents tuning this
// starter's logging via `logger.<name>.tag=_app_registry_nacos`, so pin it — a
// rename here has to come with a doc change.
func TestStarterTagName(t *testing.T) {
	const want = "_app_registry_nacos"
	for _, name := range log.GetAllTags() {
		if name == want {
			return
		}
	}
	t.Fatalf("tag %q not registered (got %v)", want, log.GetAllTags())
}
