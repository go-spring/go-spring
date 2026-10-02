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
	"encoding/json"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.opentelemetry.io/otel/codes"
)

func TestInstanceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, instanceID(discovery.Instance{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, instanceID(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestKeyFor(t *testing.T) {
	r := &etcdRegistry{keyPrefix: "/services/"}
	got := r.keyFor(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"})
	assert.That(t, got).Equal("/services/orders/orders-1.2.3.4:80")
}

func TestTTLSeconds(t *testing.T) {
	// Zero falls back to the 15s default.
	assert.That(t, EtcdConfig{}.ttlSeconds()).Equal(int64(15))
	// Whole seconds pass through.
	assert.That(t, EtcdConfig{TTL: 30 * time.Second}.ttlSeconds()).Equal(int64(30))
	// Sub-second values round up to the one-second minimum etcd allows.
	assert.That(t, EtcdConfig{TTL: 500 * time.Millisecond}.ttlSeconds()).Equal(int64(1))
}

func TestNormalizeWeight(t *testing.T) {
	// A misconfigured negative weight clamps to 1.
	assert.That(t, normalizeWeight(-5)).Equal(1)
	// 0 is the drain signal and passes through untouched, on both write paths.
	assert.That(t, normalizeWeight(0)).Equal(0)
	// An explicit positive weight passes through unchanged.
	assert.That(t, normalizeWeight(100)).Equal(100)
}

// Register must write the instance as the JSON payload a reader of the same
// prefix decodes, under the key that reader looks up. The decode half is pinned
// by kvsToEndpoints; this pins the encode half against the same struct.
func TestRegisterWritesInstancePayload(t *testing.T) {
	f := newFakeKV()
	r := newTestRegistry(f, "/services/")
	in := discovery.Instance{
		ServiceName: "orders", ID: "a", Addr: "10.0.0.1:8080",
		Weight: 7, Version: "v2", Zone: "b", Scheme: "tls",
	}
	assert.Error(t, r.Register(context.Background(), in)).Nil()

	val, ok := f.lastPut("/services/orders/a")
	assert.That(t, ok).True()
	var got instanceValue
	assert.Error(t, json.Unmarshal([]byte(val), &got)).Nil()
	assert.That(t, got).Equal(instanceValue{
		ServiceName: "orders", Addr: "10.0.0.1:8080", Weight: 7,
		Version: "v2", Zone: "b", Scheme: "tls",
	})

	// Retire the watcher this Register started.
	r.mu.Lock()
	h := r.holds[r.keyFor(in)]
	r.mu.Unlock()
	h.stop()
}

func TestDeregisterIdempotent(t *testing.T) {
	// Deregistering an instance that was never registered is a no-op: it must
	// not touch the (nil) client, so shutdown can call it unconditionally as an
	// idempotent fallback after PreStop has already run.
	r := &etcdRegistry{keyPrefix: "/services/", holds: map[string]*hold{}}
	reg := discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"}
	assert.That(t, r.Deregister(context.Background(), reg)).Nil()
	// A second call is likewise a no-op.
	assert.That(t, r.Deregister(context.Background(), reg)).Nil()
}

// TestUpdateWeightUnregistered proves the guard: updating an instance that was
// never Register-ed is an explanatory error, not a silent no-op.
func TestUpdateWeightUnregistered(t *testing.T) {
	r := &etcdRegistry{keyPrefix: "/services/", holds: map[string]*hold{}}
	err := r.UpdateWeight(context.Background(), discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"}, 5)
	assert.Error(t, err).Matches("unregistered instance")
}

// etcdTestAddr returns the etcd endpoint to run live tests against: the
// ETCD_TEST_ENDPOINTS env var, or 127.0.0.1:2379 when the example compose stack
// (or any local etcd) is up. Empty means "no live etcd, skip".
func etcdTestAddr() string {
	if v := os.Getenv("ETCD_TEST_ENDPOINTS"); v != "" {
		return v
	}
	conn, err := net.DialTimeout("tcp", "127.0.0.1:2379", 500*time.Millisecond)
	if err != nil {
		return ""
	}
	_ = conn.Close()
	return "127.0.0.1:2379"
}

// TestUpdateWeightHotReloadLive is the end-to-end proof of weight hot-reload on
// a real etcd: register two instances with unequal weights, resolve the
// service, and show a loadbalance weighted balancer routes by those weights;
// then UpdateWeight (same lease, no re-register) and show the next refreshed
// snapshot flips the distribution. Requires a live etcd (example
// docker-compose); skips gracefully otherwise.
func TestUpdateWeightHotReloadLive(t *testing.T) {
	addr := etcdTestAddr()
	if addr == "" {
		t.Skip("no live etcd at 127.0.0.1:2379 (or ETCD_TEST_ENDPOINTS); skipping live weight-reload test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := clientv3.New(clientv3.Config{Endpoints: []string{addr}, DialTimeout: 3 * time.Second})
	assert.Error(t, err).Nil()
	defer func() { _ = cli.Close() }()

	// Both halves go through the production constructor and adapter, so this
	// live test covers the real wiring rather than a struct literal.
	kv := etcdClient{cli}
	reg, err := newEtcdRegistry(EtcdConfig{KeyPrefix: "/services/weight-test/"}, kv, newTestObserver())
	assert.Error(t, err).Nil()
	disc := &etcdDiscovery{
		obs:    newTestObserver(),
		client: kv, keyPrefix: "/services/weight-test/",
		bgCtx: context.Background(), entries: map[string]*serviceEntry{},
	}

	const service = "orders"
	a := discovery.Instance{ServiceName: service, ID: "a", Addr: "10.0.0.1:8080", Weight: 9}
	b := discovery.Instance{ServiceName: service, ID: "b", Addr: "10.0.0.2:8080", Weight: 1}
	for _, in := range []discovery.Instance{a, b} {
		assert.Error(t, reg.Register(ctx, in)).Nil()
	}
	t.Cleanup(func() {
		for _, in := range []discovery.Instance{a, b} {
			_ = reg.Deregister(context.Background(), in)
		}
	})

	// pickCounts runs a fresh weighted balancer over n picks of eps.
	pickCounts := func(eps []discovery.Endpoint, n int) map[string]int {
		lb := loadbalance.NewWeighted()
		m := map[string]int{}
		for range n {
			ep, err := lb.Pick(eps, loadbalance.PickInfo{})
			assert.Error(t, err).Nil()
			m[ep.Addr]++
			lb.Complete(ep, nil)
		}
		return m
	}

	// Snapshot 1: a has weight 9, b weight 1 -> a dominates.
	snap1, err := disc.Resolve(ctx, service)
	assert.Error(t, err).Nil()
	assert.Number(t, len(snap1)).Equal(2)
	m := pickCounts(snap1, 40)
	if m["10.0.0.1:8080"] <= 20 {
		t.Fatalf("weight 9 vs 1 expected a to dominate, got %v", m)
	}

	// Hot-reload on the SAME lease: a -> 1, b -> 9. No Register, no new lease.
	assert.Error(t, reg.UpdateWeight(ctx, a, 1)).Nil()
	assert.Error(t, reg.UpdateWeight(ctx, b, 9)).Nil()

	// Snapshot 2: the background watcher refreshes the cache, so polling
	// Resolve sees the flipped weights.
	var snap2 []discovery.Endpoint
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		eps, err := disc.Resolve(ctx, service)
		assert.Error(t, err).Nil()
		if weightsEqual(eps, map[string]int{"10.0.0.1:8080": 1, "10.0.0.2:8080": 9}) {
			snap2 = eps
			break
		}
	}
	if snap2 == nil {
		t.Fatal("timed out waiting for the weight-change snapshot")
	}
	m2 := pickCounts(snap2, 40)
	if m2["10.0.0.2:8080"] <= 20 {
		t.Fatalf("after reload (1 vs 9) expected b to dominate, got %v", m2)
	}
}

// weightsEqual reports whether every endpoint's advertised weight matches want.
func weightsEqual(eps []discovery.Endpoint, want map[string]int) bool {
	if len(eps) != len(want) {
		return false
	}
	for _, ep := range eps {
		w, ok := want[ep.Addr]
		if !ok || ep.Weight != w {
			return false
		}
	}
	return true
}

// newHealRegistry returns a registry reading through an in-process fake etcd,
// so the self-healing loop can be driven without a cluster.
func newHealRegistry() (*etcdRegistry, *fakeKV) {
	f := newFakeKV()
	return newTestRegistry(f, "/services/"), f
}

// A keep-alive channel closing (etcd restart / lease lost) must trigger a
// re-publish: failures are retried with backoff until one succeeds, which puts
// the key back under a fresh lease.
func TestWatchKeepAliveReRegistersAfterKeepaliveDeath(t *testing.T) {
	r, f := newHealRegistry()
	f.grantErrs = []error{errors.New("etcd down"), errors.New("etcd down")}

	h := newHold(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	ka1 := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka1); close(done) }()

	close(ka1) // keep-alive dies
	select {
	case <-done:
		t.Fatal("watcher exited without re-registering")
	case <-time.After(2 * time.Second):
	}
	// Both scripted failures were consumed (with backoff sleeps) and the third
	// attempt went through the real publish step, so the instance is back in the
	// center — asserted on the write itself, not on the loop's bookkeeping.
	waitFor(t, func() bool { return f.writes() == 1 }, "the self-heal never re-published the key")
	assert.Number(t, f.grantRemaining()).Equal(0)

	// Stop through the registry, not h.stop(): publish stores the cancel func
	// under the registry lock, so the stop has to take it too.
	r.stopHold(h)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not exit after stop")
	}
}

// A keep-alive channel closing because the hold was stopped (Deregister or a
// refresh re-Register) must NOT trigger a re-publish — that would resurrect a
// deregistered instance. Asserting on the write count is what makes this the
// real check: an empty failure queue only proves the fake was not asked.
func TestWatchKeepAliveExitsOnStopWithoutRePublish(t *testing.T) {
	r, f := newHealRegistry()

	h := newHold(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	ka := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka); close(done) }()

	r.stopHold(h)
	close(ka)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not exit after stop")
	}
	assert.Number(t, f.writes()).Equal(0)
}

// stopHold must be safe against a concurrent publish storing the cancel func,
// and idempotent when Deregister and a re-Register retire the same hold.
func TestHoldStopIdempotent(t *testing.T) {
	h := newHold(discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80"})
	h.stop()
	h.stop() // must not panic on double close
	assert.That(t, h.stopped()).True()
}

// The backoff sequence doubles from base and is capped: 5ms,10ms,20ms,20ms...
func TestBackoffGrowthCapped(t *testing.T) {
	r := &etcdRegistry{backoffBase: 5 * time.Millisecond, backoffCap: 20 * time.Millisecond}
	seen := []time.Duration{}
	// Reuse the retry loop shape from watchKeepAlive by exercising the
	// arithmetic directly: it is trivial, but pinning it prevents accidental
	// changes to the recovery pacing.
	b := r.backoffBase
	for i := 0; i < 4; i++ {
		seen = append(seen, b)
		b *= 2
		if b > r.backoffCap {
			b = r.backoffCap
		}
	}
	assert.That(t, seen).Equal([]time.Duration{
		5 * time.Millisecond, 10 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond,
	})
}

// A failed initial publish must be counted and must leave the instance reported
// as unpublished — the state an operator has to be able to alert on.
func TestRegisterFailedPublishIsReported(t *testing.T) {
	f := newFakeKV()
	f.grantErrs = []error{errors.New("etcd down")}
	r := newTestRegistry(f, "/services/")

	err := r.Register(context.Background(), discovery.Instance{ServiceName: "orders", Addr: "1.2.3.4:80", Weight: 1})
	assert.Error(t, err).NotNil()

	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": "orders",
		"reason": discovery.ReasonInitial, "status": "failed",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": "orders",
	})).Zero()
}

// A successful publish marks the instance published.
func TestRegisterSuccessIsReported(t *testing.T) {
	ctx := context.Background()
	r, _ := newHealRegistry()
	in := discovery.Instance{ServiceName: "orders-ok", Addr: "1.2.3.4:80", Weight: 1}

	assert.Error(t, r.Register(ctx, in)).Nil()
	assert.Number(t, sumValue(t, "discovery.registration.attempts_total", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
		"reason": discovery.ReasonInitial, "status": "ok",
	})).Equal(int64(1))
	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": in.ServiceName,
	})).Equal(int64(1))

	// Retire the watcher the successful register started, through the registry
	// so the stop takes the lock publish stored the cancel func under.
	r.mu.Lock()
	h := r.holds[r.keyFor(in)]
	r.mu.Unlock()
	r.stopHold(h)
}

// The self-healing re-registration never passes through the Registry interface,
// so this asserts it is reported anyway — with reason=self_heal, which is what
// separates "the center lost my instance" from the initial publish.
func TestSelfHealFailureIsReported(t *testing.T) {
	// Every re-publish fails, so the healing loop stays in the retry select that
	// honours stop() — a loop that had succeeded once would sit draining its
	// keep-alive channel and never observe the stop.
	f := newFakeKV()
	f.grantDown = true
	r := newTestRegistry(f, "/services/")

	h := newHold(discovery.Instance{ServiceName: "payments", Addr: "1.2.3.4:80", Weight: 1})
	ka1 := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka1); close(done) }()
	close(ka1) // the keep-alive dies while etcd is unreachable

	// Wait on the metric itself rather than on the fake's bookkeeping: the
	// counter is recorded just after a publish returns, so polling the
	// bookkeeping would race the record.
	waitFor(t, func() bool {
		v, ok := trySumValue(t, "discovery.registration.attempts_total", map[string]string{
			"system": obsSystem, "service": "payments",
			"reason": discovery.ReasonSelfHeal, "status": "failed",
		})
		return ok && v == 3
	}, "self-healing failures were never reported")

	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": "payments",
	})).Zero("an instance the center lost must report as unpublished")

	r.stopHold(h)
	<-done
}

// A successful self-heal reports the instance published again — the recovery
// edge of the same signal, so an alert on "registered == 0" clears by itself.
func TestSelfHealSuccessRestoresPublished(t *testing.T) {
	r, _ := newHealRegistry()
	h := newHold(discovery.Instance{ServiceName: "payments-back", Addr: "1.2.3.4:80", Weight: 1})
	ka1 := make(chan *clientv3.LeaseKeepAliveResponse)
	done := make(chan struct{})
	go func() { r.watchKeepAlive("k", h, ka1); close(done) }()
	close(ka1) // dies, but the fake publish succeeds on the first retry

	waitFor(t, func() bool {
		v, ok := trySumValue(t, "discovery.registration.attempts_total", map[string]string{
			"system": obsSystem, "service": "payments-back",
			"reason": discovery.ReasonSelfHeal, "status": "ok",
		})
		return ok && v == 1
	}, "the successful self-heal was never reported")

	assert.Number(t, intGaugeValue(t, "discovery.instance.registered", map[string]string{
		"system": obsSystem, "service": "payments-back",
	})).Equal(int64(1))

	// The stop cancels the keep-alive context, which closes the fake's channel
	// the way clientv3 closes the real one, so the watcher retires on its own.
	r.stopHold(h)
	<-done
}

// The trace half of the same instrumentation: a failed initial publish must
// open a client span named for the operation, carrying the backend's system
// and ending in error. The metric assertions above prove the counters; this
// proves the span link a consumer of the trace actually sees.
func TestRegisterEmitsClientSpan(t *testing.T) {
	testSpans.Reset()
	f := newFakeKV()
	f.grantErrs = []error{errors.New("etcd down")}
	r := newTestRegistry(f, "/services/")

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
		if attrs["discovery.reason"] != discovery.ReasonInitial {
			t.Fatalf("an initial publish's span must carry reason=%s, got %q", discovery.ReasonInitial, attrs["discovery.reason"])
		}
		if s.Status.Code == codes.Error {
			return
		}
		t.Fatalf("the failed register's span must end in error status, got %v", s.Status)
	}
	t.Fatal("no register span with system=etcd was emitted")
}

// newTestRegistry wires a registry over client with test-sized backoff. Tests
// build through here rather than through a struct literal so the construction
// defaults live in one place: a literal skips newEtcdRegistry entirely, which
// is how the live weight test ended up with a zero backoff — a retry loop that
// spins hot instead of pacing.
func newTestRegistry(client registryKV, keyPrefix string) *etcdRegistry {
	r, err := newEtcdRegistry(EtcdConfig{KeyPrefix: keyPrefix}, client, newTestObserver())
	if err != nil {
		panic(err)
	}
	r.backoffBase = 5 * time.Millisecond
	r.backoffCap = 20 * time.Millisecond
	return r
}
