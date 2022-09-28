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

func TestFloat64(t *testing.T) {

	// atomic.Float64 and uint64 occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.Float64{}), uintptr(8))

	var f atomic.Float64
	assert.Equal(t, f.Load(), float64(0))

	f.Store(1)
	assert.Equal(t, f.Load(), float64(1))

	f.Swap(2)
	assert.Equal(t, f.Load(), float64(2))

	f.CompareAndSwap(2, 3)
	assert.Equal(t, f.Load(), float64(3))

	f.CompareAndSwap(2, 3)
	assert.Equal(t, f.Load(), float64(3))

	bytes, _ := json.Marshal(&f)
	assert.Equal(t, string(bytes), "3")
}
