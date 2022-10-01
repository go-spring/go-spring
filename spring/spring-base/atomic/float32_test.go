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
	"github.com/go-spring/spring-base/json"
)

func TestFloat32(t *testing.T) {

	// atomic.Float32 and uint32 occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Float32{}), uintptr(4))

	var f atomic.Float32
	assert.Equal(t, f.Load(), float32(0))

	v := f.Add(float32(0.5))
	assert.Equal(t, v, float32(0.5))
	assert.Equal(t, f.Load(), float32(0.5))

	f.Store(1)
	assert.Equal(t, f.Load(), float32(1))

	old := f.Swap(2)
	assert.Equal(t, old, float32(1))
	assert.Equal(t, f.Load(), float32(2))

	swapped := f.CompareAndSwap(2, 3)
	assert.True(t, swapped)
	assert.Equal(t, f.Load(), float32(3))

	swapped = f.CompareAndSwap(2, 3)
	assert.False(t, swapped)
	assert.Equal(t, f.Load(), float32(3))

	bytes, _ := json.Marshal(&f)
	assert.Equal(t, string(bytes), "3")
}
