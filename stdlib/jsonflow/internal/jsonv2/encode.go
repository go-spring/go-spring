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

package jsonv2

import (
	"encoding/json/jsontext"
)

// Encoder wraps jsontext.Encoder to implement the json.Encoder interface.
type Encoder struct {
	*jsontext.Encoder
}

// WriteValue writes a JSON value to the encoder.
func (e *Encoder) WriteValue(v []byte) error {
	return e.Encoder.WriteValue(v)
}

// WriteNull writes a JSON null token to the encoder.
func (e *Encoder) WriteNull() error {
	return e.Encoder.WriteToken(jsontext.Null)
}

// WriteBool writes a JSON boolean token to the encoder.
func (e *Encoder) WriteBool(v bool) error {
	return e.Encoder.WriteToken(jsontext.Bool(v))
}

// WriteInt writes a JSON integer token to the encoder.
func (e *Encoder) WriteInt(v int64) error {
	return e.Encoder.WriteToken(jsontext.Int(v))
}

// WriteUint writes a JSON unsigned integer token to the encoder.
func (e *Encoder) WriteUint(v uint64) error {
	return e.Encoder.WriteToken(jsontext.Uint(v))
}

// WriteFloat writes a JSON floating-point token to the encoder.
func (e *Encoder) WriteFloat(v float64) error {
	return e.Encoder.WriteToken(jsontext.Float(v))
}

// WriteString writes a JSON string token to the encoder.
func (e *Encoder) WriteString(v string) error {
	return e.Encoder.WriteToken(jsontext.String(v))
}

// WriteObjectBegin writes a JSON object begin token to the encoder.
func (e *Encoder) WriteObjectBegin() error {
	return e.Encoder.WriteToken(jsontext.BeginObject)
}

// WriteObjectEnd writes a JSON object end token to the encoder.
func (e *Encoder) WriteObjectEnd() error {
	return e.Encoder.WriteToken(jsontext.EndObject)
}

// WriteArrayBegin writes a JSON array begin token to the encoder.
func (e *Encoder) WriteArrayBegin() error {
	return e.Encoder.WriteToken(jsontext.BeginArray)
}

// WriteArrayEnd writes a JSON array end token to the encoder.
func (e *Encoder) WriteArrayEnd() error {
	return e.Encoder.WriteToken(jsontext.EndArray)
}
