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

package StarterRegistryEtcd

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/testing/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
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
// the test: an absent series is exactly the wiring bug this file exists to catch.
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

// A failed initial publish must be counted and must leave the instance reported
// as unpublished — the state an operator has to be able to alert on.
func TestRegisterFailedPublishIsReported(t *testing.T) {
	r, fails, _ := newHealRegistrar()
	fails.set(errors.New("etcd down"))

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

// A successful publish marks the instance published.
func TestRegisterSuccessIsReported(t *testing.T) {
	ctx := context.Background()
	r, _, lastKA := newHealRegistrar()
	in := instance{ServiceName: "orders-ok", Addr: "1.2.3.4:80", Weight: 1}

	assert.Error(t, r.Register(ctx, in)).Nil()
	assert.Number(t, sumValue(t, "registry.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
		"reason": discovery.ReasonInitial, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Equal(int64(1))

	// Retire the watcher the successful register started.
	r.mu.Lock()
	h := r.holds[r.keyFor(in)]
	r.mu.Unlock()
	h.stop()
	close(<-lastKA)
}

// The self-healing re-registration never passes through the Registrar interface,
// so this asserts it is reported anyway — with reason=self_heal, which is what
// separates "the center lost my instance" from the initial publish.
func TestSelfHealFailureIsReported(t *testing.T) {
	// Every re-publish fails, so the healing loop stays in the retry select that
	// honours stop() — a loop that had succeeded once would sit draining its
	// keep-alive channel and never observe the stop.
	r := &etcdRegistrar{
		keyPrefix:   "/services/",
		ttlSecs:     15,
		backoffBase: 2 * time.Millisecond,
		backoffCap:  5 * time.Millisecond,
		holds:       map[string]*hold{},
	}
	r.publish = func(*hold) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
		return nil, errors.New("etcd down")
	}

	h := newHold(instance{ServiceName: "payments", Addr: "1.2.3.4:80", Weight: 1})
	ka1 := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka1); close(done) }()
	close(ka1) // the keep-alive dies while etcd is unreachable

	// Wait on the metric itself rather than on the fake's bookkeeping: the
	// counter is recorded just after a publish returns, so polling the
	// bookkeeping would race the record.
	waitFor(t, func() bool {
		v, ok := trySumValue(t, "registry.registration.attempts_total", map[string]string{
			"system": obsSystem, "service": "payments",
			"reason": discovery.ReasonSelfHeal, "status": "failed",
		})
		return ok && v == 3
	}, "self-healing failures were never reported")

	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": "payments",
	})).Zero("an instance the center lost must report as unpublished")

	h.stop()
	<-done
}

// A successful self-heal reports the instance published again — the recovery
// edge of the same signal, so an alert on "registered == 0" clears by itself.
func TestSelfHealSuccessRestoresPublished(t *testing.T) {
	r, _, lastKA := newHealRegistrar()
	h := newHold(instance{ServiceName: "payments-back", Addr: "1.2.3.4:80", Weight: 1})
	ka1 := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka1); close(done) }()
	close(ka1) // dies, but the fake publish succeeds on the first retry

	waitFor(t, func() bool {
		v, ok := trySumValue(t, "registry.registration.attempts_total", map[string]string{
			"system": obsSystem, "service": "payments-back",
			"reason": discovery.ReasonSelfHeal, "status": "ok",
		})
		return ok && v == 1
	}, "the successful self-heal was never reported")

	assert.Number(t, intGaugeValue(t, "registry.instance.registered", map[string]string{
		"system": obsSystem, "service": "payments-back",
	})).Equal(int64(1))

	// Safe to read now: the fake sends the new channel before publish returns,
	// and the counter observed above is recorded only after that.
	h.stop()
	close(<-lastKA)
	<-done
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

// The discovery side must report a failed seed sync, which is what starts the
// freshness clock for a service that has never synced successfully. Pointing the
// client at a closed port drives the real failure path without a cluster.
func TestDiscoveryReportsFailedSeedSync(t *testing.T) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:1"},
		DialTimeout: 200 * time.Millisecond,
	})
	assert.Error(t, err).Nil()
	defer func() { _ = cli.Close() }()

	d := &etcdDiscovery{
		client: cli, keyPrefix: "/services/",
		bgCtx: context.Background(), entries: map[string]*serviceEntry{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if _, err := d.Resolve(ctx, "orders"); err == nil {
		t.Fatal("expected the seed fetch against a closed port to fail")
	}
	assert.Number(t, sumValue(t, "discovery.sync_total", map[string]string{
		"system": obsSystem, "service": "orders", "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, floatGaugeValue(t, "discovery.cache.age_seconds", map[string]string{
		"system": obsSystem, "service": "orders",
	})).GreaterOrEqual(0.0)
}

// TestDiscoveryReportsSuccessfulSeedSyncLive covers the success wiring against a
// real etcd: the seed sync is reported ok and the snapshot starts fresh. Skips
// when no live etcd is reachable.
func TestDiscoveryReportsSuccessfulSeedSyncLive(t *testing.T) {
	addr := etcdTestAddr()
	if addr == "" {
		t.Skip("no live etcd at 127.0.0.1:2379 (or ETCD_TEST_ENDPOINTS); skipping live sync test")
	}
	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{addr}, DialTimeout: 3 * time.Second})
	assert.Error(t, err).Nil()
	defer func() { _ = cli.Close() }()

	d := &etcdDiscovery{
		client: cli, keyPrefix: "/services/obs-test/",
		bgCtx: context.Background(), entries: map[string]*serviceEntry{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = d.Resolve(ctx, "orders")
	assert.Error(t, err).Nil()

	assert.Number(t, sumValue(t, "discovery.sync_total", map[string]string{
		"system": obsSystem, "service": "orders", "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, floatGaugeValue(t, "discovery.cache.age_seconds", map[string]string{
		"system": obsSystem, "service": "orders",
	})).LessThan(1.0, "a just-confirmed snapshot must read as fresh")
}

// The tag name is a user-facing config key: USAGE documents tuning this
// starter's logging via `logger.<name>.tag=_app_registry_etcd`, so pin it — a
// rename here has to come with a doc change.
func TestStarterTagName(t *testing.T) {
	const want = "_app_registry_etcd"
	for _, name := range log.GetAllTags() {
		if name == want {
			return
		}
	}
	t.Fatalf("tag %q not registered (got %v)", want, log.GetAllTags())
}
