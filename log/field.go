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

package log

import (
	"fmt"
	"math"
	"unsafe"

	"github.com/go-spring/stdlib/ordered"
)

const MsgKey = "msg"

// ValueType represents the underlying type stored in a Field.
// The Type determines how Num and Any should be interpreted.
type ValueType int

const (
	ValueTypeBool = ValueType(iota)
	ValueTypeInt64
	ValueTypeUint64
	ValueTypeFloat64
	ValueTypeString
	ValueTypeReflect
	ValueTypeArray
	ValueTypeObject
	ValueTypeFromMap
)

// Field represents a structured log field with a key and a typed value.
type Field struct {

	// The name of the field (e.g., "level", "message").
	Key string

	// The type of the value stored in the field (e.g., bool, int, string, etc.).
	Type ValueType

	// For numeric types, it stores the actual value.
	// For strings and arrays, it stores the length or relevant metadata.
	Num uint64

	// Holds the actual value based on the type:
	// - For strings, it stores a pointer to the string data.
	// - For arrays, it stores the slice of array elements.
	// - For other types, it stores the value directly.
	Any any
}

// Msg creates a string Field with the fixed key "msg".
func Msg(msg string) Field {
	return String(MsgKey, msg)
}

// Msgf formats a message and creates a Field with the fixed key "msg".
func Msgf(format string, args ...any) Field {
	return String(MsgKey, fmt.Sprintf(format, args...))
}

// Nil creates a Field whose value is nil (Type = ValueTypeReflect).
func Nil(key string) Field {
	return Reflect(key, nil)
}

// Bool creates a Field for a boolean value.
func Bool(key string, val bool) Field {
	if val {
		return Field{Key: key, Type: ValueTypeBool, Num: 1}
	}
	return Field{Key: key, Type: ValueTypeBool, Num: 0}
}

// BoolPtr creates a Field from a *bool, or Nil if pointer is nil.
func BoolPtr(key string, val *bool) Field {
	if val == nil {
		return Nil(key)
	}
	return Bool(key, *val)
}

// IntType is the type of int, int8, int16, int32, int64.
type IntType interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// Int creates a Field for an integer value.
func Int[T IntType](key string, val T) Field {
	return Field{Key: key, Type: ValueTypeInt64, Num: uint64(val)}
}

// IntPtr creates a Field from a *int, or Nil if pointer is nil.
func IntPtr[T IntType](key string, val *T) Field {
	if val == nil {
		return Nil(key)
	}
	return Int(key, *val)
}

// UintType is the type of uint, uint8, uint16, uint32, uint64.
type UintType interface {
	~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64
}

// Uint creates a Field for an unsigned integer value.
func Uint[T UintType](key string, val T) Field {
	return Field{Key: key, Type: ValueTypeUint64, Num: uint64(val)}
}

// UintPtr creates a Field from a *uint, or Nil if pointer is nil.
func UintPtr[T UintType](key string, val *T) Field {
	if val == nil {
		return Nil(key)
	}
	return Uint(key, *val)
}

// FloatType is the type of float32, float64.
type FloatType interface {
	~float32 | ~float64
}

// Float creates a Field for a float value.
func Float[T FloatType](key string, val T) Field {
	return Field{Key: key, Type: ValueTypeFloat64, Num: math.Float64bits(float64(val))}
}

// FloatPtr creates a Field from a *float, or Nil if pointer is nil.
func FloatPtr[T FloatType](key string, val *T) Field {
	if val == nil {
		return Nil(key)
	}
	return Float(key, *val)
}

// String creates a Field for a string value.
func String(key string, val string) Field {
	return Field{
		Key:  key,
		Type: ValueTypeString,
		Num:  uint64(len(val)),       // Store the length of the string
		Any:  unsafe.StringData(val), // Store the pointer to string data
	}
}

// StringPtr creates a Field from a *string, or Nil if pointer is nil.
func StringPtr(key string, val *string) Field {
	if val == nil {
		return Nil(key)
	}
	return String(key, *val)
}

// Reflect wraps any value into a Field using reflection.
func Reflect(key string, val any) Field {
	return Field{Key: key, Type: ValueTypeReflect, Any: val}
}

type bools []bool

// EncodeArray encodes a slice of bools into the encoder.
func (arr bools) EncodeArray(enc Encoder) {
	for _, v := range arr {
		enc.AppendBool(v)
	}
}

// Bools creates a Field with a slice of booleans.
func Bools(key string, val []bool) Field {
	return Array(key, bools(val))
}

type sliceOfInt[T IntType] []T

// EncodeArray encodes a slice of ints using the Encoder interface.
func (arr sliceOfInt[T]) EncodeArray(enc Encoder) {
	for _, v := range arr {
		enc.AppendInt64(int64(v))
	}
}

// Ints creates a Field with a slice of integers.
func Ints[T IntType](key string, val []T) Field {
	return Array(key, sliceOfInt[T](val))
}

type sliceOfUint[T UintType] []T

// EncodeArray encodes a slice of uints using the Encoder interface.
func (arr sliceOfUint[T]) EncodeArray(enc Encoder) {
	for _, v := range arr {
		enc.AppendUint64(uint64(v))
	}
}

// Uints creates a Field with a slice of unsigned integers.
func Uints[T UintType](key string, val []T) Field {
	return Array(key, sliceOfUint[T](val))
}

type sliceOfFloat[T FloatType] []T

// EncodeArray encodes a slice of float32s using the Encoder interface.
func (arr sliceOfFloat[T]) EncodeArray(enc Encoder) {
	for _, v := range arr {
		enc.AppendFloat64(float64(v))
	}
}

// Floats creates a Field with a slice of float32 values.
func Floats[T FloatType](key string, val []T) Field {
	return Array(key, sliceOfFloat[T](val))
}

type sliceOfString []string

// EncodeArray encodes a slice of strings using the Encoder interface.
func (arr sliceOfString) EncodeArray(enc Encoder) {
	for _, v := range arr {
		enc.AppendString(v)
	}
}

// Strings creates a Field with a slice of strings.
func Strings(key string, val []string) Field {
	return Array(key, sliceOfString(val))
}

// ArrayValue is an interface for types that can be encoded as array.
type ArrayValue interface {
	EncodeArray(enc Encoder)
}

// Array creates a Field with array type, using the ArrayValue interface.
func Array(key string, val ArrayValue) Field {
	return Field{Key: key, Type: ValueTypeArray, Any: val}
}

// Object creates a Field containing a variadic slice of Fields, treated as a nested object.
func Object(key string, fields ...Field) Field {
	return Field{Key: key, Type: ValueTypeObject, Any: fields}
}

// FieldsFromMap creates a special Field that wraps a map[string]any.
// When encoded, it expands the map into individual key-value fields.
// This allows existing map structures to be easily converted into log fields
// without manually iterating through the map and adding each field individually.
func FieldsFromMap(m map[string]any) Field {
	return Field{Key: "", Type: ValueTypeFromMap, Any: m}
}

// Any creates a Field from a value of any type by inspecting its dynamic type.
// It dispatches to the appropriate typed constructor based on the actual value.
// If the type is not explicitly handled, it falls back to using Reflect.
func Any(key string, value any) Field {
	switch val := value.(type) {
	case nil:
		return Nil(key)

	case bool:
		return Bool(key, val)
	case *bool:
		return BoolPtr(key, val)
	case []bool:
		return Bools(key, val)

	case int:
		return Int(key, val)
	case *int:
		return IntPtr(key, val)
	case []int:
		return Ints(key, val)

	case int8:
		return Int(key, val)
	case *int8:
		return IntPtr(key, val)
	case []int8:
		return Ints(key, val)

	case int16:
		return Int(key, val)
	case *int16:
		return IntPtr(key, val)
	case []int16:
		return Ints(key, val)

	case int32:
		return Int(key, val)
	case *int32:
		return IntPtr(key, val)
	case []int32:
		return Ints(key, val)

	case int64:
		return Int(key, val)
	case *int64:
		return IntPtr(key, val)
	case []int64:
		return Ints(key, val)

	case uint:
		return Uint(key, val)
	case *uint:
		return UintPtr(key, val)
	case []uint:
		return Uints(key, val)

	case uint8:
		return Uint(key, val)
	case *uint8:
		return UintPtr(key, val)
	case []uint8:
		return Uints(key, val)

	case uint16:
		return Uint(key, val)
	case *uint16:
		return UintPtr(key, val)
	case []uint16:
		return Uints(key, val)

	case uint32:
		return Uint(key, val)
	case *uint32:
		return UintPtr(key, val)
	case []uint32:
		return Uints(key, val)

	case uint64:
		return Uint(key, val)
	case *uint64:
		return UintPtr(key, val)
	case []uint64:
		return Uints(key, val)

	case float32:
		return Float(key, val)
	case *float32:
		return FloatPtr(key, val)
	case []float32:
		return Floats(key, val)

	case float64:
		return Float(key, val)
	case *float64:
		return FloatPtr(key, val)
	case []float64:
		return Floats(key, val)

	case string:
		return String(key, val)
	case *string:
		return StringPtr(key, val)
	case []string:
		return Strings(key, val)

	default:
		return Reflect(key, val)
	}
}

// Encode encodes the Field into the Encoder based on its type.
func (f Field) Encode(enc Encoder) {
	switch f.Type {
	case ValueTypeBool:
		enc.AppendKey(f.Key)
		enc.AppendBool(f.Num != 0)
	case ValueTypeInt64:
		enc.AppendKey(f.Key)
		enc.AppendInt64(int64(f.Num))
	case ValueTypeUint64:
		enc.AppendKey(f.Key)
		enc.AppendUint64(f.Num)
	case ValueTypeFloat64:
		enc.AppendKey(f.Key)
		enc.AppendFloat64(math.Float64frombits(f.Num))
	case ValueTypeString:
		enc.AppendKey(f.Key)
		enc.AppendString(unsafe.String(f.Any.(*byte), f.Num))
	case ValueTypeReflect:
		enc.AppendKey(f.Key)
		enc.AppendReflect(f.Any)
	case ValueTypeArray:
		enc.AppendKey(f.Key)
		enc.AppendArrayBegin()
		f.Any.(ArrayValue).EncodeArray(enc)
		enc.AppendArrayEnd()
	case ValueTypeObject:
		enc.AppendKey(f.Key)
		enc.AppendObjectBegin()
		EncodeFields(enc, f.Any.([]Field))
		enc.AppendObjectEnd()
	case ValueTypeFromMap:
		m := f.Any.(map[string]any)
		for _, k := range ordered.MapKeys(m) {
			Any(k, m[k]).Encode(enc)
		}
	default: // for linter
	}
}

// EncodeFields encodes a slice of Fields into the Encoder.
func EncodeFields(enc Encoder, fields []Field) {
	for _, f := range fields {
		f.Encode(enc)
	}
}
