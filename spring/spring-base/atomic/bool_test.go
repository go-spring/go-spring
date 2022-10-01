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

func TestBool(t *testing.T) {

	// atomic.Bool and uint32 occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Bool{}), uintptr(4))

	var b atomic.Bool
	assert.False(t, b.Load())

	b.Store(true)
	assert.True(t, b.Load())

	old := b.Swap(false)
	assert.True(t, old)
	assert.False(t, b.Load())

	swapped := b.CompareAndSwap(false, true)
	assert.True(t, swapped)
	assert.True(t, b.Load())

	swapped = b.CompareAndSwap(false, true)
	assert.False(t, swapped)
	assert.True(t, b.Load())

	bytes, _ := json.Marshal(&b)
	assert.Equal(t, string(bytes), "true")
}
