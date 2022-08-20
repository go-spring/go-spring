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

type Value interface {
	Encode(enc Encoder) error
}

type Field struct {
	Key string
	Val Value
}

func nilField(key string) Field {
	return Reflect(key, nil)
}

func Object(key string, val map[string]Value) Field {
	return Field{Key: key, Val: ObjectValue(val)}
}

func Array(key string, val ...Value) Field {
	return Field{Key: key, Val: ArrayValue(val)}
}

// Bool constructs a field that carries a bool.
func Bool(key string, val bool) Field {
	return Field{Key: key, Val: BoolValue(val)}
}

// BoolPtr constructs a field that carries a *bool. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func BoolPtr(key string, val *bool) Field {
	if val == nil {
		return nilField(key)
	}
	return Bool(key, *val)
}

// Int constructs a field with the given key and value.
func Int(key string, val int) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// IntPtr constructs a field that carries a *int. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func IntPtr(key string, val *int) Field {
	if val == nil {
		return nilField(key)
	}
	return Int(key, *val)
}

// Int8 constructs a field with the given key and value.
func Int8(key string, val int8) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int8Ptr constructs a field that carries a *int8. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int8Ptr(key string, val *int8) Field {
	if val == nil {
		return nilField(key)
	}
	return Int8(key, *val)
}

// Int16 constructs a field with the given key and value.
func Int16(key string, val int16) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int16Ptr constructs a field that carries a *int16. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int16Ptr(key string, val *int16) Field {
	if val == nil {
		return nilField(key)
	}
	return Int16(key, *val)
}

// Int32 constructs a field with the given key and value.
func Int32(key string, val int32) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int32Ptr constructs a field that carries a *int32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int32Ptr(key string, val *int32) Field {
	if val == nil {
		return nilField(key)
	}
	return Int32(key, *val)
}

// Int64 constructs a field with the given key and value.
func Int64(key string, val int64) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int64Ptr constructs a field that carries a *int64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int64Ptr(key string, val *int64) Field {
	if val == nil {
		return nilField(key)
	}
	return Int64(key, *val)
}

// Uint constructs a field with the given key and value.
func Uint(key string, val uint) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// UintPtr constructs a field that carries a *uint. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func UintPtr(key string, val *uint) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint(key, *val)
}

// Uint8 constructs a field with the given key and value.
func Uint8(key string, val uint8) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint8Ptr constructs a field that carries a *uint8. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint8Ptr(key string, val *uint8) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint8(key, *val)
}

// Uint16 constructs a field with the given key and value.
func Uint16(key string, val uint16) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint16Ptr constructs a field that carries a *uint16. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint16Ptr(key string, val *uint16) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint16(key, *val)
}

// Uint32 constructs a field with the given key and value.
func Uint32(key string, val uint32) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint32Ptr constructs a field that carries a *uint32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint32Ptr(key string, val *uint32) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint32(key, *val)
}

// Uint64 constructs a field with the given key and value.
func Uint64(key string, val uint64) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint64Ptr constructs a field that carries a *uint64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint64Ptr(key string, val *uint64) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint64(key, *val)
}

// Float32 constructs a field that carries a float32. The way the
// floating-point value is represented is encoder-dependent, so marshaling is
// necessarily lazy.
func Float32(key string, val float32) Field {
	return Field{Key: key, Val: Float64Value(val)}
}

// Float32Ptr constructs a field that carries a *float32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Float32Ptr(key string, val *float32) Field {
	if val == nil {
		return nilField(key)
	}
	return Float32(key, *val)
}

// Float64 constructs a field that carries a float64. The way the
// floating-point value is represented is encoder-dependent, so marshaling is
// necessarily lazy.
func Float64(key string, val float64) Field {
	return Field{Key: key, Val: Float64Value(val)}
}

// Float64Ptr constructs a field that carries a *float64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Float64Ptr(key string, val *float64) Field {
	if val == nil {
		return nilField(key)
	}
	return Float64(key, *val)
}

// String constructs a field with the given key and value.
func String(key string, val string) Field {
	return Field{Key: key, Val: StringValue(val)}
}

// StringPtr constructs a field that carries a *string. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func StringPtr(key string, val *string) Field {
	if val == nil {
		return nilField(key)
	}
	return String(key, *val)
}

// Reflect constructs a field with the given key and an arbitrary object.
func Reflect(key string, val interface{}) Field {
	return Field{Key: key, Val: ReflectValue{Val: val}}
}

type ObjectValue map[string]Value

func (v ObjectValue) Encode(enc Encoder) error {
	err := enc.AppendObjectBegin()
	if err != nil {
		return err
	}
	for key, val := range v {
		err = enc.AppendKey(key)
		if err != nil {
			return err
		}
		err = val.Encode(enc)
		if err != nil {
			return err
		}
	}
	return enc.AppendObjectEnd()
}

type ArrayValue []Value

func (v ArrayValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = val.Encode(enc)
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type BoolValue bool

func (v BoolValue) Encode(enc Encoder) error {
	return enc.AppendBool(bool(v))
}

type Int64Value int64

func (v Int64Value) Encode(enc Encoder) error {
	return enc.AppendInt64(int64(v))
}

type Uint64Value uint64

func (v Uint64Value) Encode(enc Encoder) error {
	return enc.AppendUint64(uint64(v))
}

type Float64Value float64

func (v Float64Value) Encode(enc Encoder) error {
	return enc.AppendFloat64(float64(v))
}

type StringValue string

func (v StringValue) Encode(enc Encoder) error {
	return enc.AppendString(string(v))
}

type ReflectValue struct {
	Val interface{}
}

func (v ReflectValue) Encode(enc Encoder) error {
	return enc.AppendReflect(v.Val)
}
