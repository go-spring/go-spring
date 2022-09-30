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
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func BenchmarkToTime(b *testing.B) {
	// string/parse-8      4266552 277 ns/op
	// string/go-spring-8  3559998 324 ns/op
	b.Run("string", func(b *testing.B) {
		format := "2006-01-02 15:04:05 -0700"
		v := time.Now().Format(format)
		b.Run("parse", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := time.Parse(format, v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToTimeE(v, format)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func TestToTime(t *testing.T) {

	assert.Equal(t, cast.ToTime(nil), time.Time{})

	assert.Equal(t, cast.ToTime(int(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(int8(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(int16(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(int32(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(int64(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.IntPtr(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.Int8Ptr(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.Int16Ptr(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.Int32Ptr(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.Int64Ptr(3)), time.Unix(0, 3))

	assert.Equal(t, cast.ToTime(uint(3), "ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(uint8(3), "ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(uint16(3), "ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(uint32(3), "ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(uint64(3), "ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.UintPtr(3), "s"), time.Unix(3, 0))
	assert.Equal(t, cast.ToTime(cast.Uint8Ptr(3), "s"), time.Unix(3, 0))
	assert.Equal(t, cast.ToTime(cast.Uint16Ptr(3), "s"), time.Unix(3, 0))
	assert.Equal(t, cast.ToTime(cast.Uint32Ptr(3), "s"), time.Unix(3, 0))
	assert.Equal(t, cast.ToTime(cast.Uint64Ptr(3), "s"), time.Unix(3, 0))

	assert.Equal(t, cast.ToTime(float32(3), "ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(float64(3), "ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.Float32Ptr(3)), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.Float64Ptr(3)), time.Unix(0, 3))

	assert.Equal(t, cast.ToTime("3ns"), time.Unix(0, 3))
	assert.Equal(t, cast.ToTime(cast.StringPtr("3ns")), time.Unix(0, 3))
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	{
		got := cast.ToTime("2022-09-30 15:30:00 +0800")
		expect := time.Date(2022, 9, 30, 15, 30, 0, 0, location)
		assert.True(t, got.Equal(expect))
	}
	{
		got := cast.ToTime(cast.StringPtr("2022-09-30 15:30:00 +0800"))
		expect := time.Date(2022, 9, 30, 15, 30, 0, 0, location)
		assert.True(t, got.Equal(expect))
	}

	_, err = cast.ToTimeE(true)
	assert.Error(t, err, "unable to cast type bool to Time")

	_, err = cast.ToTimeE(false)
	assert.Error(t, err, "unable to cast type bool to Time")

	_, err = cast.ToTimeE(cast.BoolPtr(true))
	assert.Error(t, err, "unable to cast type \\*bool to Time")

	_, err = cast.ToTimeE(cast.BoolPtr(false))
	assert.Error(t, err, "unable to cast type \\*bool to Time")

	_, err = cast.ToTimeE("abc")
	assert.Error(t, err, "cannot parse \"abc\" as \"2006\"")
}
