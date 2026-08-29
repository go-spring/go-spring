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
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

func TestRandomSpread(t *testing.T) {
	// Statistical: 3000 draws over 3 candidates must land near uniform (bounds
	// are ±20% around the 1000 expectation, generous enough to never flake).
	b := NewRandom()
	m := map[string]int{}
	for range 3000 {
		ep, err := b.Pick(eps("a", "b", "c"), PickInfo{})
		assert.Error(t, err).Nil()
		m[ep.Addr]++
	}
	for _, addr := range []string{"a", "b", "c"} {
		assert.Number(t, m[addr]).GreaterThan(800)
		assert.Number(t, m[addr]).LessThan(1200)
	}
}

func TestRandomEmpty(t *testing.T) {
	b := NewRandom()
	_, err := b.Pick(nil, PickInfo{})
	assert.Error(t, err).Is(ErrNoAvailable)
}

func TestRandomSingleCandidate(t *testing.T) {
	b := NewRandom()
	set := []discovery.Endpoint{{Addr: "only"}}
	for range 5 {
		ep, err := b.Pick(set, PickInfo{})
		assert.Error(t, err).Nil()
		assert.String(t, ep.Addr).Equal("only")
	}
}
