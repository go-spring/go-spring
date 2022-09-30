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

package json

import (
	"encoding/json"
	"io"
)

type WrapError struct {
	err error
}

func (w *WrapError) Error() string {
	return w.err.Error()
}

// Encoder encodes into byte sequence.
type Encoder interface {
	Encode(v interface{}) error
}

type WrapEncoder struct {
	e Encoder
}

// Encode writes the JSON encoding of v to the stream,
// followed by a newline character.
func (w *WrapEncoder) Encode(v interface{}) error {
	err := w.e.Encode(v)
	if err != nil {
		return &WrapError{err: err}
	}
	return nil
}

// Decoder decodes a byte sequence.
type Decoder interface {
	Decode(v interface{}) error
}

type WrapDecoder struct {
	d Decoder
}

// Decode reads the next JSON-encoded value from its
// input and stores it in the value pointed to by v.
func (w *WrapDecoder) Decode(v interface{}) error {
	err := w.d.Decode(v)
	if err != nil {
		return &WrapError{err: err}
	}
	return nil
}

var (
	MarshalFunc       = json.Marshal
	MarshalIndentFunc = json.MarshalIndent
	UnmarshalFunc     = json.Unmarshal
	NewEncoderFunc    = json.NewEncoder
	NewDecoderFunc    = json.NewDecoder
)

// Marshal returns the JSON encoding of v.
func Marshal(v interface{}) ([]byte, error) {
	data, err := MarshalFunc(v)
	if err != nil {
		return nil, &WrapError{err: err}
	}
	return data, nil
}

// MarshalIndent is like Marshal but applies Indent to format the output.
func MarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	data, err := MarshalIndentFunc(v, prefix, indent)
	if err != nil {
		return nil, &WrapError{err: err}
	}
	return data, nil
}

// Unmarshal parses the JSON-encoded data and stores the result
// in the value pointed to by v.
func Unmarshal(data []byte, v interface{}) error {
	err := UnmarshalFunc(data, v)
	if err != nil {
		return &WrapError{err: err}
	}
	return nil
}

// NewEncoder returns a new encoder that writes to w.
func NewEncoder(w io.Writer) Encoder {
	return &WrapEncoder{NewEncoderFunc(w)}
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) Decoder {
	return &WrapDecoder{NewDecoderFunc(r)}
}
