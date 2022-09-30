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
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func TestToUint(t *testing.T) {

	assert.Equal(t, cast.ToUint(nil), uint(0))

	assert.Equal(t, cast.ToUint(int(3)), uint(3))
	assert.Equal(t, cast.ToUint(int8(3)), uint(3))
	assert.Equal(t, cast.ToUint(int16(3)), uint(3))
	assert.Equal(t, cast.ToUint(int32(3)), uint(3))
	assert.Equal(t, cast.ToUint(int64(3)), uint(3))
	assert.Equal(t, cast.ToUint(cast.IntPtr(3)), uint(3))
	assert.Equal(t, cast.ToUint(cast.Int8Ptr(3)), uint(3))
	assert.Equal(t, cast.ToUint(cast.Int16Ptr(3)), uint(3))
	assert.Equal(t, cast.ToUint(cast.Int32Ptr(3)), uint(3))
	assert.Equal(t, cast.ToUint(cast.Int64Ptr(3)), uint(3))

	assert.Equal(t, cast.ToUint8(uint(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(uint8(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(uint16(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(uint32(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(uint64(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(cast.UintPtr(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(cast.Uint8Ptr(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(cast.Uint16Ptr(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(cast.Uint32Ptr(3)), uint8(3))
	assert.Equal(t, cast.ToUint8(cast.Uint64Ptr(3)), uint8(3))

	assert.Equal(t, cast.ToUint16(float32(3)), uint16(3))
	assert.Equal(t, cast.ToUint16(float64(3)), uint16(3))
	assert.Equal(t, cast.ToUint16(cast.Float32Ptr(3)), uint16(3))
	assert.Equal(t, cast.ToUint16(cast.Float64Ptr(3)), uint16(3))

	assert.Equal(t, cast.ToUint32("3"), uint32(3))
	assert.Equal(t, cast.ToUint32(cast.StringPtr("3")), uint32(3))

	assert.Equal(t, cast.ToUint64(true), uint64(1))
	assert.Equal(t, cast.ToUint64(false), uint64(0))
	assert.Equal(t, cast.ToUint64(cast.BoolPtr(true)), uint64(1))
	assert.Equal(t, cast.ToUint64(cast.BoolPtr(false)), uint64(0))

	_, err := cast.ToUint64E("abc")
	assert.Error(t, err, "strconv.ParseUint: parsing \"abc\": invalid syntax")
}
