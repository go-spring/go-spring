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
