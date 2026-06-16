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
	"math"
	"strconv"
	"strings"
	"testing"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/hashutil"
	"go-spring.org/stdlib/testing/assert"
)

func TestDecodeBool(t *testing.T) {
	t.Run("Decode true", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("true"))
		result, err := DecodeBool(d)
		assert.That(t, err).Nil()
		assert.That(t, result).True()
	})

	t.Run("Decode false", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("false"))
		result, err := DecodeBool(d)
		assert.That(t, err).Nil()
		assert.That(t, result).False()
	})

	t.Run("Decode null", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		_, err := DecodeBool(d)
		assert.Error(t, err).String("invalid JSON: expected boolean but got `null`")
	})

	t.Run("Decode invalid type", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`"invalid"`))
		_, err := DecodeBool(d)
		assert.Error(t, err).String("invalid JSON: expected boolean but got `invalid`")
	})
}

func TestDecodeBoolPtr(t *testing.T) {
	t.Run("Decode true pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("true"))
		result, err := DecodeBoolPtr(d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.That(t, *result).True()
	})

	t.Run("Decode false pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("false"))
		result, err := DecodeBoolPtr(d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.That(t, *result).False()
	})

	t.Run("Decode null pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeBoolPtr(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Nil()
	})
}

func TestDecodeInt(t *testing.T) {
	t.Run("Decode int", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		result, err := DecodeInt[int](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(123)
	})

	t.Run("Decode int8", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("127"))
		result, err := DecodeInt[int8](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(127)
	})

	t.Run("Decode int16", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("32767"))
		result, err := DecodeInt[int16](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(32767)
	})

	t.Run("Decode int32", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("2147483647"))
		result, err := DecodeInt[int32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(2147483647)
	})

	t.Run("Decode int64", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("9223372036854775807"))
		result, err := DecodeInt[int64](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(9223372036854775807)
	})

	t.Run("Decode negative", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("-42"))
		result, err := DecodeInt[int](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(-42)
	})

	t.Run("Decode min int8", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("-128"))
		result, err := DecodeInt[int8](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(int8(-128))
	})

	t.Run("Decode min int16", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("-32768"))
		result, err := DecodeInt[int16](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(int16(-32768))
	})

	t.Run("Decode min int32", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("-2147483648"))
		result, err := DecodeInt[int32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(int32(-2147483648))
	})

	t.Run("Decode min int64", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("-9223372036854775808"))
		result, err := DecodeInt[int64](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(int64(-9223372036854775808))
	})

	t.Run("Decode invalid character", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("abc"))
		_, err := DecodeInt[int8](d)
		assert.Error(t, err).String("jsontext: invalid character 'a' at start of value")
	})

	t.Run("Decode invalid int", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("3.0"))
		_, err := DecodeInt[int8](d)
		assert.Error(t, err).String("strconv.ParseInt: parsing \"3.0\": invalid syntax")
	})

	t.Run("Decode invalid int - 2", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("["))
		_, err := DecodeInt[int8](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `[`")
	})

	t.Run("Decode overflow", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("32767"))
		_, err := DecodeInt[int8](d)
		assert.Error(t, err).String("invalid JSON: number out of range, got `32767`")
	})
}

func TestDecodeIntPtr(t *testing.T) {
	t.Run("Decode int pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		result, err := DecodeIntPtr[int](d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.Number(t, *result).Equal(123)
	})

	t.Run("Decode null int pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeIntPtr[int](d)
		assert.That(t, err).Nil()
		assert.That(t, result).Nil()
	})

	t.Run("Decode int pointer with invalid element", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`"invalid"`))
		_, err := DecodeIntPtr[int](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `invalid`")
	})

	t.Run("Decode empty input for int pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(""))
		_, err := DecodeIntPtr[int](d)
		assert.Error(t, err).String("EOF")
	})

	t.Run("Decode invalid token for int pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("["))
		_, err := DecodeIntPtr[int](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `[`")
	})
}

func TestDecodeIntKey(t *testing.T) {
	t.Run("Decode int64 key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"123\""))
		result, err := DecodeIntKey[int64](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(int64(123))
	})

	t.Run("Decode int32 key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"456\""))
		result, err := DecodeIntKey[int32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(int32(456))
	})

	t.Run("Decode invalid int key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"invalid\""))
		_, err := DecodeIntKey[int](d)
		assert.Error(t, err).String("strconv.ParseInt: parsing \"invalid\": invalid syntax")
	})

	t.Run("Decode overflow int key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"32767\""))
		_, err := DecodeIntKey[int8](d)
		assert.Error(t, err).String("invalid JSON: number out of range, got `32767`")
	})

	t.Run("Decode numeric int key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		result, err := DecodeIntKey[int](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(123)
	})

	t.Run("Decode boolean int key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("true"))
		_, err := DecodeIntKey[int](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `true`")
	})
}

func TestDecodeUint(t *testing.T) {
	t.Run("Decode uint", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		result, err := DecodeUint[uint](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(123)
	})

	t.Run("Decode uint8", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("255"))
		result, err := DecodeUint[uint8](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(255)
	})

	t.Run("Decode uint16", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("65535"))
		result, err := DecodeUint[uint16](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(65535)
	})

	t.Run("Decode uint32", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("4294967295"))
		result, err := DecodeUint[uint32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(4294967295)
	})

	t.Run("Decode uint64", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("18446744073709551615"))
		result, err := DecodeUint[uint64](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(18446744073709551615)
	})

	t.Run("Decode invalid character", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("abc"))
		_, err := DecodeUint[uint8](d)
		assert.Error(t, err).String("jsontext: invalid character 'a' at start of value")
	})

	t.Run("Decode invalid uint", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("3.0"))
		_, err := DecodeUint[uint8](d)
		assert.Error(t, err).String("strconv.ParseUint: parsing \"3.0\": invalid syntax")
	})

	t.Run("Decode invalid uint - 2", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("["))
		_, err := DecodeUint[uint8](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `[`")
	})

	t.Run("Decode overflow", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("65535"))
		_, err := DecodeUint[uint8](d)
		assert.Error(t, err).String("invalid JSON: number out of range, got `65535`")
	})
}

func TestDecodeUintPtr(t *testing.T) {
	t.Run("Decode uint pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		result, err := DecodeUintPtr[uint](d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.Number(t, *result).Equal(123)
	})

	t.Run("Decode uint8 pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("255"))
		result, err := DecodeUintPtr[uint8](d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.Number(t, *result).Equal(255)
	})

	t.Run("Decode null uint pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeUintPtr[uint](d)
		assert.That(t, err).Nil()
		assert.That(t, result).Nil()
	})

	t.Run("Decode uint pointer with invalid element", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`"invalid"`))
		_, err := DecodeUintPtr[uint](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `invalid`")
	})
}

func TestDecodeUintKey(t *testing.T) {
	t.Run("Decode uint64 key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"123\""))
		result, err := DecodeUintKey[uint64](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(uint64(123))
	})

	t.Run("Decode uint32 key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"456\""))
		result, err := DecodeUintKey[uint32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(uint32(456))
	})

	t.Run("Decode invalid uint key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"invalid\""))
		_, err := DecodeUintKey[uint](d)
		assert.Error(t, err).String("strconv.ParseUint: parsing \"invalid\": invalid syntax")
	})

	t.Run("Decode overflow uint key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("\"65535\""))
		_, err := DecodeUintKey[uint8](d)
		assert.Error(t, err).String("invalid JSON: number out of range, got `65535`")
	})

	t.Run("Decode numeric uint key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		result, err := DecodeUintKey[uint](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(uint(123))
	})

	t.Run("Decode boolean uint key", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("true"))
		_, err := DecodeUintKey[uint](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `true`")
	})
}

func TestDecodeFloat(t *testing.T) {
	t.Run("Decode float32", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123.45"))
		result, err := DecodeFloat[float32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(123.45)
	})

	t.Run("Decode float64", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123.456789"))
		result, err := DecodeFloat[float64](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(123.456789)
	})

	t.Run("Decode negative float", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("-123.45"))
		result, err := DecodeFloat[float32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(-123.45)
	})

	t.Run("Decode scientific notation", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("1.23e2"))
		result, err := DecodeFloat[float32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(123.0)
	})

	t.Run("Decode very small float", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("1e-10"))
		result, err := DecodeFloat[float32](d)
		assert.That(t, err).Nil()
		assert.Number(t, result).Equal(1e-10)
	})

	t.Run("Decode invalid float", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("1e520"))
		_, err := DecodeFloat[float32](d)
		assert.Error(t, err).String("strconv.ParseFloat: parsing \"1e520\": value out of range")
	})

	t.Run("Decode invalid float - 2", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("["))
		_, err := DecodeFloat[float32](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `[`")
	})

	t.Run("Decode overflow float32", func(t *testing.T) {
		overflowValue := math.MaxFloat32 * 2
		d := NewDecoder(strings.NewReader(strconv.FormatFloat(overflowValue, 'f', -1, 64)))
		_, err := DecodeFloat[float32](d)
		assert.Error(t, err).String("invalid JSON: number out of range, got `680564693277057700000000000000000000000`")
	})
}

func TestDecodeFloatPtr(t *testing.T) {
	t.Run("Decode float32 pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123.45"))
		result, err := DecodeFloatPtr[float32](d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.Number(t, *result).Equal(123.45)
	})

	t.Run("Decode float64 pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123.456789"))
		result, err := DecodeFloatPtr[float64](d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.Number(t, *result).Equal(123.456789)
	})

	t.Run("Decode null float pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeFloatPtr[float32](d)
		assert.That(t, err).Nil()
		assert.That(t, result).Nil()
	})

	t.Run("Decode negative float pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("-123.45"))
		result, err := DecodeFloatPtr[float32](d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.Number(t, *result).Equal(-123.45)
	})

	t.Run("Decode scientific notation pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("1.23e2"))
		result, err := DecodeFloatPtr[float32](d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.Number(t, *result).Equal(123.0)
	})

	t.Run("Decode float pointer with invalid element", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`"invalid"`))
		_, err := DecodeFloatPtr[float32](d)
		assert.Error(t, err).String("invalid JSON: expected number but got `invalid`")
	})
}

func TestDecodeString(t *testing.T) {
	t.Run("Decode simple string", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`"hello"`))
		result, err := DecodeString(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal("hello")
	})

	t.Run("Decode empty string", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`""`))
		result, err := DecodeString(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal("")
	})

	t.Run("Decode string with special chars", func(t *testing.T) {
		expected := "hello\nworld\t\"quoted\""
		d := NewDecoder(strings.NewReader(`"hello\nworld\t\"quoted\""`))
		result, err := DecodeString(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode string with more complex escape sequences", func(t *testing.T) {
		expected := "hello\r\n\t\\\"/world"
		d := NewDecoder(strings.NewReader(`"hello\r\n\t\\\"/world"`))
		result, err := DecodeString(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode string with unicode escape", func(t *testing.T) {
		expected := "Hello 世界"
		d := NewDecoder(strings.NewReader(`"Hello \u4e16\u754c"`))
		result, err := DecodeString(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode invalid type", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		_, err := DecodeString(d)
		assert.Error(t, err).String("invalid JSON: expected string but got `123`")
	})
}

func TestDecodeStringPtr(t *testing.T) {
	t.Run("Decode string pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`"hello"`))
		result, err := DecodeStringPtr(d)
		assert.That(t, err).Nil()
		assert.That(t, result).NotNil()
		assert.That(t, *result).Equal("hello")
	})

	t.Run("Decode null string pointer", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeStringPtr(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Nil()
	})
}

func TestDecodeBytes(t *testing.T) {
	t.Run("Decode base64 bytes", func(t *testing.T) {
		originalBytes := []byte("hello world")
		encoded := base64.StdEncoding.EncodeToString(originalBytes)
		d := NewDecoder(strings.NewReader(`"` + encoded + `"`))
		result, err := DecodeBytes(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal(originalBytes)
	})

	t.Run("Decode empty bytes", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte{})
		d := NewDecoder(strings.NewReader(`"` + encoded + `"`))
		result, err := DecodeBytes(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal([]byte{})
	})

	t.Run("Decode null bytes", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeBytes(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Nil()
	})

	t.Run("Decode invalid base64", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`"invalid_base64!"`))
		_, err := DecodeBytes(d)
		assert.Error(t, err).String("illegal base64 data at input byte 7")
	})

	t.Run("Decode invalid type", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		_, err := DecodeBytes(d)
		assert.Error(t, err).String("invalid JSON: expected string but got `123`")
	})
}

func TestDecodeArray(t *testing.T) {
	t.Run("Decode int array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[1, 2, 3, 4, 5]"))
		result, err := DecodeArray(DecodeInt[int])(d)
		assert.That(t, err).Nil()
		assert.Slice(t, result).Equal([]int{1, 2, 3, 4, 5})
	})

	t.Run("Decode uint array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[1, 2, 3, 4, 5]"))
		result, err := DecodeArray(DecodeUint[uint])(d)
		assert.That(t, err).Nil()
		assert.Slice(t, result).Equal([]uint{1, 2, 3, 4, 5})
	})

	t.Run("Decode string array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`["a", "b", "c"]`))
		result, err := DecodeArray(DecodeString)(d)
		assert.That(t, err).Nil()
		assert.Slice(t, result).Equal([]string{"a", "b", "c"})
	})

	t.Run("Decode empty array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[]"))
		result, err := DecodeArray(DecodeInt[int])(d)
		assert.That(t, err).Nil()
		assert.Slice(t, result).Equal([]int{})
	})

	t.Run("Decode null array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeArray(DecodeInt[int])(d)
		assert.That(t, err).Nil()
		assert.Slice(t, result).Nil()
	})

	t.Run("Decode nested int array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[[1, 2], [3, 4, 5], [6]]"))
		result, err := DecodeArray(DecodeArray(DecodeInt[int]))(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal([][]int{{1, 2}, {3, 4, 5}, {6}})
	})

	t.Run("Decode nested string array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`[["a", "b"], ["c", "d", "e"], ["f"]]`))
		result, err := DecodeArray(DecodeArray(DecodeString))(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal([][]string{{"a", "b"}, {"c", "d", "e"}, {"f"}})
	})

	t.Run("Decode deeply nested array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[[[1, 2]], [[3, 4], [5, 6]]]"))
		result, err := DecodeArray(DecodeArray(DecodeArray(DecodeInt[int])))(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal([][][]int{{{1, 2}}, {{3, 4}, {5, 6}}})
	})

	t.Run("Decode empty nested array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[[], [], []]"))
		result, err := DecodeArray(DecodeArray(DecodeInt[int]))(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal([][]int{{}, {}, {}})
	})

	t.Run("Decode mixed nested array with empty", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[[1, 2], [], [3, 4]]"))
		result, err := DecodeArray(DecodeArray(DecodeInt[int]))(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Equal([][]int{{1, 2}, {}, {3, 4}})
	})

	t.Run("Decode invalid array type", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		_, err := DecodeArray(DecodeInt[int])(d)
		assert.Error(t, err).String("invalid JSON: expected `[` but got `123`")
	})

	t.Run("Decode int array with invalid element", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[1, 2, \"invalid\", 4]"))
		_, err := DecodeArray(DecodeInt[int])(d)
		assert.Error(t, err).String("invalid JSON: expected number but got `invalid`")
	})

	t.Run("Decode uint array with invalid element", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("[1, 2, \"invalid\", 4]"))
		_, err := DecodeArray(DecodeUint[uint])(d)
		assert.Error(t, err).String("invalid JSON: expected number but got `invalid`")
	})
}

func TestDecodeMap(t *testing.T) {
	t.Run("Decode string-int map", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`{"a": 1, "b": 2, "c": 3}`))
		result, err := DecodeMap(DecodeString, DecodeInt[int])(d)
		assert.That(t, err).Nil()
		assert.Map(t, result).Equal(map[string]int{"a": 1, "b": 2, "c": 3})
	})

	t.Run("Decode int-int map", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`{"1": 10, "2": 20, "3": 30}`))
		result, err := DecodeMap(DecodeIntKey[int], DecodeInt[int])(d)
		assert.That(t, err).Nil()
		assert.Map(t, result).Equal(map[int]int{1: 10, 2: 20, 3: 30})
	})

	t.Run("Decode empty map", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("{}"))
		result, err := DecodeMap(DecodeString, DecodeInt[int])(d)
		assert.That(t, err).Nil()
		assert.Map(t, result).Equal(map[string]int{})
	})

	t.Run("Decode null map", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeMap(DecodeString, DecodeInt[int])(d)
		assert.That(t, err).Nil()
		assert.Map(t, result).Nil()
	})

	t.Run("Decode string-int map nested in array", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`[{"a": 1, "b": 2}, {"c": 3, "d": 4}]`))
		result, err := DecodeArray(DecodeMap(DecodeString, DecodeInt[int]))(d)
		assert.That(t, err).Nil()
		expected := []map[string]int{
			{"a": 1, "b": 2},
			{"c": 3, "d": 4},
		}
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode nested map string-int", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`{"first": {"a": 1, "b": 2}, "second": {"c": 3, "d": 4}}`))
		result, err := DecodeMap(DecodeString, DecodeMap(DecodeString, DecodeInt[int]))(d)
		assert.That(t, err).Nil()
		expected := map[string]map[string]int{
			"first":  {"a": 1, "b": 2},
			"second": {"c": 3, "d": 4},
		}
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode nested map int-int", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`{"1": {"10": 100, "11": 110}, "2": {"20": 200, "21": 210}}`))
		result, err := DecodeMap(DecodeIntKey[int], DecodeMap(DecodeIntKey[int], DecodeInt[int]))(d)
		assert.That(t, err).Nil()
		expected := map[int]map[int]int{
			1: {10: 100, 11: 110},
			2: {20: 200, 21: 210},
		}
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode deeply nested structure: map of arrays of maps", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`{"data": [{"key1": 100, "key2": 200}, {"key3": 300}]}`))
		result, err := DecodeMap(DecodeString, DecodeArray(DecodeMap(DecodeString, DecodeInt[int])))(d)
		assert.That(t, err).Nil()
		expected := map[string][]map[string]int{
			"data": {
				{"key1": 100, "key2": 200},
				{"key3": 300},
			},
		}
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode complex nested structure: array of maps of arrays", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`[{"numbers": [1, 2, 3]}, {"numbers": [4, 5]}, {"other": [10, 20]}]`))
		result, err := DecodeArray(DecodeMap(DecodeString, DecodeArray(DecodeInt[int])))(d)
		assert.That(t, err).Nil()
		expected := []map[string][]int{
			{"numbers": {1, 2, 3}},
			{"numbers": {4, 5}},
			{"other": {10, 20}},
		}
		assert.That(t, result).Equal(expected)
	})

	t.Run("Decode invalid map type", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		_, err := DecodeMap(DecodeString, DecodeInt[int])(d)
		assert.Error(t, err).String("invalid JSON: expected `{` but got `123`")
	})

	t.Run("Decode map with invalid value", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("{\"a\": \"invalid\", \"b\": 2}"))
		_, err := DecodeMap(DecodeString, DecodeInt[int])(d)
		assert.Error(t, err).String("invalid JSON: expected number but got `invalid`")
	})
}

func TestDecodeObjectBegin(t *testing.T) {
	t.Run("Decode object begin with read token error", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(""))
		err := DecodeObjectBegin(d)
		assert.Error(t, err).String("EOF")
	})

	t.Run("Decode object begin with invalid token", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("123"))
		err := DecodeObjectBegin(d)
		assert.Error(t, err).String("invalid JSON: expected `{` but got `123`")
	})
}

func TestDecodeObjectEnd(t *testing.T) {
	t.Run("Decode object end with read token error", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`{"key": "value"`))
		_, _, _ = d.ReadToken()
		_, _, _ = d.ReadToken()
		_, _, _ = d.ReadToken()
		err := DecodeObjectEnd(d)
		assert.Error(t, err).String("jsontext: unexpected EOF after offset 15")
	})

	t.Run("Decode object end with invalid token", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`["key", "value"]`))
		_, _, _ = d.ReadToken()
		_, _, _ = d.ReadToken()
		_, _, _ = d.ReadToken()
		err := DecodeObjectEnd(d)
		assert.Error(t, err).String("invalid JSON: expected `}` but got `]`")
	})
}

func TestDecodeAny(t *testing.T) {
	t.Run("Decode any with read token error", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(""))
		_, err := DecodeAny[any](d)
		assert.Error(t, err).String("EOF")
	})

	t.Run("Decode any with unmarshal error", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("1e520"))
		_, err := DecodeAny[any](d)
		assert.Error(t, err).Matches("json: .* unmarshal JSON number 1e520 into Go float64: value out of range")
	})
}

type TestObject struct {

	// Base
	Int    int
	IntPtr *int
	Bytes  []byte
	Any    any

	// Object
	Object *TestObject

	// List
	StrList       []string
	StrPtrList    []*string
	ObjectList    []*TestObject
	AnyList       []any
	IntIntList    [][]int
	StrIntMapList []map[string]int64

	// Map
	IntIntMap       map[int64]int
	StrStrPtrMap    map[string]*string
	StrObjectMap    map[string]*TestObject
	StrIntMapIntMap map[int]map[string]int
	StrAnyListMap   map[string][]any
}

func NewTestObject() *TestObject {
	return &TestObject{}
}

func (b *TestObject) EncodeJSON(e Encoder) error {
	return nil
}

func (b *TestObject) DecodeJSON(d Decoder) error {
	const (
		hashInt             = 0x41a91f19c98dd49e // HashKey("Int")
		hashIntPtr          = 0x3305f2829a12fcb8 // HashKey("IntPtr")
		hashBytes           = 0xeeeea7adc131a244 // HashKey("Bytes")
		hashAny             = 0xf999d4199ff4542d // HashKey("Any")
		hashObject          = 0x730182ad28374cda // HashKey("Object")
		hashStrList         = 0x5b6d2f7ec3876598 // HashKey("StrList")
		hashStrPtrList      = 0xbcd41074374edba6 // HashKey("StrPtrList")
		hashObjectList      = 0xe57a6677f7f85d56 // HashKey("ObjectList")
		hashAnyList         = 0xd3e7d7f09c081849 // HashKey("AnyList")
		hashIntIntList      = 0x3b355d10e98d9fd7 // HashKey("IntIntList")
		hashStrIntMapList   = 0x91322ae7f82e5179 // HashKey("StrIntMapList")
		hashIntIntMap       = 0xc3172d7c329b5eef // HashKey("IntIntMap")
		hashStrStrPtrMap    = 0xc3447c678a33494b // HashKey("StrStrPtrMap")
		hashStrObjectMap    = 0xffe2e175eafdff87 // HashKey("StrObjectMap")
		hashStrIntMapIntMap = 0x1f17542c3a1b0c80 // HashKey("StrIntMapIntMap")
		hashStrAnyListMap   = 0x89083baaf946de20 // HashKey("StrAnyListMap")
	)

	if err := DecodeObjectBegin(d); err != nil {
		return err
	}

	// 设置默认值
	b.Int = 9

	// 记录必传字段
	var (
		foundInt bool
	)

	for d.PeekKind() != '}' {
		key, err := DecodeString(d)
		if err != nil {
			return err
		}
		switch hashutil.FNV1a64(key) {
		case hashInt:
			if b.Int, err = DecodeInt[int](d); err != nil {
				return err
			}
			foundInt = true
		case hashIntPtr:
			if b.IntPtr, err = DecodeIntPtr[int](d); err != nil {
				return err
			}
		case hashBytes:
			if b.Bytes, err = DecodeBytes(d); err != nil {
				return err
			}
		case hashAny:
			if b.Any, err = DecodeAny[any](d); err != nil {
				return err
			}
		case hashObject:
			if b.Object, err = DecodeObject(NewTestObject)(d); err != nil {
				return err
			}
		case hashStrList:
			if b.StrList, err = DecodeArray(DecodeString)(d); err != nil {
				return err
			}
		case hashStrPtrList:
			if b.StrPtrList, err = DecodeArray(DecodeStringPtr)(d); err != nil {
				return err
			}
		case hashObjectList:
			if b.ObjectList, err = DecodeArray(DecodeObject(NewTestObject))(d); err != nil {
				return err
			}
		case hashAnyList:
			if b.AnyList, err = DecodeArray(DecodeAny[any])(d); err != nil {
				return err
			}
		case hashIntIntList:
			if b.IntIntList, err = DecodeArray(DecodeArray(DecodeInt[int]))(d); err != nil {
				return err
			}
		case hashStrIntMapList:
			if b.StrIntMapList, err = DecodeArray(DecodeMap(DecodeString, DecodeInt[int64]))(d); err != nil {
				return err
			}
		case hashIntIntMap:
			if b.IntIntMap, err = DecodeMap(DecodeIntKey[int64], DecodeInt[int])(d); err != nil {
				return err
			}
		case hashStrStrPtrMap:
			if b.StrStrPtrMap, err = DecodeMap(DecodeString, DecodeStringPtr)(d); err != nil {
				return err
			}
		case hashStrObjectMap:
			if b.StrObjectMap, err = DecodeMap(DecodeString, DecodeObject(NewTestObject))(d); err != nil {
				return err
			}
		case hashStrIntMapIntMap:
			if b.StrIntMapIntMap, err = DecodeMap(DecodeIntKey[int], DecodeMap(DecodeString, DecodeInt[int]))(d); err != nil {
				return err
			}
		case hashStrAnyListMap:
			if b.StrAnyListMap, err = DecodeMap(DecodeString, DecodeArray(DecodeAny[any]))(d); err != nil {
				return err
			}
		default:
			if err = d.SkipValue(); err != nil {
				return err
			}
		}
	}

	if err := DecodeObjectEnd(d); err != nil {
		return err
	}

	// 检查必传字段
	if !foundInt {
		return errutil.Explain(nil, "missing required field Int")
	}
	return nil
}

func TestDecodeObject(t *testing.T) {

	t.Run("Normal case with all fields", func(t *testing.T) {
		s := `{
			"Int": 3,
			"IntPtr": 3,
			"String": "hello",
			"StringPtr": "world",
			"Bytes": "aGVsbG8=",
			"Any": "any_value",
			"Object": {
				"Int": 5,
				"IntPtr": 5,
				"String": "nested",
				"StringPtr": "object",
				"Bytes": "bmVzdGVk"
			},
			"StrList": ["str1", "str2", "str3"],
			"StrPtrList": ["ptr1", "ptr2"],
			"ObjectList": [
				{
					"Int": 7,
					"IntPtr": 7,
					"String": "in",
					"StringPtr": "list",
					"Bytes": "bGlzdA=="
				},
				{
					"Int": 8,
					"IntPtr": 8,
					"String": "second",
					"StringPtr": "object",
					"Bytes": "c2Vjb25k"
				}
			],
			"AnyList": ["any1", 123, true],
			"IntIntList": [[1, 2], [3, 4, 5], [6]],
			"StrIntMapList": [
				{"key1": 10, "key2": 20},
				{"key3": 30}
			],
			"IntIntMap": {
				"1": 10,
				"2": 20
			},
			"StrStrPtrMap": {
				"map_key1": "map_value1",
				"map_key2": "map_value2"
			},
			"StrObjectMap": {
				"obj_key1": {
					"Int": 9,
					"IntPtr": 9,
					"String": "in",
					"StringPtr": "map",
					"Bytes": "bWFw"
				},
				"obj_key2": {
					"Int": 10,
					"IntPtr": 10,
					"String": "second",
					"StringPtr": "map_obj",
					"Bytes": "bWFwX29iZA=="
				}
			},
			"StrIntMapIntMap": {
				"1": {
					"nested_key1": 100,
					"nested_key2": 200
				},
				"2": {
					"nested_key3": 300
				}
			},
			"StrAnyListMap": {
				"list_key1": ["a", 1, true],
				"list_key2": ["b", 2, false]
			}
		}`
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.That(t, err).Nil()
		assert.That(t, o.Int).Equal(3)
		assert.That(t, *o.IntPtr).Equal(3)
		assert.That(t, o.Bytes).Equal([]byte("hello"))
		assert.That(t, o.Any).Equal("any_value")
		assert.That(t, o.Object).NotNil()
		assert.That(t, o.Object.Int).Equal(5)
		assert.That(t, o.StrList).Equal([]string{"str1", "str2", "str3"})
		assert.That(t, o.StrPtrList).Equal([]*string{new("ptr1"), new("ptr2")})
		assert.That(t, len(o.ObjectList)).Equal(2)
		assert.That(t, o.ObjectList[0].Int).Equal(7)
		assert.That(t, o.ObjectList[1].Int).Equal(8)
		assert.That(t, o.AnyList).Equal([]any{"any1", float64(123), true})
		assert.That(t, o.IntIntList).Equal([][]int{{1, 2}, {3, 4, 5}, {6}})
		assert.That(t, o.StrIntMapList).Equal([]map[string]int64{{"key1": 10, "key2": 20}, {"key3": 30}})
		assert.That(t, o.IntIntMap).Equal(map[int64]int{1: 10, 2: 20})
		assert.That(t, o.StrStrPtrMap).Equal(map[string]*string{"map_key1": new("map_value1"), "map_key2": new("map_value2")})
		assert.That(t, len(o.StrObjectMap)).Equal(2)
		assert.That(t, o.StrObjectMap["obj_key1"]).NotNil()
		assert.That(t, o.StrObjectMap["obj_key1"].Int).Equal(9)
		assert.That(t, o.StrObjectMap["obj_key2"].Int).Equal(10)
		assert.That(t, o.StrIntMapIntMap).Equal(map[int]map[string]int{1: {"nested_key1": 100, "nested_key2": 200}, 2: {"nested_key3": 300}})
		assert.That(t, o.StrAnyListMap).Equal(map[string][]any{"list_key1": {"a", float64(1), true}, "list_key2": {"b", float64(2), false}})
	})

	t.Run("Decode object with null", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("null"))
		result, err := DecodeObject(NewTestObject)(d)
		assert.That(t, err).Nil()
		assert.That(t, result).Nil()
	})

	t.Run("Missing required Int field", func(t *testing.T) {
		s := `{"IntPtr": 5}`
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.Error(t, err).String("missing required field Int")
	})

	t.Run("Empty object with default value", func(t *testing.T) {
		s := `{}`
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.Error(t, err).String("missing required field Int")
	})

	t.Run("Invalid JSON format", func(t *testing.T) {
		s := `{"Int": 3, "IntPtr": 5`
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.That(t, err).NotNil()
	})

	t.Run("Invalid field type for Int field", func(t *testing.T) {
		s := `{"Int": "invalid", "IntPtr": 5}`
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.That(t, err).NotNil()
	})

	t.Run("Invalid nested object", func(t *testing.T) {
		s := `{"Int": 3, "Object": {"IntPtr": 5}}` // Missing required Int in nested object
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.That(t, err).NotNil()
	})

	t.Run("Valid object with minimal fields", func(t *testing.T) {
		s := `{"Int": 10}`
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.That(t, err).Nil()
		assert.That(t, o.Int).Equal(10)
	})

	t.Run("Complex nested object with all fields", func(t *testing.T) {
		s := `{"Int": 100, "Object": {"Int": 200, "Object": {"Int": 300, "IntPtr": 300}}}`
		o := &TestObject{}
		d := NewDecoder(strings.NewReader(s))
		err := o.DecodeJSON(d)
		assert.That(t, err).Nil()
		assert.That(t, o.Int).Equal(100)
		assert.That(t, o.Object).NotNil()
		assert.That(t, o.Object.Int).Equal(200)
		assert.That(t, o.Object.Object).NotNil()
		assert.That(t, o.Object.Object.Int).Equal(300)
		assert.That(t, *o.Object.Object.IntPtr).Equal(300)
	})
}
