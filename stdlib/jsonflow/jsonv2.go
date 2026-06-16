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

package jsonflow

import (
	"bytes"
	"encoding/json/jsontext"
	stdjsonv2 "encoding/json/v2"
	"io"

	"go-spring.org/stdlib/jsonflow/internal/json"
	"go-spring.org/stdlib/jsonflow/internal/jsonv2"
)

// NotForPublicUse is used to seal MarshalOptions.
type NotForPublicUse struct{}

// MarshalOptions is an interface that defines options for encoding JSON.
type MarshalOptions interface {
	JSONOptions(NotForPublicUse)
}

type (
	Indent         string
	IndentPrefix   string
	NilSliceAsNull bool
	NilMapAsNull   bool
	Deterministic  bool
)

func (Indent) JSONOptions(NotForPublicUse)         {}
func (IndentPrefix) JSONOptions(NotForPublicUse)   {}
func (NilSliceAsNull) JSONOptions(NotForPublicUse) {}
func (NilMapAsNull) JSONOptions(NotForPublicUse)   {}
func (Deterministic) JSONOptions(NotForPublicUse)  {}

// toJSONv2Options converts MarshalOptions to jsontext.Options.
func toJSONv2Options(opts []MarshalOptions) []jsontext.Options {
	options := []jsontext.Options{
		stdjsonv2.FormatNilSliceAsNull(true),
		stdjsonv2.FormatNilMapAsNull(true),
		stdjsonv2.Deterministic(true),
	}
	for _, opt := range opts {
		switch x := opt.(type) {
		case Indent:
			options = append(options, jsontext.WithIndent(string(x)))
		case IndentPrefix:
			options = append(options, jsontext.WithIndentPrefix(string(x)))
		case NilSliceAsNull:
			options = append(options, stdjsonv2.FormatNilSliceAsNull(bool(x)))
		case NilMapAsNull:
			options = append(options, stdjsonv2.FormatNilMapAsNull(bool(x)))
		case Deterministic:
			options = append(options, stdjsonv2.Deterministic(bool(x)))
		default: // for linter
		}
	}
	return options
}

// NewEncoder creates a new jsonv2.Encoder that implements the json.Encoder interface.
func NewEncoder(w io.Writer) json.Encoder {
	return jsonv2.NewEncoder(jsontext.NewEncoder(w))
}

// NewDecoder creates a new jsonv2.Decoder that implements the json.Decoder interface.
func NewDecoder(r io.Reader) json.Decoder {
	return &jsonv2.Decoder{Decoder: jsontext.NewDecoder(r)}
}

// Marshal marshals a Go value into JSON bytes.
func Marshal(i any, opts ...MarshalOptions) ([]byte, error) {
	if v, ok := i.(Object); ok {
		buf := bytes.NewBuffer(nil)
		if err := v.EncodeJSON(NewEncoder(buf)); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	return stdjsonv2.Marshal(i, toJSONv2Options(opts)...)
}

// MarshalIndent marshals a Go value into JSON bytes with indentation.
func MarshalIndent(i any, prefix, indent string) ([]byte, error) {
	return Marshal(i, IndentPrefix(prefix), Indent(indent))
}

// MarshalWrite marshals a Go value into JSON bytes and writes them to a writer.
func MarshalWrite(w io.Writer, i any, opts ...MarshalOptions) error {
	if v, ok := i.(Object); ok {
		return v.EncodeJSON(NewEncoder(w))
	}
	return stdjsonv2.MarshalWrite(w, i, toJSONv2Options(opts)...)
}

// Unmarshal unmarshals JSON bytes into a Go value.
func Unmarshal(b []byte, i any) error {
	return UnmarshalRead(bytes.NewReader(b), i)
}

// UnmarshalRead unmarshals JSON bytes from a reader into a Go value.
func UnmarshalRead(r io.Reader, i any) error {
	if v, ok := i.(Object); ok {
		d := NewDecoder(r)
		if err := v.DecodeJSON(d); err != nil {
			return err
		}
		return DecodeEOF(d)
	}
	return stdjsonv2.UnmarshalRead(r, i)
}
