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

type Int64 struct {
	_ nocopy
	_ align64
	v int64
}

// Add wrapper for atomic.AddInt64.
func (x *Int64) Add(delta int64) int64 {
	return atomic.AddInt64(&x.v, delta)
}

// Load wrapper for atomic.LoadInt64.
func (x *Int64) Load() int64 {
	return atomic.LoadInt64(&x.v)
}

// Store wrapper for atomic.StoreInt64.
func (x *Int64) Store(val int64) {
	atomic.StoreInt64(&x.v, val)
}

// Swap wrapper for atomic.SwapInt64.
func (x *Int64) Swap(new int64) int64 {
	return atomic.SwapInt64(&x.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapInt64.
func (x *Int64) CompareAndSwap(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&x.v, old, new)
}

func (x *Int64) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
