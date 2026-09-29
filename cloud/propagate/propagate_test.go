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

package propagate

import (
	"net/http"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// TestMultiMap covers the multi-map adapter: raw reads, nil-carrier reads, and
// Set replacing instead of appending.
func TestMultiMap(t *testing.T) {
	var m MultiMap
	assert.That(t, len(m.Keys())).Equal(0)
	assert.That(t, m.Values("k")).Nil()

	m = MultiMap{"k": {"a", "b"}}
	assert.That(t, m.Values("k")).Equal([]string{"a", "b"})
	assert.That(t, m.Values("missing")).Nil()

	m.Set("k", "c")
	assert.That(t, m.Values("k")).Equal([]string{"c"})
	m.Set("k", "d")
	assert.That(t, m.Values("k")).Equal([]string{"d"})

	keys := m.Keys()
	assert.That(t, len(keys)).Equal(1)
}

// TestStringMap covers the single-value adapter: an empty value reads as absent,
// and Set writes through to the map.
func TestStringMap(t *testing.T) {
	var s StringMap
	assert.That(t, s.Values("k")).Nil()

	s = StringMap{"k": "v", "empty": ""}
	assert.That(t, s.Values("k")).Equal([]string{"v"})
	assert.That(t, s.Values("empty")).Nil()
	assert.That(t, s.Values("missing")).Nil()
	assert.That(t, len(s.Keys())).Equal(2)

	s.Set("k", "w")
	assert.That(t, s["k"]).Equal("w")
}

// TestHeader covers the HTTP adapter: Values matches case-insensitively and
// Set writes canonical spelling, both by delegation to net/http.Header.
func TestHeader(t *testing.T) {
	var h Header
	assert.That(t, h.Values("X-K")).Nil()

	h = Header{}
	h.Set("x-k", "v")
	// Canonical spelling: "x-k" is stored as "X-K".
	assert.That(t, h["X-K"]).Equal([]string{"v"})
	assert.That(t, h.Values("x-k")).Equal([]string{"v"})
	assert.That(t, len(h.Keys())).Equal(1)

	// A view over the caller's own header map, not a copy.
	raw := http.Header{}
	hc := Header(raw)
	hc.Set("X-K", "v")
	assert.That(t, raw.Get("X-K")).Equal("v")
}

// TestRoundTrip pins the seam pair every marker owner builds on the carrier:
// what Set wrote is what Values reads back, through every adapter.
func TestRoundTrip(t *testing.T) {
	for _, c := range []Carrier{
		MultiMap{},
		StringMap{},
		Header{},
	} {
		c.Set("k", "v")
		assert.That(t, c.Values("k")).Equal([]string{"v"})
		c.Set("k", "w")
		assert.That(t, c.Values("k")).Equal([]string{"w"})
	}
}
