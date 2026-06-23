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

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/jsonflow/internal/json"
)

// Encoder adapts jsontext.Encoder to the json.Encoder interface.
type Encoder struct {
	enc *jsontext.Encoder
}

// NewEncoder creates an Encoder that writes to enc.
func NewEncoder(enc *jsontext.Encoder) *Encoder {
	return &Encoder{enc: enc}
}

// WriteToken writes the next JSON token to the encoder.
// For strings, token is the unescaped string content.
// For other kinds, token is the raw JSON token representation.
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
		return errutil.Explain(nil, "jsonv2: invalid JSON token kind %q", kind)
	}
}

// WriteValue writes a complete JSON value to the encoder.
func (e *Encoder) WriteValue(value []byte) error {
	return e.enc.WriteValue(value)
}
