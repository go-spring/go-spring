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
	"bytes"
	"testing"

	"benchmark-fields/encoder"
)

func BenchmarkValueInterface(b *testing.B) {

	// bools-8      10998112	105.1 ns/op	  152 B/op	  4 allocs/op
	// int64s-8      9017383	131.5 ns/op	  152 B/op	  4 allocs/op
	// float64s-8    1904684	634.8 ns/op	  344 B/op	 12 allocs/op
	// strings-8     8188070	145.1 ns/op	  152 B/op	  4 allocs/op

	arrBools := []bool{true, false, true, false, true, false}
	arrInt64s := []int64{1, 2, 3, 4, 5, 6, 7, 8}
	arrFloat64s := []float64{1.1, 2.2, 3.3, 4.4, 5.5, 6.6, 7.7, 8.8}
	arrStrings := []string{"a", "b", "c", "d", "e", "f", "g", "h"}

	b.Run("bools", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Bools("arr", arrBools)
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

	b.Run("float64s", func(b *testing.B) {
		b.ResetTimer()
		b.ReportAllocs()

		for b.Loop() {
			v := Float64s("arr", arrFloat64s)
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
}
