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

package discovery

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// TestManager_LookupByLabel pins the label -> backend contract: a configured
// label hits the exact backend it was built with, an unknown one misses.
func TestManager_LookupByLabel(t *testing.T) {
	etcd := NewStaticDiscovery(Endpoint{Addr: "10.0.0.1:2379"})
	nacos := NewStaticDiscovery(Endpoint{Addr: "10.0.0.2:8848"})
	m := NewManager(map[string]Discovery{"etcd.main": etcd, "nacos.main": nacos})

	got, ok := m.Get("etcd.main")
	assert.That(t, ok).True()
	assert.That(t, got).Same(etcd)

	got, ok = m.Get("nacos.main")
	assert.That(t, ok).True()
	assert.That(t, got).Same(nacos)

	if _, ok = m.Get("zookeeper.main"); ok {
		t.Fatal("Get reported a hit for a label that was never configured")
	}
}

// TestManager_LabelsSorted pins that Labels is deterministic (sorted), so a
// diagnostic that prints the configured labels has a stable order.
func TestManager_LabelsSorted(t *testing.T) {
	m := NewManager(map[string]Discovery{
		"nacos.main":     NewStaticDiscovery(),
		"etcd.main":      NewStaticDiscovery(),
		"consul.default": NewStaticDiscovery(),
	})
	assert.That(t, m.Labels()).Equal([]string{"consul.default", "etcd.main", "nacos.main"})
}

// TestManager_NilAndEmptyAreMisses pins the nil-receiver and no-backend cases:
// both are valid "discovery not configured" states that must read as a miss (a
// nil Manager comes from a nil governance center; an empty one from a container
// with no discovery backend bean), never a false hit or a panic.
func TestManager_NilAndEmptyAreMisses(t *testing.T) {
	var absent *Manager
	if _, ok := absent.Get("etcd.main"); ok {
		t.Fatal("a nil Manager must miss, not hit")
	}
	assert.That(t, absent.Labels()).Nil()

	empty := NewManager(nil)
	if _, ok := empty.Get("etcd.main"); ok {
		t.Fatal("a Manager over a nil map must miss, not hit")
	}
	assert.That(t, empty.Labels()).Nil()
}
