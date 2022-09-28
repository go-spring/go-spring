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
	"time"
)

// A Duration is an atomic time.Duration value.
type Duration struct {
	_ nocopy
	_ align64
	v int64
}

// Add atomically adds delta to x and returns the new value.
func (x *Duration) Add(delta time.Duration) time.Duration {
	return time.Duration(atomic.AddInt64(&x.v, int64(delta)))
}

// Load atomically loads and returns the value stored in x.
func (x *Duration) Load() time.Duration {
	return time.Duration(atomic.LoadInt64(&x.v))
}

// Store atomically stores val into x.
func (x *Duration) Store(val time.Duration) {
	atomic.StoreInt64(&x.v, int64(val))
}

// Swap atomically stores new into x and returns the old value.
func (x *Duration) Swap(new time.Duration) time.Duration {
	return time.Duration(atomic.SwapInt64(&x.v, int64(new)))
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Duration) CompareAndSwap(old, new time.Duration) bool {
	return atomic.CompareAndSwapInt64(&x.v, int64(old), int64(new))
}

// MarshalJSON returns the JSON encoding of x.
func (x *Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
