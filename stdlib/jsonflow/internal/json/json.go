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

package json

// Encoder defines a streaming JSON encoder interface.
//
// Only the lowest level interface is preserved,
// higher level requires some shallow encapsulation.
type Encoder interface {
	// WriteToken writes the next token and advances the encoder state.
	WriteToken(token string, kind Kind) error
	// WriteValue writes a JSON value to the encoder.
	WriteValue(v []byte) error
}

// Kind represents each possible JSON token kind with a single byte,
// which is conveniently the first byte of that kind's grammar
// with the restriction that numbers always be represented with '0':
//
//   - 'n': null
//   - 'f': false
//   - 't': true
//   - '"': string
//   - '0': number
//   - '{': object begin
//   - '}': object end
//   - '[': array begin
//   - ']': array end
//
// An invalid kind is usually represented using 0,
// but may be non-zero due to invalid JSON data.
type Kind byte

const InvalidKind Kind = 0

// Decoder defines a streaming JSON decoder interface.
//
// Only the lowest level interface is preserved,
// higher level requires some shallow encapsulation.
type Decoder interface {
	// PeekKind returns the Kind of the next token without consuming it.
	PeekKind() Kind
	// ReadToken reads the next token and returns its string value, kind, and error.
	ReadToken() (token string, _ Kind, _ error)
	// ReadValue reads the next value, which may be a complete JSON
	// node (object, array, or scalar), as bytes.
	ReadValue() (value []byte, _ error)
	// SkipValue skips the next value (maybe a complete JSON node).
	SkipValue() error
}
