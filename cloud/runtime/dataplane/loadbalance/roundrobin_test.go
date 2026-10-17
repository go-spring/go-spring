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

func TestRoundRobin(t *testing.T) {
	b := NewRoundRobin()
	m := counts(t, b, eps("a", "b", "c"), PickInfo{}, 9)
	assert.Number(t, m["a"]).Equal(3)
	assert.Number(t, m["b"]).Equal(3)
	assert.Number(t, m["c"]).Equal(3)

	_, err := b.Pick(nil, PickInfo{})
	assert.Error(t, err).Is(ErrNoAvailable)
}
