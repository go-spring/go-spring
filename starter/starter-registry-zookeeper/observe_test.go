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

package StarterRegistryZookeeper

import (
	"context"
	"errors"
	"os"
	"testing"

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

// The session monitor's recovery loop re-creates every registered node after a
// session loss. That path never passes through the Registrar interface, so this
// asserts it is reported anyway — with reason=self_heal, which is what
// separates "the ensemble lost my node" from the initial publish.
func TestSelfHealReRegistrationIsReported(t *testing.T) {
	r := &zkRegistrar{
		basePath: "/services",
		regs: map[string]instance{
			"/services/orders/orders-1.2.3.4:80": {
				ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1,
			},
		},
	}
	gauge := map[string]string{"system": obsSystem, "service": "orders"}

	// The session is not usable yet: the pass fails and the instance is not
	// discoverable, which is exactly what the gauge must say.
	r.reRegister = func(instance) error { return errors.New("ensemble unreachable") }
	assert.Error(t, r.reRegisterAll()).NotNil()
	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "orders",
		"reason": discovery.ReasonSelfHeal, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", gauge)).
		Zero("a session loss that could not be healed must report as unpublished")

	// The session recovers: healAll retries the pass and the node comes back.
	r.reRegister = func(instance) error { return nil }
	assert.Error(t, r.reRegisterAll()).Nil()
	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "orders",
		"reason": discovery.ReasonSelfHeal, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", gauge)).Equal(int64(1))
}

// The tag name is a user-facing config key: USAGE documents tuning this
// starter's logging via `logger.<name>.tag=_app_registry_zookeeper`, so pin it — a
// rename here has to come with a doc change.
func TestStarterTagName(t *testing.T) {
	const want = "_app_registry_zookeeper"
	for _, name := range log.GetAllTags() {
		if name == want {
			return
		}
	}
	t.Fatalf("tag %q not registered (got %v)", want, log.GetAllTags())
}
