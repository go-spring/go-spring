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

import "fmt"

type Value interface {
	Encode(enc Encoder) error
}

type Field struct {
	Key string
	Val Value
}

func Array(key string, val ...Value) Field {
	return Field{Key: key, Val: ArrayValue(val)}
}

func Object(key string, fields ...Field) Field {
	return Field{Key: key, Val: ObjectValue(fields)}
}

func nilField(key string) Field {
	return Reflect(key, nil)
}

// Bool constructs a field that carries a bool.
func Bool(key string, val bool) Field {
	return Field{Key: key, Val: BoolValue(val)}
}

// Boolp constructs a field that carries a *bool. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Boolp(key string, val *bool) Field {
	if val == nil {
		return nilField(key)
	}
	return Bool(key, *val)
}

// Int constructs a field with the given key and value.
func Int(key string, val int) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Intp constructs a field that carries a *int. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Intp(key string, val *int) Field {
	if val == nil {
		return nilField(key)
	}
	return Int(key, *val)
}

// Int8 constructs a field with the given key and value.
func Int8(key string, val int8) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int8p constructs a field that carries a *int8. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int8p(key string, val *int8) Field {
	if val == nil {
		return nilField(key)
	}
	return Int8(key, *val)
}

// Int16 constructs a field with the given key and value.
func Int16(key string, val int16) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int16p constructs a field that carries a *int16. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int16p(key string, val *int16) Field {
	if val == nil {
		return nilField(key)
	}
	return Int16(key, *val)
}

// Int32 constructs a field with the given key and value.
func Int32(key string, val int32) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int32p constructs a field that carries a *int32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int32p(key string, val *int32) Field {
	if val == nil {
		return nilField(key)
	}
	return Int32(key, *val)
}

// Int64 constructs a field with the given key and value.
func Int64(key string, val int64) Field {
	return Field{Key: key, Val: Int64Value(val)}
}

// Int64p constructs a field that carries a *int64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Int64p(key string, val *int64) Field {
	if val == nil {
		return nilField(key)
	}
	return Int64(key, *val)
}

// Uint constructs a field with the given key and value.
func Uint(key string, val uint) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uintp constructs a field that carries a *uint. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uintp(key string, val *uint) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint(key, *val)
}

// Uint8 constructs a field with the given key and value.
func Uint8(key string, val uint8) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint8p constructs a field that carries a *uint8. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint8p(key string, val *uint8) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint8(key, *val)
}

// Uint16 constructs a field with the given key and value.
func Uint16(key string, val uint16) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint16p constructs a field that carries a *uint16. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint16p(key string, val *uint16) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint16(key, *val)
}

// Uint32 constructs a field with the given key and value.
func Uint32(key string, val uint32) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint32p constructs a field that carries a *uint32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint32p(key string, val *uint32) Field {
	if val == nil {
		return nilField(key)
	}
	return Uint32(key, *val)
}

// Uint64 constructs a field with the given key and value.
func Uint64(key string, val uint64) Field {
	return Field{Key: key, Val: Uint64Value(val)}
}

// Uint64p constructs a field that carries a *uint64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Uint64p(key string, val *uint64) Field {
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

// Float32p constructs a field that carries a *float32. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Float32p(key string, val *float32) Field {
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

// Float64p constructs a field that carries a *float64. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Float64p(key string, val *float64) Field {
	if val == nil {
		return nilField(key)
	}
	return Float64(key, *val)
}

// String constructs a field with the given key and value.
func String(key string, val string) Field {
	return Field{Key: key, Val: StringValue(val)}
}

// Stringp constructs a field that carries a *string. The returned Field will safely
// and explicitly represent `nil` when appropriate.
func Stringp(key string, val *string) Field {
	if val == nil {
		return nilField(key)
	}
	return String(key, *val)
}

// Reflect constructs a field with the given key and an arbitrary object.
func Reflect(key string, val interface{}) Field {
	return Field{Key: key, Val: ReflectValue{Val: val}}
}

type ObjectValue []Field

func (v ObjectValue) Encode(enc Encoder) error {
	err := enc.AppendObjectBegin()
	if err != nil {
		return err
	}
	for _, f := range v {
		err = enc.AppendKey(f.Key)
		if err != nil {
			return err
		}
		err = f.Val.Encode(enc)
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

func Bools(key string, val []bool) Field {
	return Field{Key: key, Val: BoolsValue(val)}
}

func Ints(key string, val []int) Field {
	return Field{Key: key, Val: IntsValue(val)}
}

func Int8s(key string, val []int8) Field {
	return Field{Key: key, Val: Int8sValue(val)}
}

func Int16s(key string, val []int16) Field {
	return Field{Key: key, Val: Int16sValue(val)}
}

func Int32s(key string, val []int32) Field {
	return Field{Key: key, Val: Int32sValue(val)}
}

func Int64s(key string, val []int64) Field {
	return Field{Key: key, Val: Int64sValue(val)}
}

func Uints(key string, val []uint) Field {
	return Field{Key: key, Val: UintsValue(val)}
}

func Uint8s(key string, val []uint8) Field {
	return Field{Key: key, Val: Uint8sValue(val)}
}

func Uint16s(key string, val []uint16) Field {
	return Field{Key: key, Val: Uint16sValue(val)}
}

func Uint32s(key string, val []uint32) Field {
	return Field{Key: key, Val: Uint32sValue(val)}
}

func Uint64s(key string, val []uint64) Field {
	return Field{Key: key, Val: Uint64sValue(val)}
}

func Float32s(key string, val []float32) Field {
	return Field{Key: key, Val: Float32sValue(val)}
}

func Float64s(key string, val []float64) Field {
	return Field{Key: key, Val: Float64sValue(val)}
}

func Strings(key string, val []string) Field {
	return Field{Key: key, Val: StringsValue(val)}
}

type BoolsValue []bool

func (v BoolsValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendBool(val)
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type IntsValue []int

func (v IntsValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendInt64(int64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Int8sValue []int8

func (v Int8sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendInt64(int64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Int16sValue []int16

func (v Int16sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendInt64(int64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Int32sValue []int32

func (v Int32sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendInt64(int64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Int64sValue []int64

func (v Int64sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendInt64(val)
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type UintsValue []uint

func (v UintsValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendUint64(uint64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Uint8sValue []uint8

func (v Uint8sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendUint64(uint64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Uint16sValue []uint16

func (v Uint16sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendUint64(uint64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Uint32sValue []uint32

func (v Uint32sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendUint64(uint64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Uint64sValue []uint64

func (v Uint64sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendUint64(val)
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Float32sValue []float32

func (v Float32sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendFloat64(float64(val))
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type Float64sValue []float64

func (v Float64sValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendFloat64(val)
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

type StringsValue []string

func (v StringsValue) Encode(enc Encoder) error {
	err := enc.AppendArrayBegin()
	if err != nil {
		return err
	}
	for _, val := range v {
		err = enc.AppendString(val)
		if err != nil {
			return err
		}
	}
	return enc.AppendArrayEnd()
}

func Any(key string, value interface{}) Field {
	switch val := value.(type) {
	case nil:
		return nilField(key)
	case bool:
		return Bool(key, val)
	case *bool:
		return Boolp(key, val)
	case []bool:
		return Bools(key, val)
	case int:
		return Int(key, val)
	case *int:
		return Intp(key, val)
	case []int:
		return Ints(key, val)
	case int8:
		return Int8(key, val)
	case *int8:
		return Int8p(key, val)
	case []int8:
		return Int8s(key, val)
	case int16:
		return Int16(key, val)
	case *int16:
		return Int16p(key, val)
	case []int16:
		return Int16s(key, val)
	case int32:
		return Int32(key, val)
	case *int32:
		return Int32p(key, val)
	case []int32:
		return Int32s(key, val)
	case int64:
		return Int64(key, val)
	case *int64:
		return Int64p(key, val)
	case []int64:
		return Int64s(key, val)
	case uint:
		return Uint(key, val)
	case *uint:
		return Uintp(key, val)
	case []uint:
		return Uints(key, val)
	case uint8:
		return Uint8(key, val)
	case *uint8:
		return Uint8p(key, val)
	case []uint8:
		return Uint8s(key, val)
	case uint16:
		return Uint16(key, val)
	case *uint16:
		return Uint16p(key, val)
	case []uint16:
		return Uint16s(key, val)
	case uint32:
		return Uint32(key, val)
	case *uint32:
		return Uint32p(key, val)
	case []uint32:
		return Uint32s(key, val)
	case uint64:
		return Uint64(key, val)
	case *uint64:
		return Uint64p(key, val)
	case []uint64:
		return Uint64s(key, val)
	case float32:
		return Float32(key, val)
	case *float32:
		return Float32p(key, val)
	case []float32:
		return Float32s(key, val)
	case float64:
		return Float64(key, val)
	case *float64:
		return Float64p(key, val)
	case []float64:
		return Float64s(key, val)
	case string:
		return String(key, val)
	case *string:
		return Stringp(key, val)
	case []string:
		return Strings(key, val)
	default:
		return Reflect(key, val)
	}
}

func Message(format string, args ...interface{}) Field {
	return Field{
		Key: "msg",
		Val: MessageValue{format: format, args: args},
	}
}

type MessageValue struct {
	format string
	args   []interface{}
}

func (v MessageValue) Encode(enc Encoder) error {
	if len(v.args) == 1 {
		fn, ok := v.args[0].(func() []interface{})
		if ok {
			v.args = fn()
		}
	}
	var text string
	if len(v.args) == 0 {
		text = v.format
	} else {
		if v.format == "" {
			text = fmt.Sprint(v.args...)
		} else {
			text = fmt.Sprintf(v.format, v.args...)
		}
	}
	return enc.AppendString(text)
}
