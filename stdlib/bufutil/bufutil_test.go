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

package bufutil

import (
	"io"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestNew_PanicsOnNegativeCap(t *testing.T) {
	assert.Panic(t, func() { New(-1) }, "negative cap")
}

func TestNew_ZeroCapDiscardsEverything(t *testing.T) {
	b := New(0)
	n, err := b.Write([]byte("hello"))
	assert.That(t, err).Nil()
	assert.Number(t, n).Equal(5, "Write always reports the full size")
	assert.Number(t, b.Len()).Zero("a zero-cap buffer keeps nothing")
	assert.Number(t, len(b.Bytes())).Zero()
}

func TestWrite_KeepsUpToCapDropsOverflow(t *testing.T) {
	b := New(4)
	n, err := b.Write([]byte("hello world")) // 11 bytes into a 4-cap buffer
	assert.That(t, err).Nil()
	assert.Number(t, n).Equal(11, "overflow still reported as fully written")
	assert.Number(t, b.Len()).Equal(4, "only the cap is kept")
	assert.String(t, b.String()).Equal("hell")
}

func TestWrite_MultipleWritesAccumulateToCap(t *testing.T) {
	b := New(5)
	_, _ = b.Write([]byte("ab"))   // 2
	_, _ = b.Write([]byte("cde"))  // +3 = 5, at cap
	_, _ = b.Write([]byte("fghi")) // all dropped, still 5
	assert.String(t, b.String()).Equal("abcde")
	assert.Number(t, b.Len()).Equal(5)
}

func TestWriteString_MatchesWrite(t *testing.T) {
	b := New(3)
	n, err := b.WriteString("abcd")
	assert.That(t, err).Nil()
	assert.Number(t, n).Equal(4)
	assert.String(t, b.String()).Equal("abc")
}

// The defining property: feeding a LimitedBuffer via io.TeeReader must never
// backpressure or error the primary reader, even when the buffer is full. This is
// the contract starter-gin's body capture relies on.
func TestWrite_TeeReaderNeverBlocksOrErrorsOnOverflow(t *testing.T) {
	const cap = 8
	b := New(cap)
	src := io.NopCloser(strings.NewReader(strings.Repeat("x", 1000)))
	tee := io.TeeReader(src, b)

	got, err := io.ReadAll(tee)
	assert.That(t, err).Nil()
	// The primary flow read every byte - overflow into the buffer didn't stall it.
	assert.Number(t, len(got)).Equal(1000, "TeeReader read all 1000 bytes")
	// The capture is bounded to the cap.
	assert.Number(t, b.Len()).Equal(cap, "buffer kept only the cap")
}

func TestReset_AllowsReuse(t *testing.T) {
	b := New(4)
	_, _ = b.Write([]byte("aaaa"))
	assert.Number(t, b.Len()).Equal(4)
	b.Reset()
	assert.Number(t, b.Len()).Zero()
	_, _ = b.Write([]byte("bb"))
	assert.String(t, b.String()).Equal("bb")
	assert.Number(t, b.Len()).Equal(2)
}

func TestCap_ReturnsConfiguredMax(t *testing.T) {
	assert.Number(t, New(512).Cap()).Equal(512)
	assert.Number(t, New(0).Cap()).Zero()
}
