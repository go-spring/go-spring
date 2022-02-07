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

import "sync/atomic"

type Bool struct {
	v int32
}

func NewBool(val bool) *Bool {
	return &Bool{v: bool2int(val)}
}

func bool2int(val bool) int32 {
	if val {
		return 1
	}
	return 0
}

func int2bool(val int32) bool {
	return val == 1
}

// Store wrapper for atomic.StoreInt32.
func (i *Bool) Store(val bool) {
	atomic.StoreInt32(&i.v, bool2int(val))
}

// Load wrapper for atomic.LoadInt32.
func (i *Bool) Load() (val bool) {
	return int2bool(atomic.LoadInt32(&i.v))
}

// Swap wrapper for atomic.SwapInt32.
func (i *Bool) Swap(new bool) (old bool) {
	return int2bool(atomic.SwapInt32(&i.v, bool2int(new)))
}

// CompareAndSwap wrapper for atomic.CompareAndSwapInt32.
func (i *Bool) CompareAndSwap(old, new bool) (swapped bool) {
	return atomic.CompareAndSwapInt32(&i.v, bool2int(old), bool2int(new))
}
