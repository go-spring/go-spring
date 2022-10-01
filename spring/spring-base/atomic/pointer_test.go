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

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
	"github.com/go-spring/spring-base/json"
)

type properties struct {
	Data int
}

func BenchmarkPointer(b *testing.B) {

	// Pointer and Value have almost same access performance.
	// BenchmarkPointer/value-8	    47710915   25.72 ns/op
	// BenchmarkPointer/pointer-8   44618223   25.78 ns/op

	b.Run("pointer", func(b *testing.B) {
		var p atomic.Pointer
		for i := 0; i < b.N; i++ {
			p.Store(unsafe.Pointer(&properties{Data: 1}))
			prop := (*properties)(p.Load())
			if prop.Data != 1 {
				b.Fatal(errors.New("prop data should be 1"))
			}
		}
	})

	b.Run("value", func(b *testing.B) {
		var p atomic.Value
		for i := 0; i < b.N; i++ {
			p.Store(&properties{Data: 2})
			prop := p.Load().(*properties)
			if prop.Data != 2 {
				b.Fatal(errors.New("prop data should be 2"))
			}
		}
	})
}

func TestPointer(t *testing.T) {

	// atomic.Pointer and unsafe.Pointer occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Pointer{}), uintptr(8+8))

	var p atomic.Pointer
	assert.Equal(t, p.Load(), unsafe.Pointer(nil))

	s1 := &properties{Data: 1}
	p.Store(unsafe.Pointer(s1))
	assert.Equal(t, p.Load(), unsafe.Pointer(s1))

	s2 := &properties{Data: 2}
	old := p.Swap(unsafe.Pointer(s2))
	assert.Equal(t, old, unsafe.Pointer(s1))
	assert.Equal(t, p.Load(), unsafe.Pointer(s2))

	s3 := &properties{Data: 3}
	swapped := p.CompareAndSwap(unsafe.Pointer(s2), unsafe.Pointer(s3))
	assert.True(t, swapped)
	assert.Equal(t, p.Load(), unsafe.Pointer(s3))

	swapped = p.CompareAndSwap(unsafe.Pointer(s2), unsafe.Pointer(s3))
	assert.False(t, swapped)
	assert.Equal(t, p.Load(), unsafe.Pointer(s3))

	p.SetMarshalJSON(func(pointer unsafe.Pointer) ([]byte, error) {
		s := (*properties)(pointer)
		return json.Marshal(s)
	})

	bytes, _ := json.Marshal(&p)
	assert.Equal(t, string(bytes), `{"Data":3}`)
}
