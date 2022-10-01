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

func TestUint32(t *testing.T) {

	// atomic.Uint32 and uint32 occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Uint32{}), uintptr(4))

	var u atomic.Uint32
	assert.Equal(t, u.Load(), uint32(0))

	v := u.Add(5)
	assert.Equal(t, v, uint32(5))
	assert.Equal(t, u.Load(), uint32(5))

	u.Store(1)
	assert.Equal(t, u.Load(), uint32(1))

	old := u.Swap(2)
	assert.Equal(t, old, uint32(1))
	assert.Equal(t, u.Load(), uint32(2))

	swapped := u.CompareAndSwap(2, 3)
	assert.True(t, swapped)
	assert.Equal(t, u.Load(), uint32(3))

	swapped = u.CompareAndSwap(2, 3)
	assert.False(t, swapped)
	assert.Equal(t, u.Load(), uint32(3))

	bytes, _ := json.Marshal(&u)
	assert.Equal(t, string(bytes), "3")
}
