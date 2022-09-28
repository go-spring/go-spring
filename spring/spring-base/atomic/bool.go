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

// A Bool is an atomic bool value.
type Bool struct {
	_ nocopy
	v uint32
}

func bool2uint(val bool) uint32 {
	if val {
		return 1
	}
	return 0
}

func uint2bool(val uint32) bool {
	return val != 0
}

// Load atomically loads and returns the value stored in x.
func (x *Bool) Load() (val bool) {
	return uint2bool(atomic.LoadUint32(&x.v))
}

// Store atomically stores val into x.
func (x *Bool) Store(val bool) {
	atomic.StoreUint32(&x.v, bool2uint(val))
}

// Swap atomically stores new into x and returns the old value.
func (x *Bool) Swap(new bool) (old bool) {
	return uint2bool(atomic.SwapUint32(&x.v, bool2uint(new)))
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Bool) CompareAndSwap(old, new bool) (swapped bool) {
	return atomic.CompareAndSwapUint32(&x.v, bool2uint(old), bool2uint(new))
}

// MarshalJSON returns the JSON encoding of x.
func (x *Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
