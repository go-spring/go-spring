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

package atomic_test

import (
	"errors"
	"testing"
	"unsafe"

	"github.com/go-spring/spring-base/atomic"
)

type properties struct {
	data int
}

func BenchmarkPointer(b *testing.B) {

	b.Run("pointer", func(b *testing.B) {
		var p atomic.Pointer
		for i := 0; i < b.N; i++ {
			p.Store(unsafe.Pointer(&properties{data: 1}))
			prop := (*properties)(p.Load())
			if prop.data != 1 {
				b.Fatal(errors.New("prop data should be 1"))
			}
		}
	})

	b.Run("value", func(b *testing.B) {
		var p atomic.Value
		for i := 0; i < b.N; i++ {
			p.Store(&properties{data: 2})
			prop := p.Load().(*properties)
			if prop.data != 2 {
				b.Fatal(errors.New("prop data should be 2"))
			}
		}
	})
}
