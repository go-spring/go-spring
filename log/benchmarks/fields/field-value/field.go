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

package field_value

import (
	"math"

	"benchmark-fields/encoder"
)

type ValueType int

const (
	ValueTypeBool = ValueType(iota)
	ValueTypeInt64
	ValueTypeFloat64
	ValueTypeString
	ValueTypeReflect
	ValueTypeBools
	ValueTypeInt64s
	ValueTypeFloat64s
	ValueTypeStrings
	ValueTypeArray
	ValueTypeObject
)

// Field represents a structured log field with a key and it's value.
type Field struct {
	Key  string
	Type ValueType
	Num  uint64
	Str  string
	Any  any
}

// Nil creates a Field with a nil value.
func Nil(key string) Field {
	return Reflect(key, nil)
}

// Bool creates a Field for a boolean value.
func Bool(key string, val bool) Field {
	if val {
		return Field{
			Key:  key,
			Type: ValueTypeBool,
			Num:  1,
		}
	}
	return Field{
		Key:  key,
		Type: ValueTypeBool,
		Num:  0,
	}
}

// Int64 creates a Field for an int64 value.
func Int64(key string, val int64) Field {
	return Field{
		Key:  key,
		Type: ValueTypeInt64,
		Num:  uint64(val),
	}
}

// Float64 creates a Field for a float64 value.
func Float64(key string, val float64) Field {
	return Field{
		Key:  key,
		Type: ValueTypeFloat64,
		Num:  math.Float64bits(val),
	}
}

// String creates a Field for a string value.
func String(key string, val string) Field {
	return Field{
		Key:  key,
		Type: ValueTypeString,
		Str:  val,
	}
}

// Reflect wraps any value into a Field using reflection.
func Reflect(key string, val interface{}) Field {
	return Field{
		Key:  key,
		Type: ValueTypeReflect,
		Any:  val,
	}
}

// Bools creates a Field with a slice of booleans.
func Bools(key string, val []bool) Field {
	return Field{
		Key:  key,
		Type: ValueTypeBools,
		Any:  val,
	}
}

// Int64s creates a Field with a slice of int64 values.
func Int64s(key string, val []int64) Field {
	return Field{
		Key:  key,
		Type: ValueTypeInt64s,
		Any:  val,
	}
}

// Float64s creates a Field with a slice of float64 values.
func Float64s(key string, val []float64) Field {
	return Field{
		Key:  key,
		Type: ValueTypeFloat64s,
		Any:  val,
	}
}

// Strings creates a Field with a slice of strings.
func Strings(key string, val []string) Field {
	return Field{
		Key:  key,
		Type: ValueTypeStrings,
		Any:  val,
	}
}

// ArrayValue represents a slice of Values.
type ArrayValue interface {
	EncodeArray(enc encoder.Encoder)
}

// Array creates a Field containing a variadic slice of Values, wrapped as an array.
func Array(key string, val ArrayValue) Field {
	return Field{
		Key:  key,
		Type: ValueTypeArray,
		Any:  val,
	}
}

// Object creates a Field containing a variadic slice of Fields, treated as a nested object.
func Object(key string, fields ...Field) Field {
	return Field{
		Key:  key,
		Type: ValueTypeObject,
		Any:  fields,
	}
}

// Any creates a Field from a value of any type by inspecting its dynamic type.
// It dispatches to the appropriate typed constructor based on the actual value.
// If the type is not explicitly handled, it falls back to using Reflect.
func Any(key string, value interface{}) Field {
	switch val := value.(type) {
	case nil:
		return Nil(key)
	case bool:
		return Bool(key, val)
	case []bool:
		return Bools(key, val)
	case int64:
		return Int64(key, val)
	case []int64:
		return Int64s(key, val)
	case float64:
		return Float64(key, val)
	case []float64:
		return Float64s(key, val)
	case string:
		return String(key, val)
	case []string:
		return Strings(key, val)
	default:
		return Reflect(key, val)
	}
}

// Encode encodes the data represented by v to an Encoder.
func (f *Field) Encode(enc encoder.Encoder) {
	enc.AppendKey(f.Key)
	switch f.Type {
	case ValueTypeBool:
		enc.AppendBool(f.Num != 0)
	case ValueTypeInt64:
		enc.AppendInt64(int64(f.Num))
	case ValueTypeFloat64:
		enc.AppendFloat64(math.Float64frombits(f.Num))
	case ValueTypeString:
		enc.AppendString(f.Str)
	case ValueTypeReflect:
		enc.AppendReflect(f.Any)
	case ValueTypeBools:
		enc.AppendArrayBegin()
		for _, val := range f.Any.([]bool) {
			enc.AppendBool(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeInt64s:
		enc.AppendArrayBegin()
		for _, val := range f.Any.([]int64) {
			enc.AppendInt64(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeFloat64s:
		enc.AppendArrayBegin()
		for _, val := range f.Any.([]float64) {
			enc.AppendFloat64(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeStrings:
		enc.AppendArrayBegin()
		for _, val := range f.Any.([]string) {
			enc.AppendString(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeArray:
		enc.AppendArrayBegin()
		f.Any.(ArrayValue).EncodeArray(enc)
		enc.AppendArrayEnd()
	case ValueTypeObject:
		enc.AppendObjectBegin()
		for _, val := range f.Any.([]Field) {
			val.Encode(enc)
		}
		enc.AppendObjectEnd()
	default: // for linter
	}
}
