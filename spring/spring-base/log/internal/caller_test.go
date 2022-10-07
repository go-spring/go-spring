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

package internal_test

import (
	"fmt"
	"testing"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/log/internal"
)

func TestCaller(t *testing.T) {
	for i := 0; i < 2; i++ {
		file, line, loaded := internal.Caller(0, true)
		if i == 0 {
			assert.False(t, loaded)
		} else {
			assert.True(t, loaded)
		}
		_ = fmt.Sprintf("%s:%d\n", file, line)
	}
}

func BenchmarkCaller(b *testing.B) {
	// BenchmarkCaller/fast-8  2393902  463.3 ns/op
	// BenchmarkCaller/slow-8  1470334  814.4 ns/op
	b.Run("fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			file, line, _ := internal.Caller(0, true)
			_ = fmt.Sprintf("%s:%d\n", file, line)
		}
	})
	b.Run("slow", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			file, line, _ := internal.Caller(0, false)
			_ = fmt.Sprintf("%s:%d\n", file, line)
		}
	})
}
