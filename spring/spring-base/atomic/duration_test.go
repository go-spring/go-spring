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
	"time"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/atomic"
)

func TestDuration(t *testing.T) {

	// atomic.Duration 和 int64 占用的空间大小一样
	assert.Equal(t, unsafe.Sizeof(atomic.Duration{}), uintptr(8))

	var d atomic.Duration
	assert.Equal(t, d.Load(), time.Duration(0))

	d.Store(time.Minute)
	assert.Equal(t, d.Load(), time.Minute)
}
