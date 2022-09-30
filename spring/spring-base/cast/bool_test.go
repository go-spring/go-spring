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

func BenchmarkToBool(b *testing.B) {
	// string/strconv-8    957624752 1.27 ns/op
	// string/go-spring-8  41272039  28.3 ns/op
	b.Run("string", func(b *testing.B) {
		v := "true"
		b.Run("strconv", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := strconv.ParseBool(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToBoolE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func TestToBool(t *testing.T) {

	assert.Equal(t, cast.ToBool(nil), false)

	assert.Equal(t, cast.ToBool(int(3)), true)
	assert.Equal(t, cast.ToBool(int8(3)), true)
	assert.Equal(t, cast.ToBool(int16(3)), true)
	assert.Equal(t, cast.ToBool(int32(3)), true)
	assert.Equal(t, cast.ToBool(int64(3)), true)
	assert.Equal(t, cast.ToBool(cast.IntPtr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int8Ptr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int16Ptr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int32Ptr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int64Ptr(3)), true)

	assert.Equal(t, cast.ToBool(uint(3)), true)
	assert.Equal(t, cast.ToBool(uint8(3)), true)
	assert.Equal(t, cast.ToBool(uint16(3)), true)
	assert.Equal(t, cast.ToBool(uint32(3)), true)
	assert.Equal(t, cast.ToBool(uint64(3)), true)
	assert.Equal(t, cast.ToBool(cast.IntPtr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int8Ptr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int16Ptr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int32Ptr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Int64Ptr(3)), true)

	assert.Equal(t, cast.ToBool(float32(3)), true)
	assert.Equal(t, cast.ToBool(float64(3)), true)
	assert.Equal(t, cast.ToBool(cast.Float32Ptr(3)), true)
	assert.Equal(t, cast.ToBool(cast.Float64Ptr(3)), true)

	assert.Equal(t, cast.ToBool("3"), false)
	assert.Equal(t, cast.ToBool(cast.StringPtr("3")), false)
	assert.Equal(t, cast.ToBool("true"), true)
	assert.Equal(t, cast.ToBool(cast.StringPtr("true")), true)
	assert.Equal(t, cast.ToBool("false"), false)
	assert.Equal(t, cast.ToBool(cast.StringPtr("false")), false)

	assert.Equal(t, cast.ToBool(true), true)
	assert.Equal(t, cast.ToBool(false), false)
	assert.Equal(t, cast.ToBool(cast.BoolPtr(true)), true)
	assert.Equal(t, cast.ToBool(cast.BoolPtr(false)), false)

	_, err := cast.ToBoolE("abc")
	assert.Error(t, err, "strconv.ParseBool: parsing \"abc\": invalid syntax")
}
