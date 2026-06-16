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

	"go-spring.org/stdlib/jsonflow/internal/json"
)

// Encoder wraps jsontext.Encoder to implement the json.Encoder interface.
type Encoder struct {
	enc *jsontext.Encoder
}

// NewEncoder creates an Encoder that writes to enc.
func NewEncoder(enc *jsontext.Encoder) *Encoder {
	return &Encoder{enc: enc}
}

// WriteToken writes the next JSON token to the encoder.
func (e *Encoder) WriteToken(token string, kind json.Kind) error {
	switch kind {
	case 'n':
		return e.enc.WriteToken(jsontext.Null)
	case 'f':
		return e.enc.WriteToken(jsontext.False)
	case 't':
		return e.enc.WriteToken(jsontext.True)
	case '"':
		return e.enc.WriteToken(jsontext.String(token))
	case '0':
		return e.enc.WriteValue([]byte(token))
	case '{':
		return e.enc.WriteToken(jsontext.BeginObject)
	case '}':
		return e.enc.WriteToken(jsontext.EndObject)
	case '[':
		return e.enc.WriteToken(jsontext.BeginArray)
	case ']':
		return e.enc.WriteToken(jsontext.EndArray)
	default:
		return e.enc.WriteValue([]byte(token))
	}
}

// WriteValue writes a JSON value to the encoder.
func (e *Encoder) WriteValue(v []byte) error {
	return e.enc.WriteValue(v)
}
