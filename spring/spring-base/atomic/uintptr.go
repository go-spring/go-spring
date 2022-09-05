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
)

type Uintptr struct {
	_ nocopy
	v uintptr
}

// Add wrapper for atomic.AddUintptr.
func (x *Uintptr) Add(delta uintptr) (new uintptr) {
	return atomic.AddUintptr(&x.v, delta)
}

// Load wrapper for atomic.LoadUintptr.
func (x *Uintptr) Load() (val uintptr) {
	return atomic.LoadUintptr(&x.v)
}

// Store wrapper for atomic.StoreUintptr.
func (x *Uintptr) Store(val uintptr) {
	atomic.StoreUintptr(&x.v, val)
}

// Swap wrapper for atomic.SwapUintptr.
func (x *Uintptr) Swap(new uintptr) (old uintptr) {
	return atomic.SwapUintptr(&x.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUintptr.
func (x *Uintptr) CompareAndSwap(old, new uintptr) (swapped bool) {
	return atomic.CompareAndSwapUintptr(&x.v, old, new)
}
