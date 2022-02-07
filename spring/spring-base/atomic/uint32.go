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

import "sync/atomic"

type Uint32 struct {
	v uint32
}

func NewUint32(val uint32) *Uint32 {
	return &Uint32{v: val}
}

// Add wrapper for atomic.AddUint32.
func (u *Uint32) Add(delta uint32) (new uint32) {
	return atomic.AddUint32(&u.v, delta)
}

// Load wrapper for atomic.LoadUint32.
func (u *Uint32) Load() (val uint32) {
	return atomic.LoadUint32(&u.v)
}

// Store wrapper for atomic.StoreUint32.
func (u *Uint32) Store(val uint32) {
	atomic.StoreUint32(&u.v, val)
}

// Swap wrapper for atomic.SwapUint32.
func (u *Uint32) Swap(new uint32) (old uint32) {
	return atomic.SwapUint32(&u.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint32.
func (u *Uint32) CompareAndSwap(old, new uint32) (swapped bool) {
	return atomic.CompareAndSwapUint32(&u.v, old, new)
}
