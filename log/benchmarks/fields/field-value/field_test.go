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
	"bytes"
	"fmt"
	"testing"

	"benchmark-fields/encoder"
)

type bools []bool

func (arr bools) EncodeArray(enc encoder.Encoder) {
	for _, v := range arr {
		enc.AppendBool(v)
	}
}

type int64s []int64

func (arr int64s) EncodeArray(enc encoder.Encoder) {
	for _, v := range arr {
		enc.AppendInt64(v)
	}
}

type float64s []float64

func (arr float64s) EncodeArray(enc encoder.Encoder) {
	for _, v := range arr {
		enc.AppendFloat64(v)
	}
}

type strings []string

func (arr strings) EncodeArray(enc encoder.Encoder) {
	for _, v := range arr {
		enc.AppendString(v)
	}
}

func BenchmarkFieldValue(b *testing.B) {

	// bools-8                      10706354	123.8 ns/op	  152 B/op	  4 allocs/op
	// bools_as_ArrayValue-8        11192071	107.9 ns/op	  152 B/op	  4 allocs/op
	// int64s-8                      9132631	131.0 ns/op	  152 B/op	  4 allocs/op
	// int64s_as_ArrayValue-8        9038806	131.8 ns/op	  152 B/op	  4 allocs/op
	// float64s-8                    2022433	597.8 ns/op	  344 B/op	 12 allocs/op
	// float64s_as_ArrayValue-8      1988160	602.1 ns/op	  344 B/op	 12 allocs/op
	// strings-8                     8134866	145.5 ns/op	  152 B/op	  4 allocs/op
	// strings_as_ArrayValue-8       8170756	165.6 ns/op	  152 B/op	  4 allocs/op

	arrBools := []bool{true, false, true, false, true, false}
	arrInt64s := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	arrFloat64s := []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}
	arrStrings := []string{"a", "b", "c", "d", "e", "f", "g", "h"}

	b1 := bytes.NewBuffer(nil)
	b2 := bytes.NewBuffer(nil)
	b3 := bytes.NewBuffer(nil)
	b4 := bytes.NewBuffer(nil)
	b5 := bytes.NewBuffer(nil)
	b6 := bytes.NewBuffer(nil)
	b7 := bytes.NewBuffer(nil)
	b8 := bytes.NewBuffer(nil)

	{
		v := Bools("arr", arrBools)
		v.Encode(encoder.NewJSONEncoder(b1))
	}

	{
		v := Array("arr", bools(arrBools))
		v.Encode(encoder.NewJSONEncoder(b2))
	}

	{
		v := Int64s("arr", arrInt64s)
		v.Encode(encoder.NewJSONEncoder(b3))
	}

	{
		v := Array("arr", int64s(arrInt64s))
		v.Encode(encoder.NewJSONEncoder(b4))
	}

	{
		v := Float64s("arr", arrFloat64s)
		v.Encode(encoder.NewJSONEncoder(b5))
	}

	{
		v := Array("arr", float64s(arrFloat64s))
		v.Encode(encoder.NewJSONEncoder(b6))
	}

	{
		v := Strings("arr", arrStrings)
		v.Encode(encoder.NewJSONEncoder(b7))
	}

	{
		v := Array("arr", strings(arrStrings))
		v.Encode(encoder.NewJSONEncoder(b8))
	}

	fmt.Println(b1.String())
	fmt.Println(b2.String())
	fmt.Println(b3.String())
	fmt.Println(b4.String())
	fmt.Println(b5.String())
	fmt.Println(b6.String())
	fmt.Println(b7.String())
	fmt.Println(b8.String())

	b.Run("bools", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Bools("arr", arrBools)
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})

	b.Run("bools as ArrayValue", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Array("arr", bools(arrBools))
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})

	b.Run("int64s", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Int64s("arr", arrInt64s)
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})

	b.Run("int64s as ArrayValue", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Array("arr", int64s(arrInt64s))
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})

	b.Run("float64s", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Float64s("arr", arrFloat64s)
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})

	b.Run("float64s as ArrayValue", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Array("arr", float64s(arrFloat64s))
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})

	b.Run("strings", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Strings("arr", arrStrings)
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})

	b.Run("strings as ArrayValue", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Array("arr", strings(arrStrings))
			v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
		}
	})
}
