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
	"encoding/base64"
	"io"
	"strconv"

	"github.com/go-spring/stdlib/errutil"
	"github.com/go-spring/stdlib/jsonflow/internal/json"
	"github.com/go-spring/stdlib/mathutil"
)

const (
	errFormatBoolean = "invalid JSON: expected boolean but got `%s`"
	errFormatNumber  = "invalid JSON: expected number but got `%s`"
	errFormatString  = "invalid JSON: expected string but got `%s`"
)

// Decoder defines a streaming JSON decoder interface.
type Decoder = json.Decoder

// ParseBool parses a JSON boolean token into a Go bool.
// The input Kind must be 't' or 'f', otherwise an error is returned.
func ParseBool(token string, k json.Kind) (bool, error) {
	if k != 'f' && k != 't' {
		return false, errutil.Explain(nil, errFormatBoolean, token)
	}
	return k == 't', nil
}

// DecodeBool reads the next JSON value from the decoder and parses it as bool.
func DecodeBool(d Decoder) (bool, error) {
	return DecodeValue(ParseBool, errFormatBoolean)(d)
}

// DecodeBoolPtr reads the next JSON value and parses it as *bool.
// Returns nil if the JSON token is null.
func DecodeBoolPtr(d Decoder) (*bool, error) {
	return DecodeValuePtr(ParseBool, errFormatBoolean)(d)
}

// ParseInt parses a JSON number token into an integer type T.
// Returns an error if the token is not a number or if the value overflows.
func ParseInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](token string, k json.Kind) (T, error) {
	if k != '0' {
		return 0, errutil.Explain(nil, errFormatNumber, token)
	}
	v, err := strconv.ParseInt(token, 10, 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowInt[T](v) {
		return 0, errutil.Explain(nil, "invalid JSON: number out of range, got `%s", token)
	}
	return T(v), nil
}

// DecodeInt reads the next JSON value and parses it into an integer type T.
func DecodeInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](d Decoder) (T, error) {
	return DecodeValue(ParseInt[T], errFormatNumber)(d)
}

// DecodeIntPtr reads the next JSON value and parses it into a pointer to integer type T.
// Returns nil if the JSON token is null.
func DecodeIntPtr[T ~int | ~int8 | ~int16 | ~int32 | ~int64](d Decoder) (*T, error) {
	return DecodeValuePtr(ParseInt[T], errFormatNumber)(d)
}

// ParseIntKey parses a JSON object key as an integer type T.
// Returns an error if parsing fails or the value overflows.
func ParseIntKey[T ~int | ~int8 | ~int16 | ~int32 | ~int64](token string, _ json.Kind) (T, error) {
	v, err := strconv.ParseInt(token, 10, 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowInt[T](v) {
		return 0, errutil.Explain(nil, "invalid JSON: number out of range, got `%s", token)
	}
	return T(v), nil
}

// DecodeIntKey reads a JSON object key and parses it as an integer type T.
func DecodeIntKey[T ~int | ~int8 | ~int16 | ~int32 | ~int64](d Decoder) (T, error) {
	return DecodeValue(ParseIntKey[T], errFormatNumber)(d)
}

// ParseUint parses a JSON number token into an unsigned integer type T.
func ParseUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](token string, k json.Kind) (T, error) {
	if k != '0' {
		return 0, errutil.Explain(nil, errFormatNumber, token)
	}
	v, err := strconv.ParseUint(token, 10, 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowUint[T](v) {
		return 0, errutil.Explain(nil, "invalid JSON: number out of range, got `%s`", token)
	}
	return T(v), nil
}

// DecodeUint reads the next JSON value and parses it as an unsigned integer type T.
func DecodeUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](d Decoder) (T, error) {
	return DecodeValue(ParseUint[T], errFormatNumber)(d)
}

// DecodeUintPtr reads the next JSON value and parses it into a pointer to unsigned type T.
func DecodeUintPtr[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](d Decoder) (*T, error) {
	return DecodeValuePtr(ParseUint[T], errFormatNumber)(d)
}

// ParseUintKey parses a JSON object key as an unsigned integer type T.
func ParseUintKey[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](token string, _ json.Kind) (T, error) {
	v, err := strconv.ParseUint(token, 10, 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowUint[T](v) {
		return 0, errutil.Explain(nil, "invalid JSON: number out of range, got `%s", token)
	}
	return T(v), nil
}

// DecodeUintKey reads a JSON object key and parses it as an unsigned integer type T.
func DecodeUintKey[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](d Decoder) (T, error) {
	return DecodeValue(ParseUintKey[T], errFormatNumber)(d)
}

// ParseFloat parses a JSON number token into a float type T.
func ParseFloat[T ~float32 | ~float64](token string, k json.Kind) (T, error) {
	if k != '0' {
		return 0, errutil.Explain(nil, errFormatNumber, token)
	}
	f, err := strconv.ParseFloat(token, 64)
	if err != nil {
		return 0, err
	}
	if mathutil.OverflowFloat[T](f) {
		return 0, errutil.Explain(nil, "invalid JSON: number out of range, got `%s", token)
	}
	return T(f), nil
}

// DecodeFloat reads the next JSON value and parses it as a float type T.
func DecodeFloat[T ~float32 | ~float64](d Decoder) (T, error) {
	return DecodeValue(ParseFloat[T], errFormatNumber)(d)
}

// DecodeFloatPtr reads the next JSON value and parses it into a pointer to float type T.
func DecodeFloatPtr[T ~float32 | ~float64](d Decoder) (*T, error) {
	return DecodeValuePtr(ParseFloat[T], errFormatNumber)(d)
}

// ParseString parses a JSON string token into a Go string.
func ParseString(token string, k json.Kind) (string, error) {
	if k != '"' {
		return "", errutil.Explain(nil, errFormatString, token)
	}
	return token, nil
}

// DecodeString reads the next JSON value and parses it as a string.
func DecodeString(d Decoder) (string, error) {
	return DecodeValue(ParseString, errFormatString)(d)
}

// DecodeStringPtr reads the next JSON value and parses it as a pointer to string.
func DecodeStringPtr(d Decoder) (*string, error) {
	return DecodeValuePtr(ParseString, errFormatString)(d)
}

// ParseBytes parses a JSON string token as base64-encoded bytes.
func ParseBytes(token string, k json.Kind) ([]byte, error) {
	if k != '"' {
		return nil, errutil.Explain(nil, errFormatString, token)
	}
	return base64.StdEncoding.DecodeString(token)
}

// DecodeBytes reads the next JSON value and parses it as base64-decoded bytes.
func DecodeBytes(d Decoder) ([]byte, error) {
	if d.PeekKind() == 'n' {
		_, _, err := d.ReadToken()
		return nil, err
	}
	return DecodeValue(ParseBytes, errFormatString)(d)
}

// DecodeEOF verifies that the decoder has no remaining top-level JSON tokens.
func DecodeEOF(d Decoder) error {
	token, _, err := d.ReadToken()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return errutil.Explain(nil, "invalid JSON: unexpected token after top-level value `%s`", token)
}

// Object represents a JSON-mappable object that supports streaming decoding.
type Object interface {
	// DecodeJSON reads JSON data from the Decoder and populates the object.
	DecodeJSON(d Decoder) error
}

// DecodeObjectBegin consumes the opening '{' token of a JSON object.
// Returns an error if the next token is not '{'.
func DecodeObjectBegin(d Decoder) error {
	token, tokenKind, err := d.ReadToken()
	if err != nil {
		return err
	}
	if tokenKind != '{' {
		return errutil.Explain(nil, "invalid JSON: expected `{` but got %s", token)
	}
	return nil
}

// DecodeObjectEnd consumes the closing '}' token of a JSON object.
// Returns an error if the next token is not '}'.
func DecodeObjectEnd(d Decoder) error {
	token, tokenKind, err := d.ReadToken()
	if err != nil {
		return err
	}
	if tokenKind != '}' {
		return errutil.Explain(nil, "invalid JSON: expected `}` but got %s", token)
	}
	return nil
}

// DecodeAny decodes the next JSON value (scalar, object, or array)
// into a Go value using Decoder.Unmarshal.
func DecodeAny[T any](d Decoder) (T, error) {
	var v T
	b, err := d.ReadValue()
	if err != nil {
		return v, err
	}
	if err = Unmarshal(b, &v); err != nil {
		return v, err
	}
	return v, nil
}

// DecodeValue parses a scalar JSON value (number, boolean, or string) using parseFn.
// Returns an error if the next token is null or invalid.
func DecodeValue[T any](
	parseFn func(string, json.Kind) (T, error),
	errFormat string,
) func(d Decoder) (T, error) {
	return func(d Decoder) (T, error) {
		var v T
		token, tokenKind, err := d.ReadToken()
		if err != nil {
			return v, err
		}
		switch tokenKind {
		case 'n':
			return v, errutil.Explain(nil, errFormat, token)
		case 'f', 't', '0', '"':
			return parseFn(token, tokenKind)
		default:
			return v, errutil.Explain(nil, errFormat, token)
		}
	}
}

// DecodeValuePtr parses a scalar JSON value into a pointer type.
// Returns nil if the next token is null.
func DecodeValuePtr[T any](
	parseFn func(string, json.Kind) (T, error),
	errFormat string,
) func(d Decoder) (*T, error) {
	return func(d Decoder) (*T, error) {
		token, tokenKind, err := d.ReadToken()
		if err != nil {
			return nil, err
		}
		switch tokenKind {
		case 'n':
			return nil, nil
		case 'f', 't', '0', '"':
			var v T
			v, err = parseFn(token, tokenKind)
			if err != nil {
				return nil, err
			}
			return &v, nil
		default:
			return nil, errutil.Explain(err, errFormat, token)
		}
	}
}

// DecodeObject decodes a JSON object into a struct that implements the Object interface.
// Returns the zero value if the next token is null.
// Internally calls DecodeJSON on the object to populate its fields.
func DecodeObject[T Object](
	newFn func() T,
) func(d Decoder) (T, error) {
	return func(d Decoder) (T, error) {
		var v T
		switch d.PeekKind() {
		case 'n':
			_, _, _ = d.ReadToken()
			return v, nil
		case '{':
			v = newFn()
			if err := v.DecodeJSON(d); err != nil {
				return v, err
			}
			return v, nil
		default:
			token, _, err := d.ReadToken()
			if err != nil {
				return v, err
			}
			return v, errutil.Explain(nil, "invalid JSON: expected `{` but got `%s`", token)
		}
	}
}

// DecodeArray decodes a JSON array of arbitrary type.
// parseFn is used to parse each element of the array.
// Returns nil if the next token is null.
func DecodeArray[T any](
	parseFn func(d Decoder) (T, error),
) func(d Decoder) ([]T, error) {
	return func(d Decoder) ([]T, error) {
		switch d.PeekKind() {
		case 'n':
			_, _, _ = d.ReadToken()
			return nil, nil
		case '[':
			_, _, _ = d.ReadToken()
			v := make([]T, 0)
			for d.PeekKind() != ']' {
				i, err := parseFn(d)
				if err != nil {
					return nil, err
				}
				v = append(v, i)
			}
			_, _, _ = d.ReadToken()
			return v, nil
		default:
			token, _, err := d.ReadToken()
			if err != nil {
				return nil, err
			}
			return nil, errutil.Explain(nil, "invalid JSON: expected `[` but got `%s`", token)
		}
	}
}

// DecodeMap decodes a JSON object into a Go map.
// parseKeyFn and parseValFn are used to parse each key and value.
// Returns nil if the next token is null.
func DecodeMap[K comparable, V any](
	parseKeyFn func(d Decoder) (K, error),
	parseValFn func(d Decoder) (V, error),
) func(d Decoder) (map[K]V, error) {
	return func(d Decoder) (map[K]V, error) {
		switch d.PeekKind() {
		case 'n':
			_, _, _ = d.ReadToken()
			return nil, nil
		case '{':
			_, _, _ = d.ReadToken()
			m := make(map[K]V)
			for d.PeekKind() != '}' {
				key, err := parseKeyFn(d)
				if err != nil {
					return nil, err
				}
				val, err := parseValFn(d)
				if err != nil {
					return nil, err
				}
				m[key] = val
			}
			_, _, _ = d.ReadToken()
			return m, nil
		default:
			token, _, err := d.ReadToken()
			if err != nil {
				return nil, err
			}
			return nil, errutil.Explain(nil, "invalid JSON: expected `{` but got `%s`", token)
		}
	}
}
