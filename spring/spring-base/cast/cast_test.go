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
	"strconv"
	"testing"
	"time"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

func ptr(i interface{}) interface{} {
	switch v := i.(type) {
	case bool:
		return &v
	case int:
		return &v
	case int8:
		return &v
	case int16:
		return &v
	case int32:
		return &v
	case int64:
		return &v
	case uint:
		return &v
	case uint8:
		return &v
	case uint16:
		return &v
	case uint32:
		return &v
	case uint64:
		return &v
	case float32:
		return &v
	case float64:
		return &v
	case string:
		return &v
	default:
		return nil
	}
}

func TestToInt(t *testing.T) {

	testcases := []struct {
		param  interface{}
		expect int64
	}{
		{int64(10), int64(10)},
		{ptr(int64(10)), int64(10)},
		{10.0, int64(10)},
		{ptr(10.0), int64(10)},
		{"10", int64(10)},
		{ptr("10"), int64(10)},
		{true, int64(1)},
		{ptr(true), int64(1)},
	}

	for i, testcase := range testcases {
		v := cast.ToInt64(testcase.param)
		assert.Equal(t, v, testcase.expect, fmt.Sprintf("index %d", i))
	}
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

func BenchmarkToString(b *testing.B) {

	// int/strconv-8    419501868 2.87 ns/op
	// int/go-spring-8  60869038  18.2 ns/op
	b.Run("int", func(b *testing.B) {
		v := 10
		b.Run("strconv", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = strconv.Itoa(v)
			}
		})
		b.Run("go-spring", func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, err := cast.ToStringE(v)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}

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
