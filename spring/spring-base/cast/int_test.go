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
	"strconv"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func BenchmarkToInt(b *testing.B) {
	// string/strconv-8    81830738	13.7 ns/op
	// string/go-spring-8  26871295 44.7 ns/op
	b.Run("string", func(b *testing.B) {
		v := "10"
		b.Run("strconv", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := strconv.ParseInt(v, 0, 0)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToInt64E(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func TestToInt(t *testing.T) {

	assert.Equal(t, cast.ToInt(nil), int(0))

	assert.Equal(t, cast.ToInt(int(3)), int(3))
	assert.Equal(t, cast.ToInt(int8(3)), int(3))
	assert.Equal(t, cast.ToInt(int16(3)), int(3))
	assert.Equal(t, cast.ToInt(int32(3)), int(3))
	assert.Equal(t, cast.ToInt(int64(3)), int(3))
	assert.Equal(t, cast.ToInt(cast.IntPtr(3)), int(3))
	assert.Equal(t, cast.ToInt(cast.Int8Ptr(3)), int(3))
	assert.Equal(t, cast.ToInt(cast.Int16Ptr(3)), int(3))
	assert.Equal(t, cast.ToInt(cast.Int32Ptr(3)), int(3))
	assert.Equal(t, cast.ToInt(cast.Int64Ptr(3)), int(3))

	assert.Equal(t, cast.ToInt8(uint(3)), int8(3))
	assert.Equal(t, cast.ToInt8(uint8(3)), int8(3))
	assert.Equal(t, cast.ToInt8(uint16(3)), int8(3))
	assert.Equal(t, cast.ToInt8(uint32(3)), int8(3))
	assert.Equal(t, cast.ToInt8(uint64(3)), int8(3))
	assert.Equal(t, cast.ToInt8(cast.IntPtr(3)), int8(3))
	assert.Equal(t, cast.ToInt8(cast.Int8Ptr(3)), int8(3))
	assert.Equal(t, cast.ToInt8(cast.Int16Ptr(3)), int8(3))
	assert.Equal(t, cast.ToInt8(cast.Int32Ptr(3)), int8(3))
	assert.Equal(t, cast.ToInt8(cast.Int64Ptr(3)), int8(3))

	assert.Equal(t, cast.ToInt16(float32(3)), int16(3))
	assert.Equal(t, cast.ToInt16(float64(3)), int16(3))
	assert.Equal(t, cast.ToInt16(cast.Float32Ptr(3)), int16(3))
	assert.Equal(t, cast.ToInt16(cast.Float64Ptr(3)), int16(3))

	assert.Equal(t, cast.ToInt32("3"), int32(3))
	assert.Equal(t, cast.ToInt32(cast.StringPtr("3")), int32(3))

	assert.Equal(t, cast.ToInt64(true), int64(1))
	assert.Equal(t, cast.ToInt64(false), int64(0))
	assert.Equal(t, cast.ToInt64(cast.BoolPtr(true)), int64(1))
	assert.Equal(t, cast.ToInt64(cast.BoolPtr(false)), int64(0))

	_, err := cast.ToInt64E("abc")
	assert.Error(t, err, "strconv.ParseInt: parsing \"abc\": invalid syntax")
}
