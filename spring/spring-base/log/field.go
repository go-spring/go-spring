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

package log

// Field is used to replace printf operation with lower cost.
type Field struct {
	Key string
	Val Value
}

// Nil constructs a field that carries a nil.
func Nil(key string) Field {
	return Reflect(key, nil)
}

// Bool constructs a field that carries a bool.
func Bool(key string, val bool) Field {
	return Field{Key: key, Val: BoolValue(val)}
}

// BoolPtr constructs a field that carries a *bool.
func BoolPtr(key string, val *bool) Field {
	if val == nil {
		return Nil(key)
	}
	return Bool(key, *val)
}

// Int constructs a field that carries an int.
func Int(key string, val int) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// IntPtr constructs a field that carries a *int.
func IntPtr(key string, val *int) Field {
	if val == nil {
		return Nil(key)
	}
	return Int(key, *val)
}

// Int8 constructs a field that carries an int8.
func Int8(key string, val int8) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int8Ptr constructs a field that carries a *int8.
func Int8Ptr(key string, val *int8) Field {
	if val == nil {
		return Nil(key)
	}
	return Int8(key, *val)
}

// Int16 constructs a field that carries an int16.
func Int16(key string, val int16) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int16Ptr constructs a field that carries a *int16.
func Int16Ptr(key string, val *int16) Field {
	if val == nil {
		return Nil(key)
	}
	return Int16(key, *val)
}

// Int32 constructs a field that carries an int32.
func Int32(key string, val int32) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int32Ptr constructs a field that carries a *int32.
func Int32Ptr(key string, val *int32) Field {
	if val == nil {
		return Nil(key)
	}
	return Int32(key, *val)
}

// Int64 constructs a field that carries an int64.
func Int64(key string, val int64) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int64Ptr constructs a field that carries a *int64.
func Int64Ptr(key string, val *int64) Field {
	if val == nil {
		return Nil(key)
	}
	return Int64(key, *val)
}

// Uint constructs a field that carries an uint.
func Uint(key string, val uint) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// UintPtr constructs a field that carries a *uint.
func UintPtr(key string, val *uint) Field {
	if val == nil {
		return Nil(key)
	}
	return Uint(key, *val)
}

// Uint8 constructs a field that carries an uint8.
func Uint8(key string, val uint8) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint8Ptr constructs a field that carries a *uint8.
func Uint8Ptr(key string, val *uint8) Field {
	if val == nil {
		return Nil(key)
	}
	return Uint8(key, *val)
}

// Uint16 constructs a field that carries an uint16.
func Uint16(key string, val uint16) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint16Ptr constructs a field that carries a *uint16.
func Uint16Ptr(key string, val *uint16) Field {
	if val == nil {
		return Nil(key)
	}
	return Uint16(key, *val)
}

// Uint32 constructs a field that carries an uint32.
func Uint32(key string, val uint32) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint32Ptr constructs a field that carries a *uint32.
func Uint32Ptr(key string, val *uint32) Field {
	if val == nil {
		return Nil(key)
	}
	return Uint32(key, *val)
}

// Uint64 constructs a field that carries an uint64.
func Uint64(key string, val uint64) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint64Ptr constructs a field that carries a *uint64.
func Uint64Ptr(key string, val *uint64) Field {
	if val == nil {
		return Nil(key)
	}
	return Uint64(key, *val)
}

// Float32 constructs a field that carries a float32.
func Float32(key string, val float32) Field {
	return Field{Key: key, Val: Float64Value(val)}
}

// Float32Ptr constructs a field that carries a *float32.
func Float32Ptr(key string, val *float32) Field {
	if val == nil {
		return Nil(key)
	}
	return Float32(key, *val)
}

// Float64 constructs a field that carries a float64.
func Float64(key string, val float64) Field {
	return Field{Key: key, Val: Float64Value(val)}
}

// Float64Ptr constructs a field that carries a *float64.
func Float64Ptr(key string, val *float64) Field {
	if val == nil {
		return Nil(key)
	}
	return Float64(key, *val)
}

// String constructs a field that carries a string.
func String(key string, val string) Field {
	return Field{Key: key, Val: StringValue(val)}
}

// StringPtr constructs a field that carries a *string.
func StringPtr(key string, val *string) Field {
	if val == nil {
		return Nil(key)
	}
	return String(key, *val)
}

// Reflect constructs a field that carries an interface{},
// which should be serialized using reflection.
func Reflect(key string, val interface{}) Field {
	return Field{Key: key, Val: ReflectValue{Val: val}}
}

// Array constructs a field that carries a slice of Value.
func Array(key string, val ...Value) Field {
	return Field{Key: key, Val: ArrayValue(val)}
}

// Object constructs a field that carries a slice of Field.
func Object(key string, fields ...Field) Field {
	return Field{Key: key, Val: ObjectValue(fields)}
}

// Bools constructs a field that carries a slice of bool.
func Bools(key string, val []bool) Field {
	return Field{Key: key, Val: BoolsValue(val)}
}

// Ints constructs a field that carries a slice of int.
func Ints(key string, val []int) Field {
	return Field{Key: key, Val: IntsValue(val)}
}

// Int8s constructs a field that carries a slice of int8.
func Int8s(key string, val []int8) Field {
	return Field{Key: key, Val: Int8sValue(val)}
}

// Int16s constructs a field that carries a slice of int16.
func Int16s(key string, val []int16) Field {
	return Field{Key: key, Val: Int16sValue(val)}
}

// Int32s constructs a field that carries a slice of int32.
func Int32s(key string, val []int32) Field {
	return Field{Key: key, Val: Int32sValue(val)}
}

// Int64s constructs a field that carries a slice of int64.
func Int64s(key string, val []int64) Field {
	return Field{Key: key, Val: Int64sValue(val)}
}

// Uints constructs a field that carries a slice of uint.
func Uints(key string, val []uint) Field {
	return Field{Key: key, Val: UintsValue(val)}
}

// Uint8s constructs a field that carries a slice of uint8.
func Uint8s(key string, val []uint8) Field {
	return Field{Key: key, Val: Uint8sValue(val)}
}

// Uint16s constructs a field that carries a slice of uint16.
func Uint16s(key string, val []uint16) Field {
	return Field{Key: key, Val: Uint16sValue(val)}
}

// Uint32s constructs a field that carries a slice of uint32.
func Uint32s(key string, val []uint32) Field {
	return Field{Key: key, Val: Uint32sValue(val)}
}

// Uint64s constructs a field that carries a slice of uint64.
func Uint64s(key string, val []uint64) Field {
	return Field{Key: key, Val: Uint64sValue(val)}
}

// Float32s constructs a field that carries a slice of float32.
func Float32s(key string, val []float32) Field {
	return Field{Key: key, Val: Float32sValue(val)}
}

// Float64s constructs a field that carries a slice of float64.
func Float64s(key string, val []float64) Field {
	return Field{Key: key, Val: Float64sValue(val)}
}

// Strings constructs a field that carries a slice of string.
func Strings(key string, val []string) Field {
	return Field{Key: key, Val: StringsValue(val)}
}

// Any takes a key and an arbitrary value and chooses the best way
// to represent them as a field, falling back to a reflection-based
// approach only if necessary.
func Any(key string, value interface{}) Field {
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
		return Int8(key, val)
	case *int8:
		return Int8Ptr(key, val)
	case []int8:
		return Int8s(key, val)
	case int16:
		return Int16(key, val)
	case *int16:
		return Int16Ptr(key, val)
	case []int16:
		return Int16s(key, val)
	case int32:
		return Int32(key, val)
	case *int32:
		return Int32Ptr(key, val)
	case []int32:
		return Int32s(key, val)
	case int64:
		return Int64(key, val)
	case *int64:
		return Int64Ptr(key, val)
	case []int64:
		return Int64s(key, val)
	case uint:
		return Uint(key, val)
	case *uint:
		return UintPtr(key, val)
	case []uint:
		return Uints(key, val)
	case uint8:
		return Uint8(key, val)
	case *uint8:
		return Uint8Ptr(key, val)
	case []uint8:
		return Uint8s(key, val)
	case uint16:
		return Uint16(key, val)
	case *uint16:
		return Uint16Ptr(key, val)
	case []uint16:
		return Uint16s(key, val)
	case uint32:
		return Uint32(key, val)
	case *uint32:
		return Uint32Ptr(key, val)
	case []uint32:
		return Uint32s(key, val)
	case uint64:
		return Uint64(key, val)
	case *uint64:
		return Uint64Ptr(key, val)
	case []uint64:
		return Uint64s(key, val)
	case float32:
		return Float32(key, val)
	case *float32:
		return Float32Ptr(key, val)
	case []float32:
		return Float32s(key, val)
	case float64:
		return Float64(key, val)
	case *float64:
		return Float64Ptr(key, val)
	case []float64:
		return Float64s(key, val)
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
