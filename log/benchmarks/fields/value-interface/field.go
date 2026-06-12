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

package value_interface

import (
	"benchmark-fields/encoder"
)

// Field represents a structured log field with a key and a value.
type Field struct {
	Key string // The name of the field.
	Val Value  // The value of the field.
}

// Nil creates a Field with a nil value.
func Nil(key string) Field {
	return Reflect(key, nil)
}

// Bool creates a Field for a boolean value.
func Bool(key string, val bool) Field {
	return Field{Key: key, Val: BoolValue(val)}
}

// Int64 creates a Field for an int64 value.
func Int64(key string, val int64) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Float64 creates a Field for a float64 value.
func Float64(key string, val float64) Field {
	return Field{Key: key, Val: Float64Value(val)}
}

// String creates a Field for a string value.
func String(key string, val string) Field {
	return Field{Key: key, Val: StringValue(val)}
}

// Reflect wraps any value into a Field using reflection.
func Reflect(key string, val interface{}) Field {
	return Field{Key: key, Val: ReflectValue{Val: val}}
}

// Bools creates a Field with a slice of booleans.
func Bools(key string, val []bool) Field {
	return Field{Key: key, Val: BoolsValue(val)}
}

// Int64s creates a Field with a slice of int64 values.
func Int64s(key string, val []int64) Field {
	return Field{Key: key, Val: Int64sValue(val)}
}

// Float64s creates a Field with a slice of float64 values.
func Float64s(key string, val []float64) Field {
	return Field{Key: key, Val: Float64sValue(val)}
}

// Strings creates a Field with a slice of strings.
func Strings(key string, val []string) Field {
	return Field{Key: key, Val: StringsValue(val)}
}

// Array creates a Field containing a variadic slice of Values, wrapped as an array.
func Array(key string, val ...Value) Field {
	return Field{Key: key, Val: ArrayValue(val)}
}

// Object creates a Field containing a variadic slice of Fields, treated as a nested object.
func Object(key string, fields ...Field) Field {
	return Field{Key: key, Val: ObjectValue(fields)}
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

func (f *Field) Encode(enc encoder.Encoder) {
	enc.AppendKey(f.Key)
	f.Val.Encode(enc)
}
