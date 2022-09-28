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
	"encoding/json"
	"math"
	"sync/atomic"
)

// A Float64 is an atomic float64 value.
type Float64 struct {
	_ nocopy
	v uint64
}

// Add atomically adds delta to x and returns the new value.
func (x *Float64) Add(delta float64) (new float64) {
	return math.Float64frombits(atomic.AddUint64(&x.v, math.Float64bits(delta)))
}

// Load atomically loads and returns the value stored in x.
func (x *Float64) Load() (val float64) {
	return math.Float64frombits(atomic.LoadUint64(&x.v))
}

// Store atomically stores val into x.
func (x *Float64) Store(val float64) {
	atomic.StoreUint64(&x.v, math.Float64bits(val))
}

// Swap atomically stores new into x and returns the old value.
func (x *Float64) Swap(new float64) (old float64) {
	return math.Float64frombits(atomic.SwapUint64(&x.v, math.Float64bits(new)))
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Float64) CompareAndSwap(old, new float64) (swapped bool) {
	return atomic.CompareAndSwapUint64(&x.v, math.Float64bits(old), math.Float64bits(new))
}

// MarshalJSON returns the JSON encoding of x.
func (x *Float64) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
