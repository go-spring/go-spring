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
	"net"
	"os"
	"testing"
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/testing/assert"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// TestUpdateWeightUnregistered proves the guard: updating an instance that was
// never Register-ed is an explanatory error, not a silent no-op.
func TestUpdateWeightUnregistered(t *testing.T) {
	r := &etcdRegistrar{keyPrefix: "/services/", holds: map[string]*hold{}}
	err := r.UpdateWeight(context.Background(), instance{ServiceName: "orders", Addr: "1.2.3.4:80"}, 5)
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
// a real etcd: register two instances with unequal weights, watch the service,
// and show a loadbalance weighted balancer routes by those weights; then
// UpdateWeight (same lease, no re-register) and show the next watched snapshot
// flips the distribution. Requires a live etcd (example docker-compose); skips
// gracefully otherwise.
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

	reg := &etcdRegistrar{client: cli, keyPrefix: "/services/weight-test/", ttlSecs: 15, holds: map[string]*hold{}}
	disc := &etcdDiscovery{client: cli, keyPrefix: "/services/weight-test/"}

	const service = "orders"
	a := instance{ServiceName: service, ID: "a", Addr: "10.0.0.1:8080", Weight: 9}
	b := instance{ServiceName: service, ID: "b", Addr: "10.0.0.2:8080", Weight: 1}
	for _, in := range []instance{a, b} {
		assert.Error(t, reg.Register(ctx, in)).Nil()
	}
	t.Cleanup(func() {
		for _, in := range []instance{a, b} {
			_ = reg.Deregister(context.Background(), in)
		}
	})

	wch, err := disc.Watch(ctx, service)
	assert.Error(t, err).Nil()

	// pickCounts runs a fresh weighted balancer over n picks of eps.
	pickCounts := func(eps []discovery.Endpoint, n int) map[string]int {
		lb := loadbalance.NewWeighted()
		m := map[string]int{}
		for range n {
			r, err := lb.Pick(eps, loadbalance.PickInfo{})
			assert.Error(t, err).Nil()
			m[r.Endpoint.Addr]++
			if r.Done != nil {
				r.Done(loadbalance.DoneInfo{})
			}
		}
		return m
	}

	// Snapshot 1: a has weight 9, b weight 1 -> a dominates.
	snap1 := <-wch
	assert.Number(t, len(snap1.Endpoints)).Equal(2)
	m := pickCounts(snap1.Endpoints, 40)
	if m["10.0.0.1:8080"] <= 20 {
		t.Fatalf("weight 9 vs 1 expected a to dominate, got %v", m)
	}

	// Hot-reload on the SAME lease: a -> 1, b -> 9. No Register, no new lease.
	assert.Error(t, reg.UpdateWeight(ctx, a, 1)).Nil()
	assert.Error(t, reg.UpdateWeight(ctx, b, 9)).Nil()

	// Snapshot 2 arrives via the watch and flips the distribution.
	var snap2 []discovery.Endpoint
	for snap2 == nil {
		select {
		case wr, ok := <-wch:
			if !ok {
				t.Fatal("watch channel closed before weight change was delivered")
			}
			if wr.Err == nil && weightsEqual(wr.Endpoints, map[string]int{"10.0.0.1:8080": 1, "10.0.0.2:8080": 9}) {
				snap2 = wr.Endpoints
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for the weight-change snapshot")
		}
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
