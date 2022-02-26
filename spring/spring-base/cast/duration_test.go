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
