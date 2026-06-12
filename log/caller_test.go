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

package log

import (
	"runtime"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

func TestCaller(t *testing.T) {

	t.Run("error skip", func(t *testing.T) {
		file, line := FastCaller(100)
		assert.String(t, file).Equal("")
		assert.That(t, line).Equal(0)
	})

	t.Run("fast false", func(t *testing.T) {
		_, file, line, _ := runtime.Caller(0)
		assert.String(t, file).Matches(".*/caller_test.go")
		assert.That(t, line).Equal(35)
	})

	t.Run("fast true", func(t *testing.T) {
		for range 2 {
			file, line := FastCaller(0)
			assert.String(t, file).Matches(".*/caller_test.go")
			assert.That(t, line).Equal(42)
		}
	})

	t.Run("cache behavior", func(t *testing.T) {
		file1, line1 := FastCaller(0)
		file2, line2 := FastCaller(0)
		assert.String(t, file1).Equal(file2)
		assert.Number(t, line1).Equal(line2 - 1)
	})

	t.Run("fast vs slow consistency", func(t *testing.T) {
		fileFast, lineFast := FastCaller(0)
		_, fileSlow, lineSlow, _ := runtime.Caller(0)
		assert.String(t, fileFast).Equal(fileSlow)
		assert.Number(t, lineFast).Equal(lineSlow - 1)
	})
}

func BenchmarkCaller(b *testing.B) {

	// BenchmarkCaller/fast_skip_0-8     12433761  95.05 ns/op
	// BenchmarkCaller/slow_skip_0-8      6314623  190.3 ns/op
	// BenchmarkCaller/fast_skip_1-8      9837133  122.2 ns/op
	// BenchmarkCaller/slow_skip_1-8      3601213  332.6 ns/op
	// BenchmarkCaller/fast_cache_hit-8  12281832  97.70 ns/op

	b.Run("fast skip 0", func(b *testing.B) {
		for b.Loop() {
			FastCaller(0)
		}
	})

	b.Run("slow skip 0", func(b *testing.B) {
		for b.Loop() {
			_, file, line, _ := runtime.Caller(0)
			_, _ = file, line
		}
	})

	b.Run("fast skip 1", func(b *testing.B) {
		for b.Loop() {
			FastCaller(1)
		}
	})

	b.Run("slow skip 1", func(b *testing.B) {
		for b.Loop() {
			_, file, line, _ := runtime.Caller(0)
			_, _ = file, line
		}
	})

	b.Run("fast cache hit", func(b *testing.B) {
		FastCaller(0)
		b.ResetTimer()
		for b.Loop() {
			FastCaller(0)
		}
	})
}
