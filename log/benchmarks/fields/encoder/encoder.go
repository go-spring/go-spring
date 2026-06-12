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

package encoder

import (
	"bytes"
	"encoding/json"
	"strconv"
)

// Encoder is an interface that defines methods for appending
// structured data elements.
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
	AppendFloat64(v float64)
	AppendString(v string)
	AppendReflect(v interface{})
}

// jsonToken represents the last written token type during
// the encoding process. It helps determine when separators
// (e.g., commas) should be added.
type jsonToken int

const (
	jsonTokenUnknown jsonToken = iota
	jsonTokenObjectBegin
	jsonTokenObjectEnd
	jsonTokenArrayBegin
	jsonTokenArrayEnd
	jsonTokenKey
	jsonTokenValue
)

// JSONEncoder encodes data into JSON format.
type JSONEncoder struct {
	buf  *bytes.Buffer // Buffer to write JSON output.
	last jsonToken     // The last token type written.
}

// NewJSONEncoder creates and initializes a new JSONEncoder
// with the provided buffer.
func NewJSONEncoder(buf *bytes.Buffer) *JSONEncoder {
	return &JSONEncoder{
		buf:  buf,
		last: jsonTokenUnknown,
	}
}

// AppendEncoderBegin writes the start of an encoder section.
func (enc *JSONEncoder) AppendEncoderBegin() {
	enc.AppendObjectBegin()
}

// AppendEncoderEnd writes the end of an encoder section.
func (enc *JSONEncoder) AppendEncoderEnd() {
	enc.AppendObjectEnd()
}

// AppendObjectBegin writes the opening brace of a JSON object.
func (enc *JSONEncoder) AppendObjectBegin() {
	enc.last = jsonTokenObjectBegin
	enc.buf.WriteByte('{')
}

// AppendObjectEnd writes the closing brace of a JSON object.
func (enc *JSONEncoder) AppendObjectEnd() {
	enc.last = jsonTokenObjectEnd
	enc.buf.WriteByte('}')
}

// AppendArrayBegin writes the opening bracket of a JSON array.
func (enc *JSONEncoder) AppendArrayBegin() {
	enc.last = jsonTokenArrayBegin
	enc.buf.WriteByte('[')
}

// AppendArrayEnd writes the closing bracket of a JSON array.
func (enc *JSONEncoder) AppendArrayEnd() {
	enc.last = jsonTokenArrayEnd
	enc.buf.WriteByte(']')
}

// appendSeparator writes a comma if needed to separate
// JSON values, array elements, or object fields.
func (enc *JSONEncoder) appendSeparator() {
	if enc.last == jsonTokenObjectEnd || enc.last == jsonTokenArrayEnd || enc.last == jsonTokenValue {
		enc.buf.WriteByte(',')
	}
}

// AppendKey writes a JSON object key followed by a colon.
func (enc *JSONEncoder) AppendKey(key string) {
	enc.appendSeparator()
	enc.last = jsonTokenKey
	enc.buf.WriteByte('"')
	enc.buf.WriteString(key)
	enc.buf.WriteByte('"')
	enc.buf.WriteByte(':')
}

// AppendBool writes a boolean value in JSON format.
func (enc *JSONEncoder) AppendBool(v bool) {
	enc.appendSeparator()
	enc.last = jsonTokenValue
	enc.buf.WriteString(strconv.FormatBool(v))
}

// AppendInt64 writes an int64 value in JSON format.
func (enc *JSONEncoder) AppendInt64(v int64) {
	enc.appendSeparator()
	enc.last = jsonTokenValue
	enc.buf.WriteString(strconv.FormatInt(v, 10))
}

// AppendFloat64 writes a float64 value in JSON format.
func (enc *JSONEncoder) AppendFloat64(v float64) {
	enc.appendSeparator()
	enc.last = jsonTokenValue
	enc.buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
}

// AppendString writes a string value in JSON format
// with proper quotation marks.
func (enc *JSONEncoder) AppendString(v string) {
	enc.appendSeparator()
	enc.last = jsonTokenValue
	enc.buf.WriteByte('"')
	enc.buf.WriteString(v)
	enc.buf.WriteByte('"')
}

// AppendReflect marshals an arbitrary Go value into JSON
// and appends it. If marshalling fails, the error message
// is written as a JSON string instead.
func (enc *JSONEncoder) AppendReflect(v interface{}) {
	enc.appendSeparator()
	enc.last = jsonTokenValue
	b, err := json.Marshal(v)
	if err != nil {
		enc.buf.WriteByte('"')
		enc.buf.WriteString(err.Error())
		enc.buf.WriteByte('"')
		return
	}
	enc.buf.Write(b)
}
