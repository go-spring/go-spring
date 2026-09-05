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

package strutil

import (
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestTruncate(t *testing.T) {
	assert.String(t, Truncate("abc", 5)).Equal("abc")
	// The result never exceeds max bytes, marker included.
	assert.String(t, Truncate("abcdef", 5)).Equal("ab...")
	assert.String(t, Truncate("abcdef", 3)).Equal("abc")
	// A tiny max degenerates to a raw byte cut.
	assert.String(t, Truncate("abcdef", 2)).Equal("ab")
	// The cut lands on a rune boundary even when max splits one.
	assert.String(t, Truncate("你好世界", 7)).Equal("你...")
	assert.That(t, len(Truncate("你好世界", 7))).Equal(6)
}
