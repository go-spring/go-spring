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

// TestManager_LookupByName pins the name -> backend contract: a configured
// name returns exactly the backend registered under it, and an unknown name
// misses without yielding a backend.
func TestManager_LookupByName(t *testing.T) {
	etcd := NewStaticDiscovery(Endpoint{Addr: "10.0.0.1:2379"})
	nacos := NewStaticDiscovery(Endpoint{Addr: "10.0.0.2:8848"})
	m := NewManager(map[string]Discovery{
		"etcd.main":  etcd,
		"nacos.main": nacos,
	})

	for _, tc := range []struct {
		name string
		want Discovery
	}{
		{"etcd.main", etcd},
		{"nacos.main", nacos},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := m.Get(tc.name)
			assert.That(t, ok).True()
			assert.That(t, got).Same(tc.want)
		})
	}

	got, ok := m.Get("zookeeper.main")
	assert.That(t, ok).False()
	assert.That(t, got).Nil()
}

// TestManager_NamesSorted pins that Names is deterministic (sorted), so a
// diagnostic that prints the configured names has a stable order, and that it
// hands out a copy instead of the Manager's own storage.
func TestManager_NamesSorted(t *testing.T) {
	m := NewManager(map[string]Discovery{
		"nacos.main":     NewStaticDiscovery(),
		"etcd.main":      NewStaticDiscovery(),
		"consul.default": NewStaticDiscovery(),
	})

	names := m.Names()
	assert.That(t, names).Equal([]string{"consul.default", "etcd.main", "nacos.main"})

	names[0] = "mutated"
	assert.That(t, m.Names()[0]).Equal("consul.default")
}

// TestManager_EmptyIsAMiss pins the no-backend case: a Manager over a nil map
// misses every lookup and reports no names, never a false hit.
func TestManager_EmptyIsAMiss(t *testing.T) {
	empty := NewManager(nil)

	got, ok := empty.Get("etcd.main")
	assert.That(t, ok).False()
	assert.That(t, got).Nil()
	assert.Slice(t, empty.Names()).Empty()
}
