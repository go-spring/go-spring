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

package flatten

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// newFallback builds the "spring.cassandra" shape: an instances bucket that
// falls back to the sibling default bucket.
func newFallback(data map[string]string) *FallbackStorage {
	return WithFallback(NewPropertiesStorage(NewProperties(data)),
		"spring.cassandra.instances", "spring.cassandra.default")
}

func TestFallbackValue(t *testing.T) {
	s := newFallback(map[string]string{
		"spring.cassandra.default.timeout":          "5s",
		"spring.cassandra.instances.main.timeout":   "9s",
		"spring.cassandra.instances.main.keyspace":  "ks",
		"spring.cassandra.default.consistency":      "one",
		"spring.cassandra.instances.other.keyspace": "other",
	})

	// The instance's own leaf wins over the fallback.
	v, ok := s.Value("spring.cassandra.instances.main.timeout")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("9s")

	// An absent leaf falls back to default.
	v, ok = s.Value("spring.cassandra.instances.main.consistency")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("one")

	// A leaf set in both buckets still reads the instance's.
	v, ok = s.Value("spring.cassandra.instances.other.consistency")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("one")

	// Absent in both: no value.
	_, ok = s.Value("spring.cassandra.instances.main.username")
	assert.That(t, ok).False()

	// A key outside the instance prefix never falls back.
	_, ok = s.Value("spring.cassandra.default.timeout")
	assert.That(t, ok).True()
}

// An instance that leaves a whole field alone inherits it from the default —
// struct fields included, not just scalars.
func TestInstanceInheritsWholeStruct(t *testing.T) {
	s := newFallback(map[string]string{
		"spring.cassandra.default.tls.enabled":     "true",
		"spring.cassandra.default.tls.server-name": "db",
		"spring.cassandra.instances.main.keyspace": "ks",
	})

	got := map[string]struct{}{}
	assert.That(t, s.MapKeys("spring.cassandra.instances.main.tls", got)).True()
	assert.That(t, got).Equal(map[string]struct{}{"enabled": {}, "server-name": {}})

	v, ok := s.Value("spring.cassandra.instances.main.tls.server-name")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("db")
}

// Setting a field to EMPTY is an override, not an absence: the instance must
// not inherit the default just because its own value is the zero string. This
// is why the mechanism works on key existence, never on emptiness.
func TestInstanceEmptyValueOverridesDefault(t *testing.T) {
	s := newFallback(map[string]string{
		"spring.cassandra.default.username":        "admin",
		"spring.cassandra.instances.main.username": "",
	})

	v, ok := s.Value("spring.cassandra.instances.main.username")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("")
}

// A leaf is overridden on its own: the instance's leaf wins, and the default
// still supplies the leaves around it — even a sibling inside the same struct.
func TestFallbackLeafIsOverriddenAlone(t *testing.T) {
	s := newFallback(map[string]string{
		"spring.cassandra.default.tls.enabled":        "true",
		"spring.cassandra.default.tls.server-name":    "db",
		"spring.cassandra.instances.main.tls.enabled": "false",
	})

	v, ok := s.Value("spring.cassandra.instances.main.tls.enabled")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("false") // the instance's leaf

	v, ok = s.Value("spring.cassandra.instances.main.tls.server-name")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("db") // the sibling still comes from the default
}

func TestFallbackMapKeys(t *testing.T) {
	s := newFallback(map[string]string{
		"spring.cassandra.default.metadata.zone":        "cn",
		"spring.cassandra.default.metadata.version":     "v1",
		"spring.cassandra.instances.main.metadata.zone": "us",
	})

	// The instance defines one entry: its own key plus the default's other
	// key, so an entry is inherited one at a time — and the listing agrees
	// with what Value returns for it.
	got := map[string]struct{}{}
	assert.That(t, s.MapKeys("spring.cassandra.instances.main.metadata", got)).True()
	assert.That(t, got).Equal(map[string]struct{}{"zone": {}, "version": {}})
	v, ok := s.Value("spring.cassandra.instances.main.metadata.version")
	assert.That(t, ok).True()
	assert.That(t, v).Equal("v1")

	// Instance does not define the map: the default's keys are listed.
	got = map[string]struct{}{}
	assert.That(t, s.MapKeys("spring.cassandra.instances.other.metadata", got)).True()
	assert.That(t, got).Equal(map[string]struct{}{"zone": {}, "version": {}})

	// Neither side defines it.
	got = map[string]struct{}{}
	assert.That(t, s.MapKeys("spring.cassandra.instances.other.missing", got)).False()
}

// An array is inherited whole — and its elements agree with it: as soon as the
// instance defines one element, the elements it does not define are gone, not
// silently filled in from the default.
func TestFallbackArrayIsAllOrNothing(t *testing.T) {
	s := newFallback(map[string]string{
		"spring.cassandra.default.hosts[0]":        "10.0.0.1",
		"spring.cassandra.default.hosts[1]":        "10.0.0.2",
		"spring.cassandra.instances.main.hosts[0]": "127.0.0.1",
	})

	// The instance defines the array: only its own element, no mixing.
	got := map[string]string{}
	assert.That(t, s.SliceEntries("spring.cassandra.instances.main.hosts", got)).True()
	assert.That(t, got).Equal(map[string]string{"spring.cassandra.instances.main.hosts[0]": "127.0.0.1"})

	// ...and the element it does not define does not exist, matching the listing.
	assert.That(t, s.Exists("spring.cassandra.instances.main.hosts[1]")).False()
	_, ok := s.Value("spring.cassandra.instances.main.hosts[1]")
	assert.That(t, ok).False()

	// An instance that defines no element inherits the whole array, re-keyed
	// under the key the caller asked for.
	got = map[string]string{}
	assert.That(t, s.SliceEntries("spring.cassandra.instances.other.hosts", got)).True()
	assert.That(t, got).Equal(map[string]string{
		"spring.cassandra.instances.other.hosts[0]": "10.0.0.1",
		"spring.cassandra.instances.other.hosts[1]": "10.0.0.2",
	})
	assert.That(t, s.Exists("spring.cassandra.instances.other.hosts[1]")).True()
}

func TestFallbackExists(t *testing.T) {
	s := newFallback(map[string]string{
		"spring.cassandra.default.timeout":         "5s",
		"spring.cassandra.instances.main.keyspace": "ks",
	})

	assert.That(t, s.Exists("spring.cassandra.instances.main.keyspace")).True() // instance leaf
	assert.That(t, s.Exists("spring.cassandra.instances.main.timeout")).True()  // fallback leaf
	assert.That(t, s.Exists("spring.cassandra.instances.main")).True()          // instance node
	assert.That(t, s.Exists("spring.cassandra.instances.other")).False()        // neither
}
