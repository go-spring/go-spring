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

func TestWeighted(t *testing.T) {
	set := []discovery.Endpoint{
		{Addr: "a", Weight: 5},
		{Addr: "b", Weight: 1},
		{Addr: "c", Weight: 1},
	}
	b := NewWeighted()
	m := counts(t, b, set, PickInfo{}, 7)
	// Over one full cycle (sum of weights = 7) each endpoint gets exactly its
	// weight.
	assert.Number(t, m["a"]).Equal(5)
	assert.Number(t, m["b"]).Equal(1)
	assert.Number(t, m["c"]).Equal(1)

	// Smoothness: the 5-weight endpoint must not take all 5 slots up front —
	// b and c are interleaved within the cycle, so a never runs 5 in a row.
	b2 := NewWeighted()
	var seq []string
	for range 7 {
		ep, _ := b2.Pick(set, PickInfo{})
		seq = append(seq, ep.Addr)
	}
	run := 0
	maxRun := 0
	for i, s := range seq {
		if i > 0 && s == seq[i-1] {
			run++
		} else {
			run = 0
		}
		if run > maxRun {
			maxRun = run
		}
	}
	assert.Number(t, maxRun).LessThan(2) // no 3-in-a-row for the same endpoint

	// Zero weight is treated as weight 1.
	b3 := NewWeighted()
	m3 := counts(t, b3, eps("x", "y"), PickInfo{}, 4)
	assert.Number(t, m3["x"]).Equal(2)
	assert.Number(t, m3["y"]).Equal(2)
}
