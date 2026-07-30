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

// Package bufutil provides bounded buffers for side-channel copying, where the
// copy must never backpressure or error the primary flow.
//
// A [LimitedBuffer] wraps a [bytes.Buffer] with a byte cap: writes past the cap
// are silently discarded, but Write always reports full success (it returns
// len(p), nil). That makes it the natural sink for an [io.TeeReader] that mirrors
// a request or response body into a capture buffer for logging - the body keeps
// flowing to the handler unchanged, while the capture is bounded so a runaway
// body can't exhaust memory.
//
// This is deliberately unlike a plain [bytes.Buffer], which grows unboundedly, and
// unlike [io.LimitReader], which only limits reads and errors on overflow: a
// LimitedBuffer is a write-side, error-free bound.
package bufutil

import "bytes"

// LimitedBuffer is a [bytes.Buffer] bounded by a byte cap. Writes beyond the cap
// are dropped (not errored), and Write always reports the full write size so a
// reader/writer feeding it - notably an [io.TeeReader] - never blocks or errors.
//
// A zero-value LimitedBuffer has cap 0 and discards everything; construct one with
// [New] to set a non-zero cap.
type LimitedBuffer struct {
	buf bytes.Buffer
	max int
}

// New returns a LimitedBuffer that keeps at most max bytes; further writes are
// discarded (and still reported as written). Panics if max < 0, since a negative
// cap is a programmer error, not a runtime condition to silently paper over.
func New(max int) *LimitedBuffer {
	if max < 0 {
		panic("bufutil: negative cap")
	}
	return &LimitedBuffer{max: max}
}

// Write appends p to the buffer up to the cap, discarding any overflow. It always
// returns len(p) and a nil error, so it satisfies io.Writer without ever
// backpressuring or erroring its caller - the property an io.TeeReader relies on.
func (b *LimitedBuffer) Write(p []byte) (int, error) {
	if b.buf.Len() < b.max {
		n := b.max - b.buf.Len()
		if n > len(p) {
			n = len(p)
		}
		b.buf.Write(p[:n])
	}
	return len(p), nil
}

// WriteString appends s up to the cap. It is a convenience wrapper around Write
// that avoids a []byte conversion at the call site.
func (b *LimitedBuffer) WriteString(s string) (int, error) {
	return b.Write([]byte(s))
}

// Bytes returns the buffered bytes (at most max). The slice aliases the buffer's
// internal storage, so it is invalidated by the next Write or Reset.
func (b *LimitedBuffer) Bytes() []byte {
	return b.buf.Bytes()
}

// String returns the buffered bytes as a string (at most max bytes).
func (b *LimitedBuffer) String() string {
	return b.buf.String()
}

// Len returns the number of buffered bytes (<= cap).
func (b *LimitedBuffer) Len() int {
	return b.buf.Len()
}

// Cap returns the maximum number of bytes the buffer keeps.
func (b *LimitedBuffer) Cap() int {
	return b.max
}

// Reset discards the buffer's contents, so it can be reused for the next capture
// (e.g. the next SSE event chunk on the same stream).
func (b *LimitedBuffer) Reset() {
	b.buf.Reset()
}
