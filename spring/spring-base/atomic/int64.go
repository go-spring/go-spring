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

	"github.com/go-spring/spring-base/util"
)

type Int64 struct {
	_ util.NoCopy
	v int64
}

// Add wrapper for atomic.AddInt64.
func (i *Int64) Add(delta int64) int64 {
	return atomic.AddInt64(&i.v, delta)
}

// Load wrapper for atomic.LoadInt64.
func (i *Int64) Load() int64 {
	return atomic.LoadInt64(&i.v)
}

// Store wrapper for atomic.StoreInt64.
func (i *Int64) Store(val int64) {
	atomic.StoreInt64(&i.v, val)
}

// Swap wrapper for atomic.SwapInt64.
func (i *Int64) Swap(new int64) int64 {
	return atomic.SwapInt64(&i.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapInt64.
func (i *Int64) CompareAndSwap(old, new int64) bool {
	return atomic.CompareAndSwapInt64(&i.v, old, new)
}
