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

package StarterRegistryK8s

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"go-spring.org/log"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// testReader collects the instruments this starter emits. This backend reports
// the read side only (no registrar), so there is no span to capture — a meter
// provider is the whole harness.
var testReader sdkmetric.Reader

// TestMain installs the in-memory meter provider before any test runs. The OTel
// global meter binds to the FIRST provider set, so installing here keeps these
// assertions independent of which test happens to touch the instrumentation
// first; installing per-test would record nowhere.
func TestMain(m *testing.M) {
	prevMP := otel.GetMeterProvider()

	testReader = sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(testReader))

	otel.SetMeterProvider(mp)
	code := m.Run()
	otel.SetMeterProvider(prevMP)
	_ = mp.Shutdown(context.Background())
	os.Exit(code)
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

// sumValue returns the counter value carrying want. A missing datapoint fails
// the test: an absent series is exactly the wiring bug this file exists to catch.
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

// floatGaugeValue returns the float64 gauge value carrying want.
func floatGaugeValue(t *testing.T, name string, want map[string]string) float64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.Error(t, testReader.Collect(context.Background(), &rm)).Nil()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if g, ok := m.Data.(metricdata.Gauge[float64]); ok {
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

// errResolver is a dnsResolver whose every lookup fails, driving the failure
// side of the read-side reporting without a cluster.
type errResolver struct{ err error }

func (r errResolver) LookupSRV(_ context.Context, _, _, _ string) (string, []*net.SRV, error) {
	return "", nil, r.err
}

func (r errResolver) LookupIPAddr(_ context.Context, _ string) ([]net.IPAddr, error) {
	return nil, r.err
}

func syncTestConfig() Config {
	return Config{
		Namespace:       "default",
		PortName:        "grpc",
		ClusterDomain:   "cluster.local",
		RefreshInterval: time.Minute,
	}
}

// The discovery side must report a failed sync: a backend that never syncs
// successfully is exactly the one whose cache age has to be observable.
func TestDiscoveryReportsFailedSync(t *testing.T) {
	d := newDNSDiscovery(syncTestConfig(), errResolver{err: errors.New("no such host")})

	if _, err := d.Resolve(context.Background(), "cart"); err == nil {
		t.Fatal("expected the first lookup to fail")
	}
	assert.Number(t, sumValue(t, "discovery.sync_total", map[string]string{
		"system": obsSystem, "service": "cart", "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, floatGaugeValue(t, "discovery.cache.age_seconds", map[string]string{
		"system": obsSystem, "service": "cart",
	})).GreaterOrEqual(0.0)
}

// A confirmed snapshot must be reported ok and start the freshness clock at
// zero; only a success advances it.
func TestDiscoveryReportsSuccessfulSync(t *testing.T) {
	f := &fakeResolver{}
	f.set([]*net.SRV{{Target: "10.0.0.1.", Port: 80}}, nil)
	d := newDNSDiscovery(syncTestConfig(), f)

	if _, err := d.Resolve(context.Background(), "orders"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	assert.Number(t, sumValue(t, "discovery.sync_total", map[string]string{
		"system": obsSystem, "service": "orders", "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, floatGaugeValue(t, "discovery.cache.age_seconds", map[string]string{
		"system": obsSystem, "service": "orders",
	})).LessThan(1.0, "a just-confirmed snapshot must read as fresh")
}

// The tag name is a user-facing config key: USAGE documents tuning this
// starter's logging via `logger.<name>.tag=_app_registry_k8s`, so pin it — a
// rename here has to come with a doc change.
func TestStarterTagName(t *testing.T) {
	const want = "_app_registry_k8s"
	for _, name := range log.GetAllTags() {
		if name == want {
			return
		}
	}
	t.Fatalf("tag %q not registered (got %v)", want, log.GetAllTags())
}
