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
	"math"
	"sync/atomic"
)

type Float32 struct {
	v uint32
}

func NewFloat32(val float32) *Float32 {
	return &Float32{v: math.Float32bits(val)}
}

// Add wrapper for atomic.AddUint32.
func (u *Float32) Add(delta float32) (new float32) {
	return math.Float32frombits(atomic.AddUint32(&u.v, math.Float32bits(delta)))
}

// Load wrapper for atomic.LoadUint32.
func (u *Float32) Load() (val float32) {
	return math.Float32frombits(atomic.LoadUint32(&u.v))
}

// Store wrapper for atomic.StoreUint32.
func (u *Float32) Store(val float32) {
	atomic.StoreUint32(&u.v, math.Float32bits(val))
}

// Swap wrapper for atomic.SwapUint32.
func (u *Float32) Swap(new float32) (old float32) {
	return math.Float32frombits(atomic.SwapUint32(&u.v, math.Float32bits(new)))
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint32.
func (u *Float32) CompareAndSwap(old, new float32) (swapped bool) {
	return atomic.CompareAndSwapUint32(&u.v, math.Float32bits(old), math.Float32bits(new))
}
