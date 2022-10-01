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
	"testing"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
)

func TestUintptr(t *testing.T) {

	// atomic.Uintptr and uintptr occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Uintptr{}), uintptr(8))

	var p atomic.Uintptr
	assert.Equal(t, p.Load(), uintptr(0))

	v := p.Add(0)
	assert.Equal(t, v, uintptr(0))
	assert.Equal(t, p.Load(), uintptr(0))

	s1 := &properties{Data: 1}
	p.Store(uintptr(unsafe.Pointer(s1)))
	assert.Equal(t, p.Load(), uintptr(unsafe.Pointer(s1)))

	s2 := &properties{Data: 2}
	old := p.Swap(uintptr(unsafe.Pointer(s2)))
	assert.Equal(t, old, uintptr(unsafe.Pointer(s1)))
	assert.Equal(t, p.Load(), uintptr(unsafe.Pointer(s2)))

	s3 := &properties{Data: 3}
	swapped := p.CompareAndSwap(uintptr(unsafe.Pointer(s2)), uintptr(unsafe.Pointer(s3)))
	assert.True(t, swapped)
	assert.Equal(t, p.Load(), uintptr(unsafe.Pointer(s3)))

	swapped = p.CompareAndSwap(uintptr(unsafe.Pointer(s2)), uintptr(unsafe.Pointer(s3)))
	assert.False(t, swapped)
	assert.Equal(t, p.Load(), uintptr(unsafe.Pointer(s3)))
}
