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

package main_test

import (
	"bytes"
	"testing"

	"benchmark-fields/encoder"
	"benchmark-fields/field-value"
	"benchmark-fields/value-interface"
	"benchmark-fields/value-struct"
)

var (
	arrBools    = []bool{true, false, true, false, true, false}
	arrInt64s   = []int64{1, 2, 3, 4, 5, 6, 7, 8}
	arrFloat64s = []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}
	arrStrings  = []string{"a", "b", "c", "d", "e", "f", "g", "h"}
)

func BenchmarkFields(b *testing.B) {

	// value_interface/bools-8      10864762	109.4 ns/op	  152 B/op	  4 allocs/op
	// value_struct/bools-8         11207328	107.6 ns/op	  152 B/op	  4 allocs/op
	// field_value/bools-8          11133696	108.8 ns/op	  152 B/op	  4 allocs/op

	// value_interface/int64s-8      8831475	138.4 ns/op	  152 B/op	  4 allocs/op
	// value_struct/int64s-8         8929010	134.1 ns/op	  152 B/op	  4 allocs/op
	// field_value/int64s-8          8978941	132.0 ns/op	  152 B/op	  4 allocs/op

	// value_interface/float64s-8    1927635	614.1 ns/op	  344 B/op	 12 allocs/op
	// value_struct/float64s-8       1980488	604.1 ns/op	  344 B/op	 12 allocs/op
	// field_value/float64s-8        1992417	601.2 ns/op	  344 B/op	 12 allocs/op

	// value_interface/strings-8     8276900	144.9 ns/op	  152 B/op	  4 allocs/op
	// value_struct/strings-8        8107906	148.5 ns/op	  152 B/op	  4 allocs/op
	// field_value/strings-8         8212352	149.6 ns/op	  152 B/op	  4 allocs/op

	b.Run("value_interface", func(b *testing.B) {
		b.Run("bools", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_interface.Bools("arr", arrBools)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("int64s", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_interface.Int64s("arr", arrInt64s)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("float64s", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_interface.Float64s("arr", arrFloat64s)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("strings", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_interface.Strings("arr", arrStrings)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
	})

	b.Run("value_struct", func(b *testing.B) {
		b.Run("bools", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_struct.Bools("arr", arrBools)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("int64s", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_struct.Int64s("arr", arrInt64s)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("float64s", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_struct.Float64s("arr", arrFloat64s)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("strings", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := value_struct.Strings("arr", arrStrings)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
	})

	b.Run("field_value", func(b *testing.B) {
		b.Run("bools", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := field_value.Bools("arr", arrBools)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("int64s", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := field_value.Int64s("arr", arrInt64s)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("float64s", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := field_value.Float64s("arr", arrFloat64s)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
		b.Run("strings", func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for b.Loop() {
				v := field_value.Strings("arr", arrStrings)
				v.Encode(encoder.NewJSONEncoder(bytes.NewBuffer(nil)))
			}
		})
	})
}
