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

package loadbalance

import (
	"strconv"
	"sync"
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestConsistentHashKeyAffinity(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 50})
	set := eps("a", "b", "c")

	first, err := b.Pick(set, PickInfo{HashKey: "user-42"})
	assert.Error(t, err).Nil()

	// The same key is stable across repeated picks.
	for range 20 {
		ep, err := b.Pick(set, PickInfo{HashKey: "user-42"})
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal(first.Addr)
	}
}

func TestConsistentHashUnkeyedRoundRobin(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 50})

	// An empty HashKey spreads over every endpoint instead of pinning one.
	m := counts(t, b, eps("a", "b", "c"), PickInfo{}, 9)
	assert.Number(t, len(m)).Equal(3)
}

func TestConsistentHashEmptySet(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 50})

	for _, info := range []PickInfo{{}, {HashKey: "user-42"}} {
		_, err := b.Pick(nil, info)
		assert.Error(t, err).Is(ErrNoEndpoints)
	}
}

// TestConsistentHashTopologyChange pins the whole point of consistent hashing:
// adding endpoints moves only a small fraction of keys, unlike a modulo scheme.
func TestConsistentHashTopologyChange(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 100})
	before := map[string]string{}
	keys := make([]string, 0, 200)
	for i := range 200 {
		k := "key-" + strconv.Itoa(i)
		keys = append(keys, k)
		ep, err := b.Pick(eps("a", "b", "c"), PickInfo{HashKey: k})
		assert.Error(t, err).Nil()
		before[k] = ep.Addr
	}

	// 4 endpoints join a 3-endpoint set: ~3/7 of keys should move. The bounds are
	// loose enough to be robust and tight enough to catch a modulo-style scheme
	// (which would move ~100%).
	moved := 0
	for _, k := range keys {
		ep, err := b.Pick(eps("a", "b", "c", "d", "e", "f", "g"), PickInfo{HashKey: k})
		assert.Error(t, err).Nil()
		if ep.Addr != before[k] {
			moved++
		}
	}
	assert.Number(t, moved).GreaterThan(40)
	assert.Number(t, moved).LessThan(120)
}

// TestConsistentHashReturnsFreshEndpoint pins that the ring caches the address
// SET only: an endpoint whose metadata changed between picks must come back with
// the new value, not the copy captured when the ring was built.
func TestConsistentHashReturnsFreshEndpoint(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 50})
	pick := func(version string) discovery.Endpoint {
		t.Helper()
		ep, err := b.Pick([]discovery.Endpoint{{
			Addr:     "a",
			Metadata: map[string]string{"version": version},
		}}, PickInfo{HashKey: "user-42"})
		assert.Error(t, err).Nil()
		return ep
	}

	assert.String(t, pick("v1").Metadata["version"]).Equal("v1")
	// Same address set: the ring does not rebuild, but the endpoint must still be
	// read from this pick's snapshot.
	assert.String(t, pick("v2").Metadata["version"]).Equal("v2")
}

// TestConsistentHashRingCache pins when the ring is rebuilt: a repeated or
// reordered address set reuses the published ring, while a changed set replaces
// it. It reads the snapshot pointer directly because the cache is the behavior
// under test.
func TestConsistentHashRingCache(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 10})
	pick := func(set []discovery.Endpoint, key string) {
		t.Helper()
		_, err := b.Pick(set, PickInfo{HashKey: key})
		assert.Error(t, err).Nil()
	}

	pick(eps("a", "b", "c"), "k1")
	first := b.rings.Load()
	assert.That(t, first != nil).True()

	pick(eps("a", "b", "c"), "k2")
	assert.That(t, b.rings.Load() == first).True() // same set: no rebuild

	pick(eps("c", "b", "a"), "k1")
	assert.That(t, b.rings.Load() == first).True() // reordered: same fingerprint

	pick(eps("a", "b", "d"), "k1")
	assert.That(t, b.rings.Load() != first).True() // changed set: rebuilt
}

// TestConsistentHashCachesAlternatingSets pins why more than one ring is kept:
// zone-aware balancing narrows the candidate set per call, so multiple zones
// alternate the fingerprint. Once both subsets are cached, further alternating
// picks must not rebuild or republish anything.
func TestConsistentHashCachesAlternatingSets(t *testing.T) {
	ch := NewConsistentHash(Selection{ConsistentHashReplicas: 10})
	z := NewZoneAware(ch, "zone")
	set := []discovery.Endpoint{
		{Addr: "a", Metadata: map[string]string{"zone": "z1"}},
		{Addr: "b", Metadata: map[string]string{"zone": "z2"}},
	}
	pick := func(zone string) {
		t.Helper()
		_, err := z.Pick(set, PickInfo{Zone: zone, HashKey: "user-42"})
		assert.Error(t, err).Nil()
	}

	pick("z1")
	pick("z2")
	warm := ch.rings.Load()

	for range 10 {
		pick("z1")
		pick("z2")
	}
	assert.That(t, ch.rings.Load() == warm).True() // both cached: no republish
}

// TestRingCacheEvictsOldest pins the cache bounds: the oldest ring is dropped
// when the entry count exceeds maxCachedRings.
func TestRingCacheEvictsOldest(t *testing.T) {
	var c *ringCache
	for i := range maxCachedRings + 1 {
		c = c.add(&ringSnapshot{
			fingerprint: ringFingerprint{n: i},
			nodes:       []ringNode{{point: uint32(i)}},
		})
	}
	assert.Number(t, len(c.snapshots)).Equal(maxCachedRings)
	assert.That(t, c.find(ringFingerprint{n: 0}) != nil).False()
	assert.That(t, c.find(ringFingerprint{n: maxCachedRings}) != nil).True()
}

// TestConsistentHashRingLayout pins the ring's shape: every endpoint contributes
// exactly replicas nodes and the nodes are sorted by position.
func TestConsistentHashRingLayout(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 5})
	set := eps("a", "b", "c")

	s := b.buildRing(set, fingerprintOf(set))
	assert.Number(t, len(s.nodes)).Equal(len(set) * 5)
	for i := 1; i < len(s.nodes); i++ {
		assert.That(t, s.nodes[i-1].point <= s.nodes[i].point).True()
	}
}

// TestConsistentHashBoundsReplicas pins the resource ceiling: a code-built
// selection cannot ask for an unbounded ring, and the exported type's zero value
// is usable (the default is resolved when the ring is built).
func TestConsistentHashBoundsReplicas(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: maxReplicas + 1})
	assert.Number(t, b.replicas).Equal(maxReplicas)

	var zero ConsistentHashBalancer
	ep, err := zero.Pick(eps("a", "b"), PickInfo{HashKey: "user-42"})
	assert.Error(t, err).Nil()
	assert.That(t, ep.Addr == "a" || ep.Addr == "b").True()
}

// TestConsistentHashKeyedPickDoesNotAllocate guards the hot-path promise: once
// the ring is current, a keyed pick is one atomic load plus a binary search and
// touches the heap neither for the fingerprint nor for the lookup.
func TestConsistentHashKeyedPickDoesNotAllocate(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 20})
	set := eps("a", "b", "c")
	_, err := b.Pick(set, PickInfo{HashKey: "warm-up"}) // build the ring
	assert.Error(t, err).Nil()

	allocs := testing.AllocsPerRun(100, func() {
		_, _ = b.Pick(set, PickInfo{HashKey: "user-42"})
	})
	assert.Number(t, allocs).Equal(0)
}

// TestConsistentHashConcurrentPicks exercises the atomic ring publication: picks
// run while the address set moves underneath them, and every pick still returns
// one of the endpoints it was given. Run with -race.
func TestConsistentHashConcurrentPicks(t *testing.T) {
	b := NewConsistentHash(Selection{ConsistentHashReplicas: 50})
	sets := [][]discovery.Endpoint{
		eps("a", "b", "c"),
		eps("a", "b", "d"),
	}
	valid := map[string]bool{"a": true, "b": true, "c": true, "d": true}

	var wg sync.WaitGroup
	for i := range 8 {
		set := sets[i%len(sets)]
		wg.Go(func() {
			for j := range 100 {
				ep, err := b.Pick(set, PickInfo{HashKey: "user-" + strconv.Itoa(j)})
				assert.Error(t, err).Nil()
				assert.That(t, valid[ep.Addr]).True()
			}
		})
	}
	wg.Wait()
}

func TestFingerprintOrderIndependent(t *testing.T) {
	a := eps("a", "b", "c")
	b := eps("c", "a", "b")
	assert.That(t, fingerprintOf(a)).Equal(fingerprintOf(b))
	assert.That(t, fingerprintOf(a)).NotEqual(fingerprintOf(eps("a", "b")))
}

func BenchmarkConsistentHashPick(b *testing.B) {
	bal := NewConsistentHash(Selection{ConsistentHashReplicas: 100})
	set := eps("a", "b", "c", "d", "e")
	_, _ = bal.Pick(set, PickInfo{HashKey: "warm-up"}) // build the ring

	b.Run("keyed", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bal.Pick(set, PickInfo{HashKey: "user-42"})
		}
	})
	b.Run("unkeyed", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			_, _ = bal.Pick(set, PickInfo{})
		}
	})
}
