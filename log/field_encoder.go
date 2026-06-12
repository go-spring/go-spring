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
	"encoding/json"
	"io"
	"math"
	"strconv"
	"unicode/utf8"
)

// Writer defines the interface for writing raw data.
type Writer interface {
	io.Writer
	io.ByteWriter
	io.StringWriter
}

// Encoder defines the interface for structured logging encoders.
// Implementations control how log fields are serialized (e.g. JSON, text).
type Encoder interface {
	AppendEncoderBegin()
	AppendEncoderEnd()
	AppendObjectBegin()
	AppendObjectEnd()
	AppendArrayBegin()
	AppendArrayEnd()
	AppendKey(key string)
	AppendBool(v bool)
	AppendInt64(v int64)
	AppendUint64(v uint64)
	AppendFloat64(v float64)
	AppendString(v string)
	AppendReflect(v any)
}

var (
	_ Encoder = (*JSONEncoder)(nil)
	_ Encoder = (*TextEncoder)(nil)
)

// JSONTokenType represents the type of the last token written to JSONEncoder.
// It is used to determine when separators (commas) are required.
type JSONTokenType int

const (
	JSONTokenUnknown JSONTokenType = iota
	JSONTokenObjectBegin
	JSONTokenObjectEnd
	JSONTokenArrayBegin
	JSONTokenArrayEnd
	JSONTokenKey
	JSONTokenValue
)

// JSONEncoder encodes log fields into standard JSON format.
type JSONEncoder struct {
	out  Writer        // Buffer to write JSON output.
	last JSONTokenType // The last token type written.
}

// NewJSONEncoder creates a new JSONEncoder.
func NewJSONEncoder(out Writer) *JSONEncoder {
	return &JSONEncoder{out: out, last: JSONTokenUnknown}
}

// Reset resets the encoder's state.
func (enc *JSONEncoder) Reset() {
	enc.last = JSONTokenUnknown
}

// AppendEncoderBegin writes the start of an encoder section.
func (enc *JSONEncoder) AppendEncoderBegin() {
	enc.AppendObjectBegin()
}

// AppendEncoderEnd writes the end of an encoder section.
func (enc *JSONEncoder) AppendEncoderEnd() {
	enc.AppendObjectEnd()
}

// AppendObjectBegin writes the beginning of a JSON object.
func (enc *JSONEncoder) AppendObjectBegin() {
	enc.appendSeparator()
	enc.last = JSONTokenObjectBegin
	_ = enc.out.WriteByte('{')
}

// AppendObjectEnd writes the end of a JSON object.
func (enc *JSONEncoder) AppendObjectEnd() {
	enc.last = JSONTokenObjectEnd
	_ = enc.out.WriteByte('}')
}

// AppendArrayBegin writes the beginning of a JSON array.
func (enc *JSONEncoder) AppendArrayBegin() {
	enc.appendSeparator()
	enc.last = JSONTokenArrayBegin
	_ = enc.out.WriteByte('[')
}

// AppendArrayEnd writes the end of a JSON array.
func (enc *JSONEncoder) AppendArrayEnd() {
	enc.last = JSONTokenArrayEnd
	_ = enc.out.WriteByte(']')
}

// appendSeparator writes a comma if the previous token
// requires separation (e.g., between values).
func (enc *JSONEncoder) appendSeparator() {
	if enc.last == JSONTokenObjectEnd || enc.last == JSONTokenArrayEnd || enc.last == JSONTokenValue {
		_ = enc.out.WriteByte(',')
	}
}

// AppendKey writes a JSON key.
func (enc *JSONEncoder) AppendKey(key string) {
	enc.appendSeparator()
	enc.last = JSONTokenKey
	_ = enc.out.WriteByte('"')
	WriteLogString(enc.out, key)
	_ = enc.out.WriteByte('"')
	_ = enc.out.WriteByte(':')
}

// AppendBool writes a boolean value.
func (enc *JSONEncoder) AppendBool(v bool) {
	enc.appendSeparator()
	enc.last = JSONTokenValue
	_, _ = enc.out.WriteString(strconv.FormatBool(v))
}

// AppendInt64 writes an int64 value.
func (enc *JSONEncoder) AppendInt64(v int64) {
	enc.appendSeparator()
	enc.last = JSONTokenValue
	_, _ = enc.out.WriteString(strconv.FormatInt(v, 10))
}

// AppendUint64 writes an uint64 value.
func (enc *JSONEncoder) AppendUint64(u uint64) {
	enc.appendSeparator()
	enc.last = JSONTokenValue
	_, _ = enc.out.WriteString(strconv.FormatUint(u, 10))
}

// AppendFloat64 writes a float64 value.
func (enc *JSONEncoder) AppendFloat64(v float64) {
	enc.appendSeparator()
	enc.last = JSONTokenValue
	if math.IsNaN(v) || math.IsInf(v, 0) {
		_ = enc.out.WriteByte('"')
		_, _ = enc.out.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
		_ = enc.out.WriteByte('"')
		return
	}
	_, _ = enc.out.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
}

// AppendString writes a string value with proper escaping.
func (enc *JSONEncoder) AppendString(v string) {
	enc.appendSeparator()
	enc.last = JSONTokenValue
	_ = enc.out.WriteByte('"')
	WriteLogString(enc.out, v)
	_ = enc.out.WriteByte('"')
}

// AppendReflect marshals any Go value to JSON and writes it.
// If marshalling fails, the error message is written as a string.
func (enc *JSONEncoder) AppendReflect(v any) {
	enc.appendSeparator()
	enc.last = JSONTokenValue
	b, err := json.Marshal(v)
	if err != nil {
		_ = enc.out.WriteByte('"')
		WriteLogString(enc.out, err.Error())
		_ = enc.out.WriteByte('"')
		return
	}
	_, _ = enc.out.Write(b)
}

// TextEncoder encodes fields as "key=value" pairs separated by a delimiter.
// For nested objects and arrays, it delegates to the embedded JSONEncoder.
type TextEncoder struct {
	out         Writer       // Buffer to write the encoded output
	separator   string       // Separator used between top-level key-value pairs
	jsonEncoder *JSONEncoder // Embedded JSON encoder for nested objects/arrays
	jsonDepth   int8         // Tracks depth of nested JSON structures
	hasWritten  bool         // Tracks if the first key-value has been written
}

// NewTextEncoder creates a new TextEncoder, using the specified separator.
func NewTextEncoder(out Writer, separator string) *TextEncoder {
	return &TextEncoder{
		out:         out,
		separator:   separator,
		jsonEncoder: &JSONEncoder{out: out},
	}
}

// AppendEncoderBegin writes the start of an encoder section.
func (enc *TextEncoder) AppendEncoderBegin() {}

// AppendEncoderEnd writes the end of an encoder section.
func (enc *TextEncoder) AppendEncoderEnd() {}

// AppendObjectBegin signals the start of a JSON object.
// Increments the depth and delegates to the JSON encoder.
func (enc *TextEncoder) AppendObjectBegin() {
	enc.jsonDepth++
	enc.jsonEncoder.AppendObjectBegin()
}

// AppendObjectEnd signals the end of a JSON object.
// Decrements the depth and resets the JSON encoder if back to top level.
func (enc *TextEncoder) AppendObjectEnd() {
	enc.jsonDepth--
	enc.jsonEncoder.AppendObjectEnd()
	if enc.jsonDepth == 0 {
		enc.jsonEncoder.Reset()
	}
}

// AppendArrayBegin signals the start of a JSON array.
// Increments the depth and delegates to the JSON encoder.
func (enc *TextEncoder) AppendArrayBegin() {
	enc.jsonDepth++
	enc.jsonEncoder.AppendArrayBegin()
}

// AppendArrayEnd signals the end of a JSON array.
// Decrements the depth and resets the JSON encoder if back to top level.
func (enc *TextEncoder) AppendArrayEnd() {
	enc.jsonDepth--
	enc.jsonEncoder.AppendArrayEnd()
	if enc.jsonDepth == 0 {
		enc.jsonEncoder.Reset()
	}
}

// AppendKey appends a key for a key-value pair.
// If inside a JSON structure, the key is handled by the JSON encoder.
// Otherwise, it's written directly with proper separator handling.
func (enc *TextEncoder) AppendKey(key string) {
	if enc.jsonDepth > 0 {
		enc.jsonEncoder.AppendKey(key)
		return
	}
	if enc.hasWritten {
		_, _ = enc.out.WriteString(enc.separator)
	} else {
		enc.hasWritten = true
	}
	WriteLogString(enc.out, key)
	_ = enc.out.WriteByte('=')
}

// AppendBool appends a boolean value, using JSON encoder if nested.
func (enc *TextEncoder) AppendBool(v bool) {
	if enc.jsonDepth > 0 {
		enc.jsonEncoder.AppendBool(v)
		return
	}
	_, _ = enc.out.WriteString(strconv.FormatBool(v))
}

// AppendInt64 appends an int64 value, using JSON encoder if nested.
func (enc *TextEncoder) AppendInt64(v int64) {
	if enc.jsonDepth > 0 {
		enc.jsonEncoder.AppendInt64(v)
		return
	}
	_, _ = enc.out.WriteString(strconv.FormatInt(v, 10))
}

// AppendUint64 appends a uint64 value, using JSON encoder if nested.
func (enc *TextEncoder) AppendUint64(v uint64) {
	if enc.jsonDepth > 0 {
		enc.jsonEncoder.AppendUint64(v)
		return
	}
	_, _ = enc.out.WriteString(strconv.FormatUint(v, 10))
}

// AppendFloat64 appends a float64 value, using JSON encoder if nested.
func (enc *TextEncoder) AppendFloat64(v float64) {
	if enc.jsonDepth > 0 {
		enc.jsonEncoder.AppendFloat64(v)
		return
	}
	_, _ = enc.out.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
}

// AppendString appends a string value, using JSON encoder if nested.
func (enc *TextEncoder) AppendString(v string) {
	if enc.jsonDepth > 0 {
		enc.jsonEncoder.AppendString(v)
		return
	}
	WriteLogString(enc.out, v)
}

// AppendReflect uses reflection to marshal any value as JSON.
// If nested, delegates to JSON encoder.
func (enc *TextEncoder) AppendReflect(v any) {
	if enc.jsonDepth > 0 {
		enc.jsonEncoder.AppendReflect(v)
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		WriteLogString(enc.out, err.Error())
		return
	}
	_, _ = enc.out.Write(b)
}

/************************************* string ********************************/

// WriteLogString escapes and writes a string according to JSON rules.
func WriteLogString(out Writer, s string) {
	for i := 0; i < len(s); {
		// Try to add a single-byte (ASCII) character directly
		if tryAddRuneSelf(out, s[i]) {
			i++
			continue
		}
		// Decode multi-byte UTF-8 character
		r, size := utf8.DecodeRuneInString(s[i:])
		// Handle invalid UTF-8 encoding
		if tryAddRuneError(out, r, size) {
			i++
			continue
		}
		// Valid multi-byte rune; add as is
		_, _ = out.WriteString(s[i : i+size])
		i += size
	}
}

// tryAddRuneSelf handles ASCII characters and escapes control/quote characters.
func tryAddRuneSelf(out Writer, b byte) bool {
	const _hex = "0123456789abcdef"
	if b >= utf8.RuneSelf {
		return false // not a single-byte rune
	}
	if 0x20 <= b && b != '\\' && b != '"' {
		_ = out.WriteByte(b)
		return true
	}
	// Handle escaping
	switch b {
	case '\\', '"':
		_ = out.WriteByte('\\')
		_ = out.WriteByte(b)
	case '\n':
		_ = out.WriteByte('\\')
		_ = out.WriteByte('n')
	case '\r':
		_ = out.WriteByte('\\')
		_ = out.WriteByte('r')
	case '\t':
		_ = out.WriteByte('\\')
		_ = out.WriteByte('t')
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		_, _ = out.WriteString(`\u00`)
		_ = out.WriteByte(_hex[b>>4])
		_ = out.WriteByte(_hex[b&0xF])
	}
	return true
}

// tryAddRuneError checks and escapes invalid UTF-8 runes.
func tryAddRuneError(out Writer, r rune, size int) bool {
	if r == utf8.RuneError && size == 1 {
		_, _ = out.WriteString(`\ufffd`)
		return true
	}
	return false
}
