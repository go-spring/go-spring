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

// Decoder adapts jsontext.Decoder to the json.Decoder interface.
type Decoder struct {
	*jsontext.Decoder
}

// toKind converts jsontext.Kind to the json.Kind.
// Returns json.InvalidKind if the input kind does not match any known JSON token.
func toKind(k jsontext.Kind) json.Kind {
	switch k {
	case 'n':
		return 'n'
	case 'f':
		return 'f'
	case 't':
		return 't'
	case '"':
		return '"'
	case '0':
		return '0'
	case '{':
		return '{'
	case '}':
		return '}'
	case '[':
		return '['
	case ']':
		return ']'
	default:
		return json.InvalidKind
	}
}

// PeekKind returns the Kind of the next JSON token without consuming it.
func (d *Decoder) PeekKind() json.Kind {
	return toKind(d.Decoder.PeekKind())
}

// ReadToken reads the next JSON token and returns its string value and kind.
// For strings, token is the unescaped string content.
// For other kinds, token is the raw JSON token representation.
func (d *Decoder) ReadToken() ( /* token */ string /* kind */, json.Kind /* err */, error) {
	token, err := d.Decoder.ReadToken()
	if err != nil {
		return "", 0, err
	}
	return token.String(), toKind(token.Kind()), nil
}

// ReadValue reads the next complete JSON value as bytes.
func (d *Decoder) ReadValue() ( /* value */ []byte /* err */, error) {
	return d.Decoder.ReadValue()
}

// SkipValue skips the next complete JSON value.
func (d *Decoder) SkipValue() error {
	return d.Decoder.SkipValue()
}
