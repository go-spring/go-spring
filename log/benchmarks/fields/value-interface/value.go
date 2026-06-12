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

// Value is an interface for types that can encode themselves using an Encoder.
type Value interface {
	Encode(enc encoder.Encoder)
}

// BoolValue represents a bool carried by Field.
type BoolValue bool

// Encode encodes the data represented by v to an Encoder.
func (v BoolValue) Encode(enc encoder.Encoder) {
	enc.AppendBool(bool(v))
}

// Int64Value represents an int64 carried by Field.
type Int64Value int64

// Encode encodes the data represented by v to an Encoder.
func (v Int64Value) Encode(enc encoder.Encoder) {
	enc.AppendInt64(int64(v))
}

// Float64Value represents a float64 carried by Field.
type Float64Value float64

// Encode encodes the data represented by v to an Encoder.
func (v Float64Value) Encode(enc encoder.Encoder) {
	enc.AppendFloat64(float64(v))
}

// StringValue represents a string carried by Field.
type StringValue string

// Encode encodes the data represented by v to an Encoder.
func (v StringValue) Encode(enc encoder.Encoder) {
	enc.AppendString(string(v))
}

// ReflectValue represents an interface{} carried by Field.
type ReflectValue struct {
	Val interface{}
}

// Encode encodes the data represented by v to an Encoder.
func (v ReflectValue) Encode(enc encoder.Encoder) {
	enc.AppendReflect(v.Val)
}

// BoolsValue represents a slice of bool carried by Field.
type BoolsValue []bool

// Encode encodes the data represented by v to an Encoder.
func (v BoolsValue) Encode(enc encoder.Encoder) {
	enc.AppendArrayBegin()
	for _, val := range v {
		enc.AppendBool(val)
	}
	enc.AppendArrayEnd()
}

// Int64sValue represents a slice of int64 carried by Field.
type Int64sValue []int64

// Encode encodes the data represented by v to an Encoder.
func (v Int64sValue) Encode(enc encoder.Encoder) {
	enc.AppendArrayBegin()
	for _, val := range v {
		enc.AppendInt64(val)
	}
	enc.AppendArrayEnd()
}

// Float64sValue represents a slice of float64 carried by Field.
type Float64sValue []float64

// Encode encodes the data represented by v to an Encoder.
func (v Float64sValue) Encode(enc encoder.Encoder) {
	enc.AppendArrayBegin()
	for _, val := range v {
		enc.AppendFloat64(val)
	}
	enc.AppendArrayEnd()
}

// StringsValue represents a slice of string carried by Field.
type StringsValue []string

// Encode encodes the data represented by v to an Encoder.
func (v StringsValue) Encode(enc encoder.Encoder) {
	enc.AppendArrayBegin()
	for _, val := range v {
		enc.AppendString(val)
	}
	enc.AppendArrayEnd()
}

// ArrayValue represents a slice of Value carried by Field.
type ArrayValue []Value

// Encode encodes the data represented by v to an Encoder.
func (v ArrayValue) Encode(enc encoder.Encoder) {
	enc.AppendArrayBegin()
	for _, val := range v {
		val.Encode(enc)
	}
	enc.AppendArrayEnd()
}

// ObjectValue represents a slice of Field carried by Field.
type ObjectValue []Field

// Encode encodes the data represented by v to an Encoder.
func (v ObjectValue) Encode(enc encoder.Encoder) {
	enc.AppendObjectBegin()
	for _, f := range v {
		f.Encode(enc)
	}
	enc.AppendObjectEnd()
}
