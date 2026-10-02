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
	"sync"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// newTestObserver builds the observation layer a test's backend block would own.
// The identity is fixed (center "test"): what these tests exercise is the
// reporting, not the labelling.
func newTestObserver() *discovery.Observer {
	o, err := discovery.NewObserver(obsSystem, "test")
	if err != nil {
		panic(err)
	}
	return o
}

func TestServicePrefix(t *testing.T) {
	d := &etcdDiscovery{obs: newTestObserver(), keyPrefix: "/services/"}
	// The read prefix is exactly where etcdRegistry.keyFor writes.
	assert.That(t, d.servicePrefix("orders")).Equal("/services/orders/")
}

// mustKV builds one etcd key-value pair carrying v as its instance JSON.
func mustKV(t *testing.T, key string, v instanceValue) *mvccpb.KeyValue {
	t.Helper()
	b, err := json.Marshal(v)
	assert.That(t, err).Nil()
	return &mvccpb.KeyValue{Key: []byte(key), Value: b}
}

func TestKvsToEndpoints(t *testing.T) {
	kvs := []*mvccpb.KeyValue{
		mustKV(t, "/services/orders/a", instanceValue{ServiceName: "orders", Addr: "10.0.0.2:8080", Weight: 3,
			Version: "v2", Zone: "b", Scheme: "tls"}),
		mustKV(t, "/services/orders/b", instanceValue{ServiceName: "orders", Addr: "10.0.0.1:8080"}),
		// A malformed payload is skipped, not fatal to the snapshot.
		{Key: []byte("/services/orders/bad"), Value: []byte("not-json")},
	}
	eps := kvsToEndpoints(newTestObserver(), kvs)

	// Sorted by address, malformed entry dropped.
	assert.That(t, len(eps)).Equal(2)
	assert.That(t, eps[0].Addr).Equal("10.0.0.1:8080")
	assert.That(t, eps[1].Addr).Equal("10.0.0.2:8080")

	// Key liveness is the health signal: every decoded key is healthy.
	assert.That(t, eps[0].Healthy).True()
	assert.That(t, eps[1].Healthy).True()

	// Weight and the routing dimensions survive the mapping: scheme lands in
	// Endpoint.Scheme, version/zone surface through metadata.
	assert.That(t, eps[1].Weight).Equal(3)
	assert.That(t, eps[1].Scheme).Equal("tls")
	assert.That(t, eps[1].Metadata["version"]).Equal("v2")
	assert.That(t, eps[1].Metadata["zone"]).Equal("b")
}

func TestKvsToEndpointsSchemeFilter(t *testing.T) {
	kvs := []*mvccpb.KeyValue{
		mustKV(t, "/services/s/a", instanceValue{Addr: "10.0.0.1:80", Scheme: "tls"}),
		mustKV(t, "/services/s/b", instanceValue{Addr: "10.0.0.2:80"}),
	}
	// FilterByScheme narrows to the tls instance only.
	eps := discovery.FilterByScheme(kvsToEndpoints(newTestObserver(), kvs), "tls")
	assert.That(t, len(eps)).Equal(1)
	assert.That(t, eps[0].Addr).Equal("10.0.0.1:80")
}

func TestEndpointsKey(t *testing.T) {
	a := []discovery.Endpoint{{Addr: "10.0.0.1:80", Weight: 1}, {Addr: "10.0.0.2:80", Weight: 2}}
	b := []discovery.Endpoint{{Addr: "10.0.0.1:80", Weight: 1}, {Addr: "10.0.0.2:80", Weight: 3}}
	// Same set renders the same key; a weight change does not.
	assert.That(t, endpointsKey(a)).Equal(endpointsKey(a))
	assert.That(t, endpointsKey(a) != endpointsKey(b)).True()
}

// compile-time contract: the adapter satisfies the discovery interface.
var _ discovery.Discovery = (*etcdDiscovery)(nil)

// TestEtcdDiscoveryNoClientPanics guards the zero-value degenerate case: the
// backend always sets client, and nothing in the type relies on lazy
// initialization, so the pure helpers stay usable on a zero value.
func TestEtcdDiscoveryNoClientPanics(t *testing.T) {
	d := &etcdDiscovery{obs: newTestObserver(), keyPrefix: "/services/"}
	assert.That(t, d.servicePrefix("x")).Equal("/services/x/")
}

// fakeWatchSource is an in-process discoveryKV whose every watch comes back
// already cancelled by etcd: the stream carries the cancellation and is then
// closed. That is the failure a watch loop has to survive, and making it
// unconditional turns the re-arm into an observable second Watch call.
type fakeWatchSource struct {
	mu     sync.Mutex
	opened int
}

// watches reports how many watch streams have been opened.
func (f *fakeWatchSource) watches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opened
}

// Get serves an empty snapshot; the cancelled streams below mean the loop never
// reaches the refresh that would read it.
func (f *fakeWatchSource) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}

// Watch returns a stream that etcd has cancelled — a compaction being the
// everyday cause, and the one clientv3 reports through the response rather than
// by closing the channel.
func (f *fakeWatchSource) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse, 1)
	f.mu.Lock()
	f.opened++
	f.mu.Unlock()
	ch <- clientv3.WatchResponse{CompactRevision: 1}
	close(ch)
	return ch
}

// A watch cancelled by etcd carries no further events and never recovers on its
// own, so the loop has to report the failure and open a new watch. Without that
// the cache would freeze at its last value while every gauge still looked alive
// — a dead discovery looking exactly like a quiet cluster.
func TestWatchLoopReportsCancelAndReArms(t *testing.T) {
	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	f := &fakeWatchSource{}
	d := &etcdDiscovery{
		obs:    newTestObserver(),
		client: f, keyPrefix: "/services/",
		bgCtx: bgCtx, entries: map[string]*serviceEntry{},
	}
	done := make(chan struct{})
	go func() { d.watchLoop("orders-watch", d.entry("orders-watch")); close(done) }()

	// The cancellation reaches the freshness metric, not just the log.
	waitFor(t, func() bool {
		v, ok := trySumValue(t, "discovery.sync_total", map[string]string{
			"system": obsSystem, "service": "orders-watch", "status": "failed",
		})
		return ok && v >= 1
	}, "a cancelled watch was never reported as a failed sync")

	// And the loop re-arms instead of returning.
	waitFor(t, func() bool { return f.watches() >= 2 },
		"the cancelled watch was never re-armed")

	// Closing the backend retires the loop rather than leaving it spinning.
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not exit after the backend closed")
	}
}

// A stream that closes without etcd naming a reason is still an ended watch, and
// must not be mistaken for the quiet-cluster case.
func TestWatchLoopReportsClosedStream(t *testing.T) {
	bgCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	d := &etcdDiscovery{
		obs:    newTestObserver(),
		client: &closedStreamSource{}, keyPrefix: "/services/",
		bgCtx: bgCtx, entries: map[string]*serviceEntry{},
	}
	done := make(chan struct{})
	go func() { d.watchLoop("orders-closed", d.entry("orders-closed")); close(done) }()

	waitFor(t, func() bool {
		v, ok := trySumValue(t, "discovery.sync_total", map[string]string{
			"system": obsSystem, "service": "orders-closed", "status": "failed",
		})
		return ok && v >= 1
	}, "a closed watch stream was never reported as a failed sync")

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watch loop did not exit after the backend closed")
	}
}

// closedStreamSource is a discoveryKV whose watch closes immediately without a
// cancellation response.
type closedStreamSource struct{}

func (closedStreamSource) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	return &clientv3.GetResponse{}, nil
}

func (closedStreamSource) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	ch := make(chan clientv3.WatchResponse)
	close(ch)
	return ch
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
		obs:    newTestObserver(),
		client: etcdClient{cli}, keyPrefix: "/services/",
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
		obs:    newTestObserver(),
		client: etcdClient{cli}, keyPrefix: "/services/obs-test/",
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
