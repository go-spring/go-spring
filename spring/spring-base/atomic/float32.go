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

// A Float32 is an atomic float32 value.
type Float32 struct {
	_ nocopy
	v uint32
}

// Add atomically adds delta to x and returns the new value.
func (x *Float32) Add(delta float32) (new float32) {
	return math.Float32frombits(atomic.AddUint32(&x.v, math.Float32bits(delta)))
}

// Load atomically loads and returns the value stored in x.
func (x *Float32) Load() (val float32) {
	return math.Float32frombits(atomic.LoadUint32(&x.v))
}

// Store atomically stores val into x.
func (x *Float32) Store(val float32) {
	atomic.StoreUint32(&x.v, math.Float32bits(val))
}

// Swap atomically stores new into x and returns the old value.
func (x *Float32) Swap(new float32) (old float32) {
	return math.Float32frombits(atomic.SwapUint32(&x.v, math.Float32bits(new)))
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Float32) CompareAndSwap(old, new float32) (swapped bool) {
	return atomic.CompareAndSwapUint32(&x.v, math.Float32bits(old), math.Float32bits(new))
}

// MarshalJSON returns the JSON encoding of x.
func (x *Float32) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
