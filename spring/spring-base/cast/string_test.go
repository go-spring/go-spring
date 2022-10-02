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
	"bytes"
	"errors"
	"html/template"
	"strconv"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/cast"
)

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

func TestToString(t *testing.T) {

	assert.Equal(t, cast.ToString(nil), "")

	assert.Equal(t, cast.ToString(int(3)), "3")
	assert.Equal(t, cast.ToString(int8(3)), "3")
	assert.Equal(t, cast.ToString(int16(3)), "3")
	assert.Equal(t, cast.ToString(int32(3)), "3")
	assert.Equal(t, cast.ToString(int64(3)), "3")
	assert.Equal(t, cast.ToString(cast.IntPtr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Int8Ptr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Int16Ptr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Int32Ptr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Int64Ptr(3)), "3")

	assert.Equal(t, cast.ToString(uint(3)), "3")
	assert.Equal(t, cast.ToString(uint8(3)), "3")
	assert.Equal(t, cast.ToString(uint16(3)), "3")
	assert.Equal(t, cast.ToString(uint32(3)), "3")
	assert.Equal(t, cast.ToString(uint64(3)), "3")
	assert.Equal(t, cast.ToString(cast.UintPtr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Uint8Ptr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Uint16Ptr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Uint32Ptr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Uint64Ptr(3)), "3")

	assert.Equal(t, cast.ToString(float32(3)), "3")
	assert.Equal(t, cast.ToString(float64(3)), "3")
	assert.Equal(t, cast.ToString(cast.Float32Ptr(3)), "3")
	assert.Equal(t, cast.ToString(cast.Float64Ptr(3)), "3")

	assert.Equal(t, cast.ToString("3"), "3")
	assert.Equal(t, cast.ToString(cast.StringPtr("3")), "3")

	assert.Equal(t, cast.ToString(true), "true")
	assert.Equal(t, cast.ToString(false), "false")
	assert.Equal(t, cast.ToString(cast.BoolPtr(true)), "true")
	assert.Equal(t, cast.ToString(cast.BoolPtr(false)), "false")

	assert.Equal(t, cast.ToString([]byte("3")), "3")
	assert.Equal(t, cast.ToString(template.HTML("3")), "3")
	assert.Equal(t, cast.ToString(template.URL("3")), "3")
	assert.Equal(t, cast.ToString(template.JS("3")), "3")
	assert.Equal(t, cast.ToString(template.CSS("3")), "3")
	assert.Equal(t, cast.ToString(template.HTMLAttr("3")), "3")
	assert.Equal(t, cast.ToString(bytes.NewBufferString("abc")), "abc")
	assert.Equal(t, cast.ToString(errors.New("abc")), "abc")

	type String string
	_, err := cast.ToStringE(String("abc"))
	assert.Error(t, err, "unable to cast type \\(cast_test\\.String\\) to string")
}

func TestBytesToString(t *testing.T) {
	b := []byte("hello, gopher!")
	s := cast.BytesToString(b)
	assert.Equal(t, b, []byte(s))
}

func TestStringToBytes(t *testing.T) {
	s := "hello, gopher!"
	b := cast.StringToBytes(s)
	assert.Equal(t, string(b), s)
}
