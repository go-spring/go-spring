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

func BenchmarkToDuration(b *testing.B) {
	// string/parse-8      28863253 38.5 ns/op
	// string/go-spring-8  18037459 66.7 ns/op
	b.Run("string", func(b *testing.B) {
		v := cast.ToString(time.Now().UnixNano()) + "ns"
		b.Run("parse", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := time.ParseDuration(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToDurationE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

func TestToDuration(t *testing.T) {

	assert.Equal(t, cast.ToDuration(nil), time.Duration(0))

	assert.Equal(t, cast.ToDuration(int(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(int8(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(int16(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(int32(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(int64(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.IntPtr(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.Int8Ptr(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.Int16Ptr(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.Int32Ptr(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.Int64Ptr(3)), time.Duration(3))

	assert.Equal(t, cast.ToDuration(uint(3), time.Millisecond), 3*time.Millisecond)
	assert.Equal(t, cast.ToDuration(uint8(3), time.Millisecond), 3*time.Millisecond)
	assert.Equal(t, cast.ToDuration(uint16(3), time.Millisecond), 3*time.Millisecond)
	assert.Equal(t, cast.ToDuration(uint32(3), time.Millisecond), 3*time.Millisecond)
	assert.Equal(t, cast.ToDuration(uint64(3), time.Millisecond), 3*time.Millisecond)
	assert.Equal(t, cast.ToDuration(cast.UintPtr(3), time.Second), 3*time.Second)
	assert.Equal(t, cast.ToDuration(cast.Uint8Ptr(3), time.Second), 3*time.Second)
	assert.Equal(t, cast.ToDuration(cast.Uint16Ptr(3), time.Second), 3*time.Second)
	assert.Equal(t, cast.ToDuration(cast.Uint32Ptr(3), time.Second), 3*time.Second)
	assert.Equal(t, cast.ToDuration(cast.Uint64Ptr(3), time.Second), 3*time.Second)

	assert.Equal(t, cast.ToDuration(float32(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(float64(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.Float32Ptr(3)), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.Float64Ptr(3)), time.Duration(3))

	assert.Equal(t, cast.ToDuration("3ns"), time.Duration(3))
	assert.Equal(t, cast.ToDuration(cast.StringPtr("3ns")), time.Duration(3))

	assert.Equal(t, cast.ToDuration(time.Second), time.Second)
	assert.Equal(t, cast.ToDuration(time.Minute), time.Minute)

	_, err := cast.ToDurationE(true)
	assert.Error(t, err, "unable to cast type \\(bool\\) to time.Duration")

	_, err = cast.ToDurationE(false)
	assert.Error(t, err, "unable to cast type \\(bool\\) to time.Duration")

	_, err = cast.ToDurationE(cast.BoolPtr(true))
	assert.Error(t, err, "unable to cast type \\(\\*bool\\) to time.Duration")

	_, err = cast.ToDurationE(cast.BoolPtr(false))
	assert.Error(t, err, "unable to cast type \\(\\*bool\\) to time.Duration")

	_, err = cast.ToDurationE("3")
	assert.Error(t, err, "time: missing unit in duration \"3\"")

	_, err = cast.ToDurationE("abc")
	assert.Error(t, err, "time: invalid duration \"abc\"")
}
