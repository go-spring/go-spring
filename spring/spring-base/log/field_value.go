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

type funcValue func() []Field

func (v funcValue) Encode(enc Encoder) error {
	return nil
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
