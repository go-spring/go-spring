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
	"sync/atomic"

	"github.com/go-spring/spring-base/json"
)

// An Int32 is an atomic int32 value.
type Int32 struct {
	_ nocopy
	v int32
}

// Add atomically adds delta to x and returns the new value.
func (x *Int32) Add(delta int32) (new int32) {
	return atomic.AddInt32(&x.v, delta)
}

// Load atomically loads and returns the value stored in x.
func (x *Int32) Load() (val int32) {
	return atomic.LoadInt32(&x.v)
}

// Store atomically stores val into x.
func (x *Int32) Store(val int32) {
	atomic.StoreInt32(&x.v, val)
}

// Swap atomically stores new into x and returns the old value.
func (x *Int32) Swap(new int32) (old int32) {
	return atomic.SwapInt32(&x.v, new)
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Int32) CompareAndSwap(old, new int32) (swapped bool) {
	return atomic.CompareAndSwapInt32(&x.v, old, new)
}

// MarshalJSON returns the JSON encoding of x.
func (x *Int32) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.Load())
}
