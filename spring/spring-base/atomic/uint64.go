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
	"sync/atomic"
)

type Uint64 struct {
	_ nocopy
	_ align64
	v uint64
}

// Add wrapper for atomic.AddUint64.
func (x *Uint64) Add(delta uint64) (new uint64) {
	return atomic.AddUint64(&x.v, delta)
}

// Load wrapper for atomic.LoadUint64.
func (x *Uint64) Load() (val uint64) {
	return atomic.LoadUint64(&x.v)
}

// Store wrapper for atomic.StoreUint64.
func (x *Uint64) Store(val uint64) {
	atomic.StoreUint64(&x.v, val)
}

// Swap wrapper for atomic.SwapUint64.
func (x *Uint64) Swap(new uint64) (old uint64) {
	return atomic.SwapUint64(&x.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint64.
func (x *Uint64) CompareAndSwap(old, new uint64) (swapped bool) {
	return atomic.CompareAndSwapUint64(&x.v, old, new)
}

func (x *Uint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
