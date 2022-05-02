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

	"github.com/go-spring/spring-base/util"
)

type Float64 struct {
	_ util.NoCopy
	v uint64
}

// Add wrapper for atomic.AddUint64.
func (u *Float64) Add(delta float64) (new float64) {
	return math.Float64frombits(atomic.AddUint64(&u.v, math.Float64bits(delta)))
}

// Load wrapper for atomic.LoadUint64.
func (u *Float64) Load() (val float64) {
	return math.Float64frombits(atomic.LoadUint64(&u.v))
}

// Store wrapper for atomic.StoreUint64.
func (u *Float64) Store(val float64) {
	atomic.StoreUint64(&u.v, math.Float64bits(val))
}

// Swap wrapper for atomic.SwapUint64.
func (u *Float64) Swap(new float64) (old float64) {
	return math.Float64frombits(atomic.SwapUint64(&u.v, math.Float64bits(new)))
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint64.
func (u *Float64) CompareAndSwap(old, new float64) (swapped bool) {
	return atomic.CompareAndSwapUint64(&u.v, math.Float64bits(old), math.Float64bits(new))
}
