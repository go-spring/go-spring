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

package StarterLockEtcd

import (
	"testing"
	"time"

	"go-spring.org/cloud/lock"
	"go-spring.org/stdlib/testing/assert"
)

// The acquire/renew/release paths need a live etcd cluster (the concurrency
// package drives real leases), which unit tests here cannot provide; see
// example/ for a docker-gated smoke run. These tests therefore cover the pure
// conversion helpers the live path depends on.

func TestTTLSeconds(t *testing.T) {
	// Whole seconds pass through.
	assert.That(t, ttlSeconds(30*time.Second)).Equal(30)
	assert.That(t, ttlSeconds(15*time.Second)).Equal(15)
	// Sub-second values round up to the one-second floor etcd requires.
	assert.That(t, ttlSeconds(500*time.Millisecond)).Equal(1)
	assert.That(t, ttlSeconds(1500*time.Millisecond)).Equal(2)
	assert.That(t, ttlSeconds(10*time.Millisecond)).Equal(1)
	// Zero/negative falls back to the 30s package default.
	assert.That(t, ttlSeconds(0)).Equal(30)
	assert.That(t, ttlSeconds(-time.Second)).Equal(30)
}

// TestDefaults_OnlyTTL proves the starter layers only TTL beneath per-call
// opts: etcd manages lease keep-alive and blocking acquire internally, so a
// starter-level renew/retry default would be meaningless.
func TestDefaults_OnlyTTL(t *testing.T) {
	l := &etcdLocker{defaults: lock.DefaultOptions{TTL: 45 * time.Second}}

	o := lock.Resolve(l.defaults)
	assert.That(t, o.TTL).Equal(45 * time.Second)    // starter default applied
	assert.That(t, o.RenewInterval).Equal(o.TTL / 3) // package default, not a starter one
	assert.That(t, o.RetryInterval).Equal(100 * time.Millisecond)

	// A per-call WithTTL always wins over the starter default.
	o = lock.Resolve(l.defaults, lock.WithTTL(time.Second))
	assert.That(t, o.TTL).Equal(time.Second)
}
