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

package randutil_test

import (
	"testing"

	"go-spring.org/stdlib/randutil"
	"go-spring.org/stdlib/testing/assert"
)

func TestHex(t *testing.T) {
	s := randutil.Hex(16)
	assert.That(t, len(s)).Equal(32)
	for _, c := range s {
		assert.That(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')).True()
	}
	// Two calls differ (1-bit collision odds per pair are 2^-128).
	assert.That(t, s != randutil.Hex(16)).True()
}

func TestURLSafe(t *testing.T) {
	s := randutil.URLSafe(32)
	assert.That(t, len(s)).Equal(43) // 32 bytes → 43 unpadded base64 chars
	assert.That(t, s != randutil.URLSafe(32)).True()
}
