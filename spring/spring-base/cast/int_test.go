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
