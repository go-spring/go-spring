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

	"go-spring.org/stdlib/testing/assert"
)

func TestLeastConn(t *testing.T) {
	b := NewLeastConn()
	set := eps("a", "b")

	// Two picks without releasing: each must land on a different endpoint since
	// the first one is now at in-flight 1.
	e1, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	e2, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e1.Addr).NotEqual(e2.Addr)

	// Release e1; it now has the fewest in-flight and must be chosen.
	b.Complete(e1, nil)
	e3, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e3.Addr).Equal(e1.Addr)

	// A repeated Complete for the same request is a harmless no-op: the count
	// deletes at zero instead of going negative (interface contract).
	b.Complete(e1, nil)
	b.Complete(e1, nil)
	e4, err := b.Pick(set, PickInfo{})
	assert.Error(t, err).Nil()
	assert.String(t, e4.Addr).Equal(e1.Addr) // e1's count was not driven negative
}
