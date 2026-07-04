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
	"cmp"
	"encoding/base64"
	"maps"
	"math"
	"slices"
	"strconv"

	"go-spring.org/stdlib/jsonflow/internal/json"
)

// Encoder is a streaming JSON encoder.
type Encoder = json.Encoder

// JSONEncoder represents a value that can write itself as one JSON value.
type JSONEncoder interface {
	// EncodeJSON writes this object as one JSON value to the Encoder.
	EncodeJSON(e Encoder) error
}

// EncodeNull encodes a JSON null value.
func EncodeNull(e Encoder) error {
	return e.WriteToken("null", 'n')
}

// EncodeBool encodes a boolean value to JSON.
func EncodeBool[T ~bool](e Encoder, v T) error {
	if bool(v) {
		return e.WriteToken("true", 't')
	}
	return e.WriteToken("false", 'f')
}

// EncodeBoolPtr encodes a pointer to boolean value to JSON.
func EncodeBoolPtr[T ~bool](e Encoder, v *T) error {
	if v == nil {
		return EncodeNull(e)
	}
	return EncodeBool(e, *v)
}

// EncodeInt encodes an integer value to JSON.
func EncodeInt[T ~int | ~int8 | ~int16 | ~int32 | ~int64](e Encoder, i T) error {
	return e.WriteToken(strconv.FormatInt(int64(i), 10), '0')
}

// EncodeIntPtr encodes a pointer to integer value to JSON.
func EncodeIntPtr[T ~int | ~int8 | ~int16 | ~int32 | ~int64](e Encoder, i *T) error {
	if i == nil {
		return EncodeNull(e)
	}
	return EncodeInt(e, *i)
}

// EncodeIntKey encodes an integer-like map key as a JSON object member name.
func EncodeIntKey[T ~int | ~int8 | ~int16 | ~int32 | ~int64](e Encoder, v T) error {
	return e.WriteToken(strconv.FormatInt(int64(v), 10), '"')
}

// EncodeUint encodes an unsigned integer value to JSON.
func EncodeUint[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](e Encoder, u T) error {
	return e.WriteToken(strconv.FormatUint(uint64(u), 10), '0')
}

// EncodeUintPtr encodes a pointer to unsigned integer value to JSON.
func EncodeUintPtr[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](e Encoder, u *T) error {
	if u == nil {
		return EncodeNull(e)
	}
	return EncodeUint(e, *u)
}

// EncodeUintKey encodes an unsigned integer-like map key as a JSON object member name.
func EncodeUintKey[T ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64](e Encoder, v T) error {
	return e.WriteToken(strconv.FormatUint(uint64(v), 10), '"')
}

// EncodeFloat encodes a floating-point value to JSON.
func EncodeFloat[T ~float32 | ~float64](e Encoder, f T) error {
	switch {
	case math.IsNaN(float64(f)):
		return EncodeString(e, "NaN")
	case math.IsInf(float64(f), +1):
		return EncodeString(e, "Infinity")
	case math.IsInf(float64(f), -1):
		return EncodeString(e, "-Infinity")
	default:
		bitSize := 64
		switch any(f).(type) {
		case float32:
			bitSize = 32
		}
		return e.WriteToken(strconv.FormatFloat(float64(f), 'g', -1, bitSize), '0')
	}
}

// EncodeFloatPtr encodes a pointer to floating-point value to JSON.
func EncodeFloatPtr[T ~float32 | ~float64](e Encoder, f *T) error {
	if f == nil {
		return EncodeNull(e)
	}
	return EncodeFloat(e, *f)
}

// EncodeString encodes a string value to JSON.
func EncodeString[T ~string](e Encoder, s T) error {
	return e.WriteToken(string(s), '"')
}

// EncodeStringPtr encodes a pointer to string value to JSON.
func EncodeStringPtr[T ~string](e Encoder, s *T) error {
	if s == nil {
		return EncodeNull(e)
	}
	return EncodeString(e, *s)
}

// EncodeStringKey encodes a string-like map key as a JSON object member name.
func EncodeStringKey[T ~string](e Encoder, v T) error {
	return e.WriteToken(string(v), '"')
}

// EncodeBytes encodes bytes as a base64 JSON string.
func EncodeBytes(e Encoder, b []byte) error {
	if b == nil {
		return EncodeNull(e)
	}
	return EncodeString(e, base64.StdEncoding.EncodeToString(b))
}

// EncodeAny marshals an arbitrary Go value and writes it as a raw JSON value.
func EncodeAny[T any](e Encoder, v T) error {
	b, err := Marshal(v)
	if err != nil {
		return err
	}
	return e.WriteValue(b)
}

// EncodeObject encodes an object that implements JSONEncoder.
func EncodeObject[T JSONEncoder](e Encoder, v T) error {
	if JSONEncoder(v) == nil {
		return EncodeNull(e)
	}
	return v.EncodeJSON(e)
}

// EncodeArrayBegin encodes the opening token of a JSON array.
func EncodeArrayBegin(e Encoder) error {
	return e.WriteToken("[", '[')
}

// EncodeArrayEnd encodes the closing token of a JSON array.
func EncodeArrayEnd(e Encoder) error {
	return e.WriteToken("]", ']')
}

// EncodeArray encodes a slice using the provided item encoder.
func EncodeArray[T any](
	encodeItem func(Encoder, T) error,
) func(Encoder, []T) error {
	return func(e Encoder, arr []T) error {
		if arr == nil {
			return EncodeNull(e)
		}
		if err := EncodeArrayBegin(e); err != nil {
			return err
		}
		for _, v := range arr {
			if err := encodeItem(e, v); err != nil {
				return err
			}
		}
		return EncodeArrayEnd(e)
	}
}

// EncodeObjectBegin encodes the opening token of a JSON object.
func EncodeObjectBegin(e Encoder) error {
	return e.WriteToken("{", '{')
}

// EncodeObjectEnd encodes the closing token of a JSON object.
func EncodeObjectEnd(e Encoder) error {
	return e.WriteToken("}", '}')
}

// EncodeMap encodes a map as a JSON object using the provided key and value encoders.
func EncodeMap[K cmp.Ordered, V any](
	encodeKey func(Encoder, K) error,
	encodeVal func(Encoder, V) error,
) func(Encoder, map[K]V) error {
	return func(e Encoder, m map[K]V) error {
		if m == nil {
			return EncodeNull(e)
		}
		if err := EncodeObjectBegin(e); err != nil {
			return err
		}
		keys := slices.Collect(maps.Keys(m))
		slices.Sort(keys)
		for _, k := range keys {
			if err := encodeKey(e, k); err != nil {
				return err
			}
			if err := encodeVal(e, m[k]); err != nil {
				return err
			}
		}
		return EncodeObjectEnd(e)
	}
}
