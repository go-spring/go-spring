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
	"encoding/json"
	"testing"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
)

func TestInt32(t *testing.T) {

	// atomic.Int32 and int32 occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Int32{}), uintptr(4))

	var i atomic.Int32
	assert.Equal(t, i.Load(), int32(0))

	i.Store(1)
	assert.Equal(t, i.Load(), int32(1))

	i.Swap(2)
	assert.Equal(t, i.Load(), int32(2))

	i.CompareAndSwap(2, 3)
	assert.Equal(t, i.Load(), int32(3))

	i.CompareAndSwap(2, 3)
	assert.Equal(t, i.Load(), int32(3))

	bytes, _ := json.Marshal(&i)
	assert.Equal(t, string(bytes), "3")
}
