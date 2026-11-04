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

package StarterDiscoveryConsul

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

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

// histCount returns the number of records on the duration datapoint carrying
// want, failing the test when no such datapoint was collected.
func histCount(t *testing.T, name string, want map[string]string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	assert.Error(t, testReader.Collect(context.Background(), &rm)).Nil()
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if h, ok := m.Data.(metricdata.Histogram[float64]); ok {
				for _, p := range h.DataPoints {
					if attrsMatch(p.Attributes, want) {
						return int64(p.Count)
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

// TestRegisterMultiInstance pins the BindEach assembly through a real gs
// container: two blocks under ${spring.discovery.consul} become two backend
// beans, each contributing one discovery.Registry (what the discovery
// registration core collects) and one health.Indicator. The two fake agents
// stand in for two Consul clusters, so the whole construction path — client,
// startup probe, both halves — runs without docker.
func TestRegisterMultiInstance(t *testing.T) {
	srvA := httptest.NewServer(&fakeConsul{})
	defer srvA.Close()
	srvB := httptest.NewServer(&fakeConsul{})
	defer srvB.Close()

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.discovery.consul.one.address", srvA.URL)
		app.Property("spring.discovery.consul.two.address", srvB.URL)
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
				t.Fatalf("indicator %s must probe UP against its fake agent: %v", ind.Name, err)
			}
		}
		if !names["discovery-consul:one"] || !names["discovery-consul:two"] {
			t.Fatalf("want discovery-consul:one and discovery-consul:two indicators, got %v", names)
		}
	})
}

// TestRegisterNotTriggered proves the OnProperty guard: with no
// ${spring.discovery.consul.*} entries configured, no backend or indicator
// beans register.
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
// starter's logging via `logger.<name>.tag=_app_discovery_consul`, so pin it — a
// rename here has to come with a doc change.
func TestStarterTagName(t *testing.T) {
	const want = "_app_discovery_consul"
	for _, name := range log.GetAllTags() {
		if name == want {
			return
		}
	}
	t.Fatalf("tag %q not registered (got %v)", want, log.GetAllTags())
}
