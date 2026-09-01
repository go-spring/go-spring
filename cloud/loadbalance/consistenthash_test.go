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
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestConsistentHash(t *testing.T) {
	b := NewConsistentHash(50)
	set := eps("a", "b", "c")

	// Same key is stable across repeated picks.
	first, err := b.Pick(set, PickInfo{HashKey: "user-42"})
	assert.Error(t, err).Nil()
	for range 20 {
		ep, err := b.Pick(set, PickInfo{HashKey: "user-42"})
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal(first.Addr)
	}

	// Empty key falls back to round-robin spread rather than one instance.
	m := counts(t, b, set, PickInfo{}, 9)
	assert.Number(t, len(m)).Equal(3)
}

func TestConsistentHashTopologyChange(t *testing.T) {
	// Adding an endpoint must move only a small fraction of keys (the whole
	// point of consistent hashing): with 4 endpoints joining a 3-endpoint set
	// (~4/7 of traffic should shift in expectation), the vast majority of
	// 200 keys stay put.
	b := NewConsistentHash(100)
	before := map[string]string{}
	keys := make([]string, 0, 200)
	for i := range 200 {
		k := "key-" + strconv.Itoa(i)
		keys = append(keys, k)
		ep, err := b.Pick(eps("a", "b", "c"), PickInfo{HashKey: k})
		assert.Error(t, err).Nil()
		before[k] = ep.Addr
	}
	moved := 0
	for _, k := range keys {
		ep, err := b.Pick(eps("a", "b", "c", "d", "e", "f", "g"), PickInfo{HashKey: k})
		assert.Error(t, err).Nil()
		if ep.Addr != before[k] {
			moved++
		}
	}
	// Expected movement is 3/7 (~86 of 200) with some slack; a modulo-based
	// scheme would move ~100%.
	assert.Number(t, moved).LessThan(120)
	assert.Number(t, moved).GreaterThan(40)
}

func TestFingerprintOrderIndependent(t *testing.T) {
	a := eps("a", "b", "c")
	b := eps("c", "a", "b")
	assert.String(t, fingerprint(a)).Equal(fingerprint(b))
	assert.String(t, fingerprint(a)).NotEqual(fingerprint(eps("a", "b")))
}
