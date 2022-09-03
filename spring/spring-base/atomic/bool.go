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

// Store wrapper for atomic.StoreUint32.
func (x *Bool) Store(val bool) {
	atomic.StoreUint32(&x.v, bool2uint(val))
}

// Load wrapper for atomic.LoadUint32.
func (x *Bool) Load() (val bool) {
	return uint2bool(atomic.LoadUint32(&x.v))
}

// Swap wrapper for atomic.SwapUint32.
func (x *Bool) Swap(new bool) (old bool) {
	return uint2bool(atomic.SwapUint32(&x.v, bool2uint(new)))
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint32.
func (x *Bool) CompareAndSwap(old, new bool) (swapped bool) {
	return atomic.CompareAndSwapUint32(&x.v, bool2uint(old), bool2uint(new))
}

func (x *Bool) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
