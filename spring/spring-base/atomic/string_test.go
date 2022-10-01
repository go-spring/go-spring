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

func TestString(t *testing.T) {

	// atomic.String and interface{} occupy the same space
	assert.Equal(t, unsafe.Sizeof(atomic.String{}), uintptr(16))

	var s atomic.String
	assert.Equal(t, s.Load(), "")

	s.Store("1")
	assert.Equal(t, s.Load(), "1")

	bytes, _ := json.Marshal(&s)
	assert.Equal(t, string(bytes), `"1"`)
}
