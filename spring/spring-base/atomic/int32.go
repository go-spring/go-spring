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

package atomic

import (
	"sync/atomic"

	"github.com/go-spring/spring-base/util"
)

type Int32 struct {
	_ util.NoCopy
	v int32
}

// Add wrapper for atomic.AddInt32.
func (i *Int32) Add(delta int32) (new int32) {
	return atomic.AddInt32(&i.v, delta)
}

// Store wrapper for atomic.StoreInt32.
func (i *Int32) Store(val int32) {
	atomic.StoreInt32(&i.v, val)
}

// Load wrapper for atomic.LoadInt32.
func (i *Int32) Load() (val int32) {
	return atomic.LoadInt32(&i.v)
}

// Swap wrapper for atomic.SwapInt32.
func (i *Int32) Swap(new int32) (old int32) {
	return atomic.SwapInt32(&i.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapInt32.
func (i *Int32) CompareAndSwap(old, new int32) (swapped bool) {
	return atomic.CompareAndSwapInt32(&i.v, old, new)
}
