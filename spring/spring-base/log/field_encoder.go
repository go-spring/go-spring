/*
 * Copyright 2012-2019 the original author or authors.
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
	"bytes"
	"encoding/json"
	"strconv"
	"unicode/utf8"
)

// An Encoder is used to serialize strongly-typed Field.
type Encoder interface {
	AppendEncoderBegin() error
	AppendEncoderEnd() error
	AppendObjectBegin() error
	AppendObjectEnd() error
	AppendArrayBegin() error
	AppendArrayEnd() error
	AppendKey(key string) error
	AppendBool(v bool) error
	AppendInt64(v int64) error
	AppendUint64(v uint64) error
	AppendFloat64(v float64) error
	AppendString(v string) error
	AppendReflect(v interface{}) error
}

var (
	_ Encoder = (*JSONEncoder)(nil)
	_ Encoder = (*FlatEncoder)(nil)
)

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

// JSONEncoder encodes Fields in json format.
type JSONEncoder struct {
	buf  *bytes.Buffer
	last jsonToken
}

// NewJSONEncoder returns a new *JSONEncoder.
func NewJSONEncoder(buf *bytes.Buffer) *JSONEncoder {
	return &JSONEncoder{
		buf:  buf,
		last: jsonTokenUnknown,
	}
}

// Reset resets the *JSONEncoder.
func (enc *JSONEncoder) Reset() {
	enc.last = jsonTokenUnknown
}

// AppendEncoderBegin appends an encoder begin character.
func (enc *JSONEncoder) AppendEncoderBegin() error {
	enc.last = jsonTokenObjectBegin
	enc.buf.WriteByte('{')
	return nil
}

// AppendEncoderEnd appends an encoder end character.
func (enc *JSONEncoder) AppendEncoderEnd() error {
	enc.last = jsonTokenObjectEnd
	enc.buf.WriteByte('}')
	return nil
}

// AppendObjectBegin appends a object begin character.
func (enc *JSONEncoder) AppendObjectBegin() error {
	enc.last = jsonTokenObjectBegin
	enc.buf.WriteByte('{')
	return nil
}

// AppendObjectEnd appends an object end character.
func (enc *JSONEncoder) AppendObjectEnd() error {
	enc.last = jsonTokenObjectEnd
	enc.buf.WriteByte('}')
	return nil
}

// AppendArrayBegin appends an array begin character.
func (enc *JSONEncoder) AppendArrayBegin() error {
	enc.last = jsonTokenArrayBegin
	enc.buf.WriteByte('[')
	return nil
}

// AppendArrayEnd appends an array end character.
func (enc *JSONEncoder) AppendArrayEnd() error {
	enc.last = jsonTokenArrayEnd
	enc.buf.WriteByte(']')
	return nil
}

func (enc *JSONEncoder) appendSeparator(curr jsonToken) {
	switch curr {
	case jsonTokenKey:
		if enc.last == jsonTokenObjectEnd || enc.last == jsonTokenArrayEnd || enc.last == jsonTokenValue {
			enc.buf.WriteByte(',')
		}
	case jsonTokenValue:
		if enc.last == jsonTokenValue {
			enc.buf.WriteByte(',')
		}
	}
}

// AppendKey appends a key.
func (enc *JSONEncoder) AppendKey(key string) error {
	enc.appendSeparator(jsonTokenKey)
	enc.last = jsonTokenKey
	enc.buf.WriteByte('"')
	enc.safeAddString(key)
	enc.buf.WriteByte('"')
	enc.buf.WriteByte(':')
	return nil
}

// AppendBool appends a bool.
func (enc *JSONEncoder) AppendBool(v bool) error {
	enc.appendSeparator(jsonTokenValue)
	enc.last = jsonTokenValue
	enc.buf.WriteString(strconv.FormatBool(v))
	return nil
}

// AppendInt64 appends an int64.
func (enc *JSONEncoder) AppendInt64(v int64) error {
	enc.appendSeparator(jsonTokenValue)
	enc.last = jsonTokenValue
	enc.buf.WriteString(strconv.FormatInt(v, 10))
	return nil
}

// AppendUint64 appends a uint64.
func (enc *JSONEncoder) AppendUint64(u uint64) error {
	enc.appendSeparator(jsonTokenValue)
	enc.last = jsonTokenValue
	enc.buf.WriteString(strconv.FormatUint(u, 10))
	return nil
}

// AppendFloat64 appends a float64.
func (enc *JSONEncoder) AppendFloat64(v float64) error {
	enc.appendSeparator(jsonTokenValue)
	enc.last = jsonTokenValue
	enc.buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	return nil
}

// AppendString appends a string.
func (enc *JSONEncoder) AppendString(v string) error {
	enc.appendSeparator(jsonTokenValue)
	enc.last = jsonTokenValue
	enc.buf.WriteByte('"')
	enc.safeAddString(v)
	enc.buf.WriteByte('"')
	return nil
}

// AppendReflect appends an interface{}.
func (enc *JSONEncoder) AppendReflect(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	enc.appendSeparator(jsonTokenValue)
	enc.last = jsonTokenValue
	enc.buf.Write(b)
	return nil
}

// safeAddString JSON-escapes a string and appends it to the buf.
func (enc *JSONEncoder) safeAddString(s string) {
	for i := 0; i < len(s); {
		if enc.tryAddRuneSelf(s[i]) {
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if enc.tryAddRuneError(r, size) {
			i++
			continue
		}
		enc.buf.WriteString(s[i : i+size])
		i += size
	}
}

// tryAddRuneSelf appends b if it's valid UTF-8 character represented in a single byte.
func (enc *JSONEncoder) tryAddRuneSelf(b byte) bool {
	const _hex = "0123456789abcdef"
	if b >= utf8.RuneSelf {
		return false
	}
	if 0x20 <= b && b != '\\' && b != '"' {
		enc.buf.WriteByte(b)
		return true
	}
	switch b {
	case '\\', '"':
		enc.buf.WriteByte('\\')
		enc.buf.WriteByte(b)
	case '\n':
		enc.buf.WriteByte('\\')
		enc.buf.WriteByte('n')
	case '\r':
		enc.buf.WriteByte('\\')
		enc.buf.WriteByte('r')
	case '\t':
		enc.buf.WriteByte('\\')
		enc.buf.WriteByte('t')
	default:
		// Encode bytes < 0x20, except for the escape sequences above.
		enc.buf.WriteString(`\u00`)
		enc.buf.WriteByte(_hex[b>>4])
		enc.buf.WriteByte(_hex[b&0xF])
	}
	return true
}

func (enc *JSONEncoder) tryAddRuneError(r rune, size int) bool {
	if r == utf8.RuneError && size == 1 {
		enc.buf.WriteString(`\ufffd`)
		return true
	}
	return false
}

// FlatEncoder encodes Fields in flat format.
type FlatEncoder struct {
	buf         *bytes.Buffer
	separator   string
	jsonEncoder *JSONEncoder
	jsonDepth   int8
	init        bool
}

// NewFlatEncoder return a new *FlatEncoder with separator.
func NewFlatEncoder(buf *bytes.Buffer, separator string) *FlatEncoder {
	return &FlatEncoder{
		buf:         buf,
		separator:   separator,
		jsonEncoder: &JSONEncoder{buf: buf},
	}
}

// AppendEncoderBegin appends an encoder begin character.
func (enc *FlatEncoder) AppendEncoderBegin() error {
	return nil
}

// AppendEncoderEnd appends an encoder end character.
func (enc *FlatEncoder) AppendEncoderEnd() error {
	return nil
}

// AppendObjectBegin appends a object begin character.
func (enc *FlatEncoder) AppendObjectBegin() error {
	enc.jsonDepth++
	return enc.jsonEncoder.AppendObjectBegin()
}

// AppendObjectEnd appends an object end character.
func (enc *FlatEncoder) AppendObjectEnd() error {
	enc.jsonDepth--
	err := enc.jsonEncoder.AppendObjectEnd()
	if enc.jsonDepth == 0 {
		enc.jsonEncoder.Reset()
	}
	return err
}

// AppendArrayBegin appends an array begin character.
func (enc *FlatEncoder) AppendArrayBegin() error {
	enc.jsonDepth++
	return enc.jsonEncoder.AppendArrayBegin()
}

// AppendArrayEnd appends an array end character.
func (enc *FlatEncoder) AppendArrayEnd() error {
	enc.jsonDepth--
	err := enc.jsonEncoder.AppendArrayEnd()
	if enc.jsonDepth == 0 {
		enc.jsonEncoder.Reset()
	}
	return err
}

// AppendKey appends a key.
func (enc *FlatEncoder) AppendKey(key string) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendKey(key)
	}
	if enc.init {
		enc.buf.WriteString(enc.separator)
	}
	enc.init = true
	enc.buf.WriteString(key)
	enc.buf.WriteByte('=')
	return nil
}

// AppendBool appends a bool.
func (enc *FlatEncoder) AppendBool(v bool) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendBool(v)
	}
	enc.buf.WriteString(strconv.FormatBool(v))
	return nil
}

// AppendInt64 appends a int64.
func (enc *FlatEncoder) AppendInt64(v int64) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendInt64(v)
	}
	enc.buf.WriteString(strconv.FormatInt(v, 10))
	return nil
}

// AppendUint64 appends a uint64.
func (enc *FlatEncoder) AppendUint64(v uint64) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendUint64(v)
	}
	enc.buf.WriteString(strconv.FormatUint(v, 10))
	return nil
}

// AppendFloat64 appends a float64.
func (enc *FlatEncoder) AppendFloat64(v float64) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendFloat64(v)
	}
	enc.buf.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
	return nil
}

// AppendString appends a string.
func (enc *FlatEncoder) AppendString(v string) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendString(v)
	}
	enc.buf.WriteString(v)
	return nil
}

// AppendReflect appends an interface{}.
func (enc *FlatEncoder) AppendReflect(v interface{}) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendReflect(v)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	enc.buf.Write(b)
	return nil
}
