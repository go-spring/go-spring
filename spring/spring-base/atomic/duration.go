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
	"time"
)

type Duration struct {
	v int64
}

func NewDuration(val time.Duration) *Duration {
	return &Duration{v: int64(val)}
}

// Add wrapper for atomic.AddInt64.
func (i *Duration) Add(delta time.Duration) time.Duration {
	return time.Duration(atomic.AddInt64(&i.v, int64(delta)))
}

// Load wrapper for atomic.LoadInt64.
func (i *Duration) Load() time.Duration {
	return time.Duration(atomic.LoadInt64(&i.v))
}

// Store wrapper for atomic.StoreInt64.
func (i *Duration) Store(val time.Duration) {
	atomic.StoreInt64(&i.v, int64(val))
}

// Swap wrapper for atomic.SwapInt64.
func (i *Duration) Swap(new time.Duration) time.Duration {
	return time.Duration(atomic.SwapInt64(&i.v, int64(new)))
}

// CompareAndSwap wrapper for atomic.CompareAndSwapInt64.
func (i *Duration) CompareAndSwap(old, new time.Duration) bool {
	return atomic.CompareAndSwapInt64(&i.v, int64(old), int64(new))
}
