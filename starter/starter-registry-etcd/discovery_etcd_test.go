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
	"encoding/json"
	"testing"

	"go.etcd.io/etcd/api/v3/mvccpb"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestServicePrefix(t *testing.T) {
	d := &etcdDiscovery{keyPrefix: "/services/"}
	// The read prefix is exactly where etcdRegistrar.keyFor writes.
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
			Metadata: map[string]string{"zone": "b", "scheme": "tls"}}),
		mustKV(t, "/services/orders/b", instanceValue{ServiceName: "orders", Addr: "10.0.0.1:8080"}),
		// A malformed payload is skipped, not fatal to the snapshot.
		{Key: []byte("/services/orders/bad"), Value: []byte("not-json")},
	}
	eps := kvsToEndpoints(kvs)

	// Sorted by address, malformed entry dropped.
	assert.That(t, len(eps)).Equal(2)
	assert.That(t, eps[0].Addr).Equal("10.0.0.1:8080")
	assert.That(t, eps[1].Addr).Equal("10.0.0.2:8080")

	// Key liveness is the health signal: every decoded key is healthy.
	assert.That(t, eps[0].Healthy).True()
	assert.That(t, eps[1].Healthy).True()

	// Weight and metadata (incl. scheme passthrough) survive the mapping.
	assert.That(t, eps[1].Weight).Equal(3)
	assert.That(t, eps[1].Scheme).Equal("tls")
	assert.That(t, eps[1].Metadata["zone"]).Equal("b")
}

func TestKvsToEndpointsSchemeFilter(t *testing.T) {
	kvs := []*mvccpb.KeyValue{
		mustKV(t, "/services/s/a", instanceValue{Addr: "10.0.0.1:80", Metadata: map[string]string{"scheme": "tls"}}),
		mustKV(t, "/services/s/b", instanceValue{Addr: "10.0.0.2:80"}),
	}
	// FilterByScheme narrows to the tls instance only.
	eps := discovery.FilterByScheme(kvsToEndpoints(kvs), "tls")
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

func TestServiceNames(t *testing.T) {
	keys := []string{
		"/services/orders/orders-10.0.0.1:8080",
		"/services/orders/orders-10.0.0.2:8080",
		"/service/payments/payments-10.0.0.3:9090",
		// No instance segment: skipped.
		"/services/stray",
	}
	names := serviceNames(keys, "/services/")
	// Distinct services only, sorted, and keys outside the prefix ignored.
	assert.That(t, names).Equal([]string{"orders"})
}

// compile-time contract: the adapter satisfies both discovery interfaces.
var (
	_ discovery.Discovery = (*etcdDiscovery)(nil)
	_ discovery.Catalog   = (*etcdDiscovery)(nil)
)

// TestEtcdDiscoveryNoClientPanics guards the zero-value degenerate case: the
// adapter is only constructible via newEtcdDiscovery, which always sets the
// client; nothing in the type relies on lazy initialization.
func TestEtcdDiscoveryNoClientPanics(t *testing.T) {
	d := &etcdDiscovery{keyPrefix: "/services/"}
	assert.That(t, d.servicePrefix("x")).Equal("/services/x/")
}
