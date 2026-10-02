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

package StarterDiscoveryEtcd

import (
	"context"
	"os"
	"testing"
	"time"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
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
// assertions independent of which test happens to touch the instrumentation
// first; installing per-test would record nowhere.
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
// the test: an absent series is exactly the wiring bug these tests exist to catch.
func sumValue(t *testing.T, name string, want map[string]string) int64 {
	t.Helper()
	v, ok := trySumValue(t, name, want)
	if !ok {
		t.Fatalf("no %s datapoint for %v", name, want)
	}
	return v
}

// trySumValue is sumValue without the failure, for polling a counter that has
// not been recorded yet.
func trySumValue(t *testing.T, name string, want map[string]string) (int64, bool) {
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
						return p.Value, true
					}
				}
			}
		}
	}
	return 0, false
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

// waitFor polls cond until it holds or the deadline passes.
func waitFor(t *testing.T, cond func() bool, msg string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal(msg)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// TestRegisterMultiInstanceLive pins the BindEach assembly through a real gs
// container against a live etcd: two blocks become two backend beans, each
// contributing one discovery.Registry (what the discovery registration core
// collects) and one health.Indicator whose probe answers for its own cluster.
// Skips when no live etcd is reachable — the consul starter carries the
// no-docker version of the same assembly contract.
func TestRegisterMultiInstanceLive(t *testing.T) {
	addr := etcdTestAddr()
	if addr == "" {
		t.Skip("no live etcd at 127.0.0.1:2379 (or ETCD_TEST_ENDPOINTS); skipping live assembly test")
	}

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.discovery.etcd.one.endpoints", addr)
		app.Property("spring.discovery.etcd.two.endpoints", addr)
	}).RunTest(t, func(s *struct {
		Regs []discovery.Registry `autowire:""`
		Inds []*health.Indicator  `autowire:""`
	}) {
		if len(s.Regs) != 2 {
			t.Fatalf("want 2 registry beans (one per block), got %d", len(s.Regs))
		}

		names := map[string]bool{}
		for _, ind := range s.Inds {
			names[ind.Name] = true
			if err := ind.Probe(context.Background()); err != nil {
				t.Fatalf("indicator %s must probe UP against the live cluster: %v", ind.Name, err)
			}
		}
		if !names["discovery-etcd:one"] || !names["discovery-etcd:two"] {
			t.Fatalf("want discovery-etcd:one and discovery-etcd:two indicators, got %v", names)
		}
	})
}

// TestRegisterNotTriggered proves the OnProperty guard: with no
// ${spring.discovery.etcd.*} entries configured, no backend or indicator beans
// register.
func TestRegisterNotTriggered(t *testing.T) {
	gs.Web(false).RunTest(t, func(s *struct {
		Regs []discovery.Registry `autowire:""`
		Inds []*health.Indicator  `autowire:""`
	}) {
		if len(s.Regs) != 0 {
			t.Fatalf("no registry beans should register without config, got %d", len(s.Regs))
		}
		if len(s.Inds) != 0 {
			t.Fatalf("no indicators should register without config, got %d", len(s.Inds))
		}
	})
}

// The tag name is a user-facing config key: USAGE documents tuning this
// starter's logging via `logger.<name>.tag=_app_discovery_etcd`, so pin it — a
// rename here has to come with a doc change.
func TestStarterTagName(t *testing.T) {
	const want = "_app_discovery_etcd"
	for _, name := range log.GetAllTags() {
		if name == want {
			return
		}
	}
	t.Fatalf("tag %q not registered (got %v)", want, log.GetAllTags())
}
