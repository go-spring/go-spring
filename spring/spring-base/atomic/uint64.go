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

type Uint64 struct {
	v uint64
}

func NewUint64(val uint64) *Uint64 {
	return &Uint64{v: val}
}

// Add wrapper for atomic.AddUint64.
func (u *Uint64) Add(delta uint64) (new uint64) {
	return atomic.AddUint64(&u.v, delta)
}

// Load wrapper for atomic.LoadUint64.
func (u *Uint64) Load() (val uint64) {
	return atomic.LoadUint64(&u.v)
}

// Store wrapper for atomic.StoreUint64.
func (u *Uint64) Store(val uint64) {
	atomic.StoreUint64(&u.v, val)
}

// Swap wrapper for atomic.SwapUint64.
func (u *Uint64) Swap(new uint64) (old uint64) {
	return atomic.SwapUint64(&u.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint64.
func (u *Uint64) CompareAndSwap(old, new uint64) (swapped bool) {
	return atomic.CompareAndSwapUint64(&u.v, old, new)
}
