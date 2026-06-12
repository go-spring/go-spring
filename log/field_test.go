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
	"bytes"
	"encoding/json"
	"math"
	"testing"

	"github.com/go-spring/stdlib/testing/assert"
)

var testFields = []Field{
	Msgf("hello %s", "中国"),
	Msg("hello world\n\\\t\"\r"),
	Any("null", nil),
	Any("bool", false),
	Any("bool_ptr", new(true)),
	Any("bool_ptr_nil", (*bool)(nil)),
	Any("bools", []bool{true, true, false}),
	Any("int", int(1)),
	Any("int_ptr", new(int(1))),
	Any("int_ptr_nil", (*int)(nil)),
	Any("int_slice", []int{int(1), int(2), int(3)}),
	Any("int8", int8(1)),
	Any("int8_ptr", new(int8(1))),
	Any("int8_ptr_nil", (*int8)(nil)),
	Any("int8_slice", []int8{int8(1), int8(2), int8(3)}),
	Any("int16", int16(1)),
	Any("int16_ptr", new(int16(1))),
	Any("int16_ptr_nil", (*int16)(nil)),
	Any("int16_slice", []int16{int16(1), int16(2), int16(3)}),
	Any("int32", int32(1)),
	Any("int32_ptr", new(int32(1))),
	Any("int32_ptr_nil", (*int32)(nil)),
	Any("int32_slice", []int32{int32(1), int32(2), int32(3)}),
	Any("int64", int64(1)),
	Any("int64_ptr", new(int64(1))),
	Any("int64_ptr_nil", (*int64)(nil)),
	Any("int64_slice", []int64{int64(1), int64(2), int64(3)}),
	Any("uint", uint(1)),
	Any("uint_ptr", new(uint(1))),
	Any("uint_ptr_nil", (*uint)(nil)),
	Any("uint_slice", []uint{uint(1), uint(2), uint(3)}),
	Any("uint8", uint8(1)),
	Any("uint8_ptr", new(uint8(1))),
	Any("uint8_ptr_nil", (*uint8)(nil)),
	Any("uint8_slice", []uint8{uint8(1), uint8(2), uint8(3)}),
	Any("uint16", uint16(1)),
	Any("uint16_ptr", new(uint16(1))),
	Any("uint16_ptr_nil", (*uint16)(nil)),
	Any("uint16_slice", []uint16{uint16(1), uint16(2), uint16(3)}),
	Any("uint32", uint32(1)),
	Any("uint32_ptr", new(uint32(1))),
	Any("uint32_ptr_nil", (*uint32)(nil)),
	Any("uint32_slice", []uint32{uint32(1), uint32(2), uint32(3)}),
	Any("uint64", uint64(1)),
	Any("uint64_ptr", new(uint64(1))),
	Any("uint64_ptr_nil", (*uint64)(nil)),
	Any("uint64_slice", []uint64{uint64(1), uint64(2), uint64(3)}),
	Any("float32", float32(1)),
	Any("float32_ptr", new(float32(1))),
	Any("float32_ptr_nil", (*float32)(nil)),
	Any("float32_slice", []float32{float32(1), float32(2), float32(3)}),
	Any("float64", float64(1)),
	Any("float64_ptr", new(float64(1))),
	Any("float64_ptr_nil", (*float64)(nil)),
	Any("float64_slice", []float64{float64(1), float64(2), float64(3)}),
	Any("string", "\x80\xC2\xED\xA0\x08"),
	Any("string_ptr", new("a")),
	Any("string_ptr_nil", (*string)(nil)),
	Any("string_slice", []string{"a", "b", "c"}),
	Object("object", Any("int64", int64(1)), Any("uint64", uint64(1)), Any("string", "a")),
	Any("struct", struct{ Int64 int64 }{10}),
}

func TestJSONEncoder(t *testing.T) {

	t.Run("chan error", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()
		enc.AppendKey("chan")
		enc.AppendReflect(make(chan error))
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal(`{"chan":"json: unsupported type: chan error"}`)
	})

	t.Run("success", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()
		EncodeFields(enc, testFields)
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).JSONEqual(`{
	    "msg": "hello world\n\\\t\"\r",
	    "null": null,
	    "bool": false,
	    "bool_ptr": true,
	    "bool_ptr_nil": null,
	    "bools": [
	        true,
	        true,
	        false
	    ],
	    "int": 1,
	    "int_ptr": 1,
	    "int_ptr_nil": null,
	    "int_slice": [
	        1,
	        2,
	        3
	    ],
	    "int8": 1,
	    "int8_ptr": 1,
	    "int8_ptr_nil": null,
	    "int8_slice": [
	        1,
	        2,
	        3
	    ],
	    "int16": 1,
	    "int16_ptr": 1,
	    "int16_ptr_nil": null,
	    "int16_slice": [
	        1,
	        2,
	        3
	    ],
	    "int32": 1,
	    "int32_ptr": 1,
	    "int32_ptr_nil": null,
	    "int32_slice": [
	        1,
	        2,
	        3
	    ],
	    "int64": 1,
	    "int64_ptr": 1,
	    "int64_ptr_nil": null,
	    "int64_slice": [
	        1,
	        2,
	        3
	    ],
	    "uint": 1,
	    "uint_ptr": 1,
	    "uint_ptr_nil": null,
	    "uint_slice": [
	        1,
	        2,
	        3
	    ],
	    "uint8": 1,
	    "uint8_ptr": 1,
	    "uint8_ptr_nil": null,
	    "uint8_slice": [
	        1,
	        2,
	        3
	    ],
	    "uint16": 1,
	    "uint16_ptr": 1,
	    "uint16_ptr_nil": null,
	    "uint16_slice": [
	        1,
	        2,
	        3
	    ],
	    "uint32": 1,
	    "uint32_ptr": 1,
	    "uint32_ptr_nil": null,
	    "uint32_slice": [
	        1,
	        2,
	        3
	    ],
	    "uint64": 1,
	    "uint64_ptr": 1,
	    "uint64_ptr_nil": null,
	    "uint64_slice": [
	        1,
	        2,
	        3
	    ],
	    "float32": 1,
	    "float32_ptr": 1,
	    "float32_ptr_nil": null,
	    "float32_slice": [
	        1,
	        2,
	        3
	    ],
	    "float64": 1,
	    "float64_ptr": 1,
	    "float64_ptr_nil": null,
	    "float64_slice": [
	        1,
	        2,
	        3
	    ],
	    "string": "\ufffd\ufffd\ufffd\ufffd\u0008",
	    "string_ptr": "a",
	    "string_ptr_nil": null,
	    "string_slice": [
	        "a",
	        "b",
	        "c"
	    ],
	    "object": {
	        "int64": 1,
	        "uint64": 1,
	        "string": "a"
	    },
	    "struct": {
	        "Int64": 10
	    }
	}`)
	})

	t.Run("nested objects and arrays", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()

		// Nested object with one field
		enc.AppendKey("nested_obj")
		enc.AppendObjectBegin()
		enc.AppendKey("inner_field")
		enc.AppendString("inner_value")
		enc.AppendObjectEnd()

		// Nested array with two string items
		enc.AppendKey("nested_arr")
		enc.AppendArrayBegin()
		enc.AppendString("item1")
		enc.AppendString("item2")
		enc.AppendArrayEnd()

		// Empty object
		enc.AppendKey("empty_obj")
		enc.AppendObjectBegin()
		enc.AppendObjectEnd()

		// Empty array
		enc.AppendKey("empty_arr")
		enc.AppendArrayBegin()
		enc.AppendArrayEnd()

		enc.AppendEncoderEnd()
		expected := `{"nested_obj":{"inner_field":"inner_value"},"nested_arr":["item1","item2"],"empty_obj":{},"empty_arr":[]}`
		assert.String(t, buf.String()).JSONEqual(expected)
	})

	t.Run("special characters", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()
		enc.AppendKey("special")
		enc.AppendString("line1\nline2\ttabbed\rreturn")
		enc.AppendKey("quotes")
		enc.AppendString(`quotation "mark"`)
		enc.AppendKey("backslash")
		enc.AppendString(`path\to\file`)
		enc.AppendEncoderEnd()
		expected := `{"special":"line1\nline2\ttabbed\rreturn","quotes":"quotation \"mark\"","backslash":"path\\to\\file"}`
		assert.String(t, buf.String()).JSONEqual(expected)
	})

	t.Run("numeric types", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()
		enc.AppendKey("negative_int")
		enc.AppendInt64(-42)
		enc.AppendKey("zero")
		enc.AppendInt64(0)
		enc.AppendKey("positive_int")
		enc.AppendInt64(42)
		enc.AppendKey("uint_val")
		enc.AppendUint64(42)
		enc.AppendKey("float_val")
		enc.AppendFloat64(3.14159)
		enc.AppendKey("negative_float")
		enc.AppendFloat64(-3.14159)
		enc.AppendKey("zero_float")
		enc.AppendFloat64(0.0)
		enc.AppendEncoderEnd()
		expected := `{"negative_int":-42,"zero":0,"positive_int":42,"uint_val":42,"float_val":3.14159,"negative_float":-3.14159,"zero_float":0}`
		assert.String(t, buf.String()).JSONEqual(expected)
	})

	t.Run("special float values", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()
		enc.AppendKey("nan")
		enc.AppendFloat64(math.NaN())
		enc.AppendKey("pos_inf")
		enc.AppendFloat64(math.Inf(1))
		enc.AppendKey("neg_inf")
		enc.AppendFloat64(math.Inf(-1))
		enc.AppendEncoderEnd()

		assert.That(t, json.Valid(buf.Bytes())).True()
		assert.String(t, buf.String()).JSONEqual(`{"nan":"NaN","pos_inf":"+Inf","neg_inf":"-Inf"}`)
	})

	t.Run("boolean types", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()
		enc.AppendKey("true_val")
		enc.AppendBool(true)
		enc.AppendKey("false_val")
		enc.AppendBool(false)
		enc.AppendEncoderEnd()
		expected := `{"true_val":true,"false_val":false}`
		assert.String(t, buf.String()).JSONEqual(expected)
	})

	t.Run("nil values", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()
		enc.AppendKey("nil_field")
		enc.AppendReflect(nil)
		enc.AppendEncoderEnd()
		expected := `{"nil_field":null}`
		assert.String(t, buf.String()).JSONEqual(expected)
	})

	t.Run("complex nested structure", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewJSONEncoder(buf)
		enc.AppendEncoderBegin()

		enc.AppendKey("complex")
		enc.AppendObjectBegin()
		enc.AppendKey("level1")
		enc.AppendObjectBegin()
		enc.AppendKey("level2")
		enc.AppendArrayBegin()

		// First object in array
		enc.AppendObjectBegin()
		enc.AppendKey("id")
		enc.AppendInt64(1)
		enc.AppendObjectEnd()

		// Second object in array
		enc.AppendObjectBegin()
		enc.AppendKey("id")
		enc.AppendInt64(2)
		enc.AppendObjectEnd()

		enc.AppendArrayEnd()  // close level2 array
		enc.AppendObjectEnd() // close level1 object
		enc.AppendObjectEnd() // close complex object

		enc.AppendEncoderEnd()
		expected := `{"complex":{"level1":{"level2":[{"id":1},{"id":2}]}}}`
		assert.String(t, buf.String()).JSONEqual(expected)
	})
}

func TestTextEncoder(t *testing.T) {

	t.Run("chan error", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, "||")
		enc.AppendEncoderBegin()
		enc.AppendKey("chan")
		enc.AppendReflect(make(chan error))
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal("chan=json: unsupported type: chan error")
	})

	t.Run("success", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, "||")
		enc.AppendEncoderBegin()
		EncodeFields(enc, testFields)

		// Nested object with map
		enc.AppendKey("object_2")
		enc.AppendObjectBegin()
		enc.AppendKey("map")
		enc.AppendReflect(map[string]int{"a": 1})
		enc.AppendObjectEnd()

		// Array of objects
		enc.AppendKey("array_2")
		enc.AppendArrayBegin()
		enc.AppendReflect(map[string]int{"a": 1})
		enc.AppendReflect(map[string]int{"a": 1})
		enc.AppendArrayEnd()

		enc.AppendEncoderEnd()
		const expect = `msg=hello 中国||msg=hello world\n\\\t\"\r||null=null||` +
			`bool=false||bool_ptr=true||bool_ptr_nil=null||bools=[true,true,false]||` +
			`int=1||int_ptr=1||int_ptr_nil=null||int_slice=[1,2,3]||` +
			`int8=1||int8_ptr=1||int8_ptr_nil=null||int8_slice=[1,2,3]||` +
			`int16=1||int16_ptr=1||int16_ptr_nil=null||int16_slice=[1,2,3]||` +
			`int32=1||int32_ptr=1||int32_ptr_nil=null||int32_slice=[1,2,3]||` +
			`int64=1||int64_ptr=1||int64_ptr_nil=null||int64_slice=[1,2,3]||` +
			`uint=1||uint_ptr=1||uint_ptr_nil=null||uint_slice=[1,2,3]||` +
			`uint8=1||uint8_ptr=1||uint8_ptr_nil=null||uint8_slice=[1,2,3]||` +
			`uint16=1||uint16_ptr=1||uint16_ptr_nil=null||uint16_slice=[1,2,3]||` +
			`uint32=1||uint32_ptr=1||uint32_ptr_nil=null||uint32_slice=[1,2,3]||` +
			`uint64=1||uint64_ptr=1||uint64_ptr_nil=null||uint64_slice=[1,2,3]||` +
			`float32=1||float32_ptr=1||float32_ptr_nil=null||float32_slice=[1,2,3]||` +
			`float64=1||float64_ptr=1||float64_ptr_nil=null||float64_slice=[1,2,3]||` +
			`string=\ufffd\ufffd\ufffd\ufffd\u0008||string_ptr=a||string_ptr_nil=null||string_slice=["a","b","c"]||` +
			`object={"int64":1,"uint64":1,"string":"a"}||struct={"Int64":10}||` +
			`object_2={"map":{"a":1}}||array_2=[{"a":1},{"a":1}]`
		assert.String(t, buf.String()).Equal(expect)
	})

	t.Run("nil values", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, " ")
		enc.AppendEncoderBegin()
		enc.AppendKey("nil_field")
		enc.AppendReflect(nil)
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal("nil_field=null")
	})

	t.Run("nested objects and arrays", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, " ")
		enc.AppendEncoderBegin()

		// Nested object
		enc.AppendKey("nested_obj")
		enc.AppendObjectBegin()
		enc.AppendKey("inner_field")
		enc.AppendString("inner_value")
		enc.AppendObjectEnd()

		// Nested array
		enc.AppendKey("nested_arr")
		enc.AppendArrayBegin()
		enc.AppendString("item1")
		enc.AppendString("item2")
		enc.AppendArrayEnd()

		enc.AppendEncoderEnd()
		expected := `nested_obj={"inner_field":"inner_value"} nested_arr=["item1","item2"]`
		assert.String(t, buf.String()).Equal(expected)
	})

	t.Run("different separators", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, ", ")
		enc.AppendEncoderBegin()
		enc.AppendKey("field1")
		enc.AppendString("value1")
		enc.AppendKey("field2")
		enc.AppendString("value2")
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal("field1=value1, field2=value2")
	})

	t.Run("special characters", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, " ")
		enc.AppendEncoderBegin()
		enc.AppendKey("special")
		enc.AppendString("line1\nline2\ttabbed\rreturn")
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal("special=line1\\nline2\\ttabbed\\rreturn")
	})

	t.Run("empty fields", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, " ")
		enc.AppendEncoderBegin()

		// Empty object
		enc.AppendKey("empty_obj")
		enc.AppendObjectBegin()
		enc.AppendObjectEnd()

		// Empty array
		enc.AppendKey("empty_arr")
		enc.AppendArrayBegin()
		enc.AppendArrayEnd()

		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal("empty_obj={} empty_arr=[]")
	})

	t.Run("numeric types", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, " ")
		enc.AppendEncoderBegin()
		enc.AppendKey("int_val")
		enc.AppendInt64(-42)
		enc.AppendKey("uint_val")
		enc.AppendUint64(42)
		enc.AppendKey("float_val")
		enc.AppendFloat64(3.14159)
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal("int_val=-42 uint_val=42 float_val=3.14159")
	})

	t.Run("boolean types", func(t *testing.T) {
		buf := bytes.NewBuffer(nil)
		enc := NewTextEncoder(buf, " ")
		enc.AppendEncoderBegin()
		enc.AppendKey("true_val")
		enc.AppendBool(true)
		enc.AppendKey("false_val")
		enc.AppendBool(false)
		enc.AppendEncoderEnd()
		assert.String(t, buf.String()).Equal("true_val=true false_val=false")
	})
}
