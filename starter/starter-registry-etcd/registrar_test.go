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
	"testing"
	"time"

	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestInstanceID(t *testing.T) {
	// An explicit ID is used verbatim.
	assert.That(t, instanceID(discovery.Registration{ID: "fixed", ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("fixed")
	// Otherwise it is derived from name and addr so restarts replace the entry.
	assert.That(t, instanceID(discovery.Registration{ServiceName: "orders", Addr: "1.2.3.4:80"})).Equal("orders-1.2.3.4:80")
}

func TestKeyFor(t *testing.T) {
	r := &etcdRegistrar{keyPrefix: "/services/"}
	got := r.keyFor(discovery.Registration{ServiceName: "orders", Addr: "1.2.3.4:80"})
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

func TestDeregisterIdempotent(t *testing.T) {
	// Deregistering an instance that was never registered is a no-op: it must
	// not touch the (nil) client, so shutdown can call it unconditionally as an
	// idempotent fallback after PreStop has already run.
	r := &etcdRegistrar{keyPrefix: "/services/", holds: map[string]*hold{}}
	reg := discovery.Registration{ServiceName: "orders", Addr: "1.2.3.4:80"}
	assert.That(t, r.Deregister(context.Background(), reg)).Nil()
	// A second call is likewise a no-op.
	assert.That(t, r.Deregister(context.Background(), reg)).Nil()
}
