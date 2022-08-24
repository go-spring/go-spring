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
)

type Encoder interface {
	AppendEncoderBegin() error
	AppendEncoderEnd() error
	AppendObjectBegin() error
	AppendObjectEnd() error
	AppendArrayBegin() error
	AppendArrayEnd() error
	AppendKey(key string) error
	AppendBool(bool) error
	AppendInt64(int64) error
	AppendUint64(uint64) error
	AppendFloat64(float64) error
	AppendString(string) error
	AppendReflect(v interface{}) error
	AppendBuffer([]byte) error
}

var (
	_ Encoder = (*jsonEncoder)(nil)
	_ Encoder = (*flatEncoder)(nil)
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

type jsonEncoder struct {
	buf  *bytes.Buffer
	last jsonToken
}

func NewJSONEncoder(buf *bytes.Buffer) *jsonEncoder {
	return &jsonEncoder{
		buf:  buf,
		last: jsonTokenUnknown,
	}
}

func (enc *jsonEncoder) Reset() {
	enc.last = jsonTokenUnknown
}

func (enc *jsonEncoder) AppendEncoderBegin() error {
	enc.last = jsonTokenObjectBegin
	return enc.buf.WriteByte('{')
}

func (enc *jsonEncoder) AppendEncoderEnd() error {
	enc.last = jsonTokenObjectEnd
	return enc.buf.WriteByte('}')
}

func (enc *jsonEncoder) AppendObjectBegin() error {
	enc.last = jsonTokenObjectBegin
	return enc.buf.WriteByte('{')
}

func (enc *jsonEncoder) AppendObjectEnd() error {
	enc.last = jsonTokenObjectEnd
	return enc.buf.WriteByte('}')
}

func (enc *jsonEncoder) AppendArrayBegin() error {
	enc.last = jsonTokenArrayBegin
	return enc.buf.WriteByte('[')
}

func (enc *jsonEncoder) AppendArrayEnd() error {
	enc.last = jsonTokenArrayEnd
	return enc.buf.WriteByte(']')
}

func (enc *jsonEncoder) appendSeparator(curr jsonToken) error {
	switch curr {
	case jsonTokenKey:
		if enc.last == jsonTokenObjectEnd || enc.last == jsonTokenArrayEnd || enc.last == jsonTokenValue {
			return enc.buf.WriteByte(',')
		}
	case jsonTokenValue:
		if enc.last == jsonTokenValue {
			return enc.buf.WriteByte(',')
		}
	}
	return nil
}

func (enc *jsonEncoder) AppendKey(key string) error {
	err := enc.appendSeparator(jsonTokenKey)
	if err != nil {
		return err
	}
	enc.last = jsonTokenKey
	err = enc.buf.WriteByte('"')
	if err != nil {
		return err
	}
	_, err = enc.buf.WriteString(key)
	if err != nil {
		return err
	}
	err = enc.buf.WriteByte('"')
	if err != nil {
		return err
	}
	return enc.buf.WriteByte(':')
}

func (enc *jsonEncoder) AppendBool(b bool) error {
	err := enc.appendSeparator(jsonTokenValue)
	if err != nil {
		return err
	}
	enc.last = jsonTokenValue
	_, err = enc.buf.WriteString(strconv.FormatBool(b))
	return err
}

func (enc *jsonEncoder) AppendInt64(i int64) error {
	err := enc.appendSeparator(jsonTokenValue)
	if err != nil {
		return err
	}
	enc.last = jsonTokenValue
	_, err = enc.buf.WriteString(strconv.FormatInt(i, 10))
	return err
}

func (enc *jsonEncoder) AppendUint64(u uint64) error {
	err := enc.appendSeparator(jsonTokenValue)
	if err != nil {
		return err
	}
	enc.last = jsonTokenValue
	_, err = enc.buf.WriteString(strconv.FormatUint(u, 10))
	return err
}

func (enc *jsonEncoder) AppendFloat64(f float64) error {
	err := enc.appendSeparator(jsonTokenValue)
	if err != nil {
		return err
	}
	enc.last = jsonTokenValue
	_, err = enc.buf.WriteString(strconv.FormatFloat(f, 'f', -1, 64))
	return err
}

func (enc *jsonEncoder) AppendString(s string) error {
	err := enc.appendSeparator(jsonTokenValue)
	if err != nil {
		return err
	}
	enc.last = jsonTokenValue
	err = enc.buf.WriteByte('"')
	if err != nil {
		return err
	}
	_, err = enc.buf.WriteString(s)
	if err != nil {
		return err
	}
	return enc.buf.WriteByte('"')
}

func (enc *jsonEncoder) AppendReflect(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = enc.appendSeparator(jsonTokenValue)
	if err != nil {
		return err
	}
	enc.last = jsonTokenValue
	_, err = enc.buf.Write(b)
	return err
}

func (enc *jsonEncoder) AppendBuffer(b []byte) error {
	err := enc.appendSeparator(jsonTokenKey)
	if err != nil {
		return err
	}
	enc.last = jsonTokenValue
	_, err = enc.buf.Write(b)
	return err
}

type flatEncoder struct {
	buf         *bytes.Buffer
	separator   string
	jsonEncoder *jsonEncoder
	jsonDepth   int
	init        bool
}

func NewFlatEncoder(buf *bytes.Buffer, separator string) *flatEncoder {
	return &flatEncoder{
		buf:         buf,
		separator:   separator,
		jsonEncoder: &jsonEncoder{buf: buf},
	}
}

func (enc *flatEncoder) AppendEncoderBegin() error {
	return nil
}

func (enc *flatEncoder) AppendEncoderEnd() error {
	return nil
}

func (enc *flatEncoder) AppendObjectBegin() error {
	enc.jsonDepth++
	return enc.jsonEncoder.AppendObjectBegin()
}

func (enc *flatEncoder) AppendObjectEnd() error {
	enc.jsonDepth--
	err := enc.jsonEncoder.AppendObjectEnd()
	if enc.jsonDepth == 0 {
		enc.jsonEncoder.Reset()
	}
	return err
}

func (enc *flatEncoder) AppendArrayBegin() error {
	enc.jsonDepth++
	return enc.jsonEncoder.AppendArrayBegin()
}

func (enc *flatEncoder) AppendArrayEnd() error {
	enc.jsonDepth--
	err := enc.jsonEncoder.AppendArrayEnd()
	if enc.jsonDepth == 0 {
		enc.jsonEncoder.Reset()
	}
	return err
}

func (enc *flatEncoder) AppendKey(key string) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendKey(key)
	}
	if enc.init {
		_, err := enc.buf.WriteString(enc.separator)
		if err != nil {
			return err
		}
	}
	enc.init = true
	_, err := enc.buf.WriteString(key)
	if err != nil {
		return err
	}
	return enc.buf.WriteByte('=')
}

func (enc *flatEncoder) AppendBool(b bool) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendBool(b)
	}
	_, err := enc.buf.WriteString(strconv.FormatBool(b))
	return err
}

func (enc *flatEncoder) AppendInt64(i int64) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendInt64(i)
	}
	_, err := enc.buf.WriteString(strconv.FormatInt(i, 10))
	return err
}

func (enc *flatEncoder) AppendUint64(u uint64) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendUint64(u)
	}
	_, err := enc.buf.WriteString(strconv.FormatUint(u, 10))
	return err
}

func (enc *flatEncoder) AppendFloat64(f float64) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendFloat64(f)
	}
	_, err := enc.buf.WriteString(strconv.FormatFloat(f, 'f', -1, 64))
	return err
}

func (enc *flatEncoder) AppendString(s string) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendString(s)
	}
	_, err := enc.buf.WriteString(s)
	return err
}

func (enc *flatEncoder) AppendReflect(v interface{}) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendReflect(v)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = enc.buf.Write(b)
	return err
}

func (enc *flatEncoder) AppendBuffer(b []byte) error {
	if enc.jsonDepth > 0 {
		return enc.jsonEncoder.AppendBuffer(b)
	}
	_, err := enc.buf.Write(b)
	return err
}
