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

package StarterRegistryConsul

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
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

// closedPortClient returns a Consul client whose agent address has nothing
// listening: api.NewClient does not dial, so every call fails at the transport.
func closedPortClient(t *testing.T) *api.Client {
	t.Helper()
	client, err := api.NewClient(&api.Config{Address: "127.0.0.1:1"})
	assert.Error(t, err).Nil()
	return client
}

// reachableClient returns a Consul client pointed at a fake agent that answers
// any request with 200, so the write paths exercise their success branches.
func reachableClient(t *testing.T) *api.Client {
	t.Helper()
	srv := httptest.NewServer(new(fakeConsul))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&api.Config{Address: srv.URL})
	assert.Error(t, err).Nil()
	return client
}

// A failed initial publish must be counted and must leave the instance reported
// as unpublished.
func TestRegisterFailureIsReported(t *testing.T) {
	r := &consulRegistrar{
		client:     closedPortClient(t),
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]instance{},
	}

	err := r.Register(context.Background(), instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	assert.Error(t, err).NotNil()

	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "orders",
		"reason": discovery.ReasonInitial, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": "orders",
	})).Zero()
}

// The TTL heartbeat's self-healing escalation (reRegister) never passes through
// the Registrar interface, so this asserts it is reported anyway — with
// reason=self_heal, and with the gauge following each attempt's outcome.
func TestSelfHealIsReported(t *testing.T) {
	in := instance{ServiceName: "payments", Addr: "1.2.3.4:80", Weight: 1}
	id := serviceID(in)
	r := &consulRegistrar{
		client:     closedPortClient(t),
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]instance{id: in},
	}
	gauge := map[string]string{"system": obsSystem, "service": "payments"}

	// The agent is unreachable: the escalation fails and the instance is no
	// longer discoverable, which is the state worth alerting on.
	assert.Error(t, r.reRegister(id)).NotNil()
	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "payments",
		"reason": discovery.ReasonSelfHeal, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", gauge)).Zero()

	// The agent comes back: the same escalation succeeds and the gauge clears.
	r.client = reachableClient(t)
	assert.Error(t, r.reRegister(id)).Nil()
	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "payments",
		"reason": discovery.ReasonSelfHeal, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", gauge)).Equal(int64(1))
}

// A repeat Deregister is the normal shutdown path, not a failure: the registry
// core deregisters from both PreStop and the Stop fallback. Consul answering
// 404 for an id it no longer holds is that no-op, and it must be reported as
// success so a clean shutdown does not look like a wave of deregister failures.
func TestDeregisterRepeatIsANoOp(t *testing.T) {
	in := instance{ServiceName: "orders-repeat", Addr: "1.2.3.4:80", Weight: 1}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`Unknown service ID "orders-repeat-1.2.3.4:80"`))
	}))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&api.Config{Address: srv.URL})
	assert.Error(t, err).Nil()
	r := &consulRegistrar{
		client:     client,
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]instance{},
	}

	assert.Error(t, r.Deregister(context.Background(), in)).Nil()
	assert.Number(t, histCount(t, "registry.operation.duration", map[string]string{
		"system": obsSystem, "operation": "deregister", "service": in.ServiceName, "status": "ok",
	})).Equal(int64(1))
}

// Deregistration and weight re-advertisement are first-class operations on the
// duration metric — the drain path in particular, which the registry core does
// not log.
func TestDeregisterAndWeightChangeAreReported(t *testing.T) {
	in := instance{ServiceName: "orders-drain", Addr: "1.2.3.4:80", Weight: 1}
	id := serviceID(in)
	r := &consulRegistrar{
		client:     reachableClient(t),
		ttl:        time.Second,
		heartbeats: map[string]chan struct{}{},
		regs:       map[string]instance{id: in},
	}
	ctx := context.Background()

	assert.Error(t, r.UpdateWeight(ctx, in, 0)).Nil()
	assert.Number(t, histCount(t, "registry.operation.duration", map[string]string{
		"system": obsSystem, "operation": "update_weight", "service": in.ServiceName, "status": "ok",
	})).Equal(int64(1))

	assert.Error(t, r.Deregister(ctx, in)).Nil()
	assert.Number(t, histCount(t, "registry.operation.duration", map[string]string{
		"system": obsSystem, "operation": "deregister", "service": in.ServiceName, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Zero("a deregistered instance is no longer published")
}

// The tag name is a user-facing config key: USAGE documents tuning this
// starter's logging via `logger.<name>.tag=_app_registry_consul`, so pin it — a
// rename here has to come with a doc change.
func TestStarterTagName(t *testing.T) {
	const want = "_app_registry_consul"
	for _, name := range log.GetAllTags() {
		if name == want {
			return
		}
	}
	t.Fatalf("tag %q not registered (got %v)", want, log.GetAllTags())
}
