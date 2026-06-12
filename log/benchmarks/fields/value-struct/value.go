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

package value_struct

import (
	"math"
	"unsafe"

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

type Value struct {
	Type ValueType
	Num  uint64
	Any  any
}

// BoolValue represents a bool carried by Field.
func BoolValue(v bool) Value {
	if v {
		return Value{
			Type: ValueTypeBool,
			Num:  1,
		}
	} else {
		return Value{
			Type: ValueTypeBool,
			Num:  0,
		}
	}
}

// Int64Value represents an int64 carried by Field.
func Int64Value(v int64) Value {
	return Value{
		Type: ValueTypeInt64,
		Num:  uint64(v),
	}
}

// Float64Value represents a float64 carried by Field.
func Float64Value(v float64) Value {
	return Value{
		Type: ValueTypeFloat64,
		Num:  math.Float64bits(v),
	}
}

// StringValue represents a string carried by Field.
func StringValue(v string) Value {
	return Value{
		Type: ValueTypeString,
		Num:  uint64(len(v)),
		Any:  unsafe.StringData(v),
	}
}

// ReflectValue represents an interface{} carried by Field.
func ReflectValue(v any) Value {
	return Value{
		Type: ValueTypeReflect,
		Any:  v,
	}
}

// BoolsValue represents a slice of bool carried by Field.
func BoolsValue(v []bool) Value {
	return Value{
		Type: ValueTypeBools,
		Any:  v,
	}
}

// Int64sValue represents a slice of int64 carried by Field.
func Int64sValue(v []int64) Value {
	return Value{
		Type: ValueTypeInt64s,
		Any:  v,
	}
}

// Float64sValue represents a slice of float64 carried by Field.
func Float64sValue(v []float64) Value {
	return Value{
		Type: ValueTypeFloat64s,
		Any:  v,
	}
}

// StringsValue represents a slice of string carried by Field.
func StringsValue(v []string) Value {
	return Value{
		Type: ValueTypeStrings,
		Any:  v,
	}
}

type arrVal []Value

func (v arrVal) Encode(enc encoder.Encoder) {
	enc.AppendArrayBegin()
	for _, val := range v {
		val.Encode(enc)
	}
	enc.AppendArrayEnd()
}

// ArrayValue represents a slice of Value carried by Field.
func ArrayValue(val ...Value) Value {
	return Value{
		Type: ValueTypeArray,
		Any:  arrVal(val),
	}
}

type objVal []Field

func (v objVal) Encode(enc encoder.Encoder) {
	enc.AppendObjectBegin()
	for _, f := range v {
		f.Encode(enc)
	}
	enc.AppendObjectEnd()
}

// ObjectValue represents a slice of Field carried by Field.
func ObjectValue(fields ...Field) Value {
	return Value{
		Type: ValueTypeObject,
		Any:  objVal(fields),
	}
}

// Encode encodes the Value to the encoder.
func (v Value) Encode(enc encoder.Encoder) {
	switch v.Type {
	case ValueTypeBool:
		enc.AppendBool(v.Num != 0)
	case ValueTypeInt64:
		enc.AppendInt64(int64(v.Num))
	case ValueTypeFloat64:
		enc.AppendFloat64(math.Float64frombits(v.Num))
	case ValueTypeString:
		enc.AppendString(unsafe.String(v.Any.(*byte), v.Num))
	case ValueTypeReflect:
		enc.AppendReflect(v.Any)
	case ValueTypeBools:
		enc.AppendArrayBegin()
		for _, val := range v.Any.([]bool) {
			enc.AppendBool(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeInt64s:
		enc.AppendArrayBegin()
		for _, val := range v.Any.([]int64) {
			enc.AppendInt64(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeFloat64s:
		enc.AppendArrayBegin()
		for _, val := range v.Any.([]float64) {
			enc.AppendFloat64(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeStrings:
		enc.AppendArrayBegin()
		for _, val := range v.Any.([]string) {
			enc.AppendString(val)
		}
		enc.AppendArrayEnd()
	case ValueTypeArray:
		v.Any.(arrVal).Encode(enc)
	case ValueTypeObject:
		v.Any.(objVal).Encode(enc)
	default: // for linter
	}
}
