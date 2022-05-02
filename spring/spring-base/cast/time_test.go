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
	"fmt"
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

	t.Run("unit", func(t *testing.T) {

		testcases := []struct {
			value  int64
			unit   string
			expect time.Time
		}{
			{1, cast.Nanosecond, time.Unix(0, 1)},
			{1, cast.Millisecond, time.Unix(0, 1*1e6)},
			{1, cast.Second, time.Unix(1, 0)},
			{1, cast.Hour, time.Unix(0, 0).Add(time.Hour)},
		}

		for i, testcase := range testcases {
			s := cast.ToTime(testcase.value, testcase.unit)
			assert.Equal(t, s, testcase.expect, fmt.Sprintf("index %d", i))
		}
	})

	t.Run("format", func(t *testing.T) {

		testcases := []struct {
			value  string
			format string
			expect time.Time
		}{
			{
				"1970-01-01 08:00:00.000000001 +0800",
				"2006-01-02 15:04:05.000000000 -0700",
				time.Unix(0, 1),
			},
			{
				"1s",
				"",
				time.Unix(1, 0),
			},
			{
				"1h1m1s",
				"",
				time.Unix(3661, 0),
			},
		}

		for i, testcase := range testcases {
			s := cast.ToTime(testcase.value, testcase.format)
			assert.Equal(t, s, testcase.expect, fmt.Sprintf("index %d", i))
		}
	})
}
