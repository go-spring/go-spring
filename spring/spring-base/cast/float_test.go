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

package cast_test

import (
	"errors"
	"strconv"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func BenchmarkToFloat(b *testing.B) {
	// string/strconv-8    59966035 20.0 ns/op
	// string/go-spring-8  22259067 47.3 ns/op
	b.Run("string", func(b *testing.B) {
		v := "10"
		b.Run("strconv", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := strconv.ParseFloat(v, 64)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToFloat64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func TestToFloat(t *testing.T) {

	assert.Equal(t, cast.ToFloat32(nil), float32(0))

	assert.Equal(t, cast.ToFloat32(int(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(int8(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(int16(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(int32(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(int64(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.IntPtr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Int8Ptr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Int16Ptr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Int32Ptr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Int64Ptr(3)), float32(3))

	assert.Equal(t, cast.ToFloat32(uint(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(uint8(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(uint16(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(uint32(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(uint64(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.UintPtr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Uint8Ptr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Uint16Ptr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Uint32Ptr(3)), float32(3))
	assert.Equal(t, cast.ToFloat32(cast.Uint64Ptr(3)), float32(3))

	assert.Equal(t, cast.ToFloat64(float32(3)), float64(3))
	assert.Equal(t, cast.ToFloat64(float64(3)), float64(3))
	assert.Equal(t, cast.ToFloat64(cast.Float32Ptr(3)), float64(3))
	assert.Equal(t, cast.ToFloat64(cast.Float64Ptr(3)), float64(3))

	assert.Equal(t, cast.ToFloat64("3"), float64(3))
	assert.Equal(t, cast.ToFloat64(cast.StringPtr("3")), float64(3))

	assert.Equal(t, cast.ToFloat64(true), float64(1))
	assert.Equal(t, cast.ToFloat64(false), float64(0))
	assert.Equal(t, cast.ToFloat64(cast.BoolPtr(true)), float64(1))
	assert.Equal(t, cast.ToFloat64(cast.BoolPtr(false)), float64(0))

	_, err := cast.ToFloat64E("abc")
	assert.Error(t, err, "strconv.ParseFloat: parsing \"abc\": invalid syntax")

	_, err = cast.ToFloat64E(errors.New("abc"))
	assert.Error(t, err, "unable to cast type \\(\\*errors\\.errorString\\) to float64")
}
