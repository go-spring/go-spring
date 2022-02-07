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
	"unsafe"
)

type UnsafePointer struct {
	v unsafe.Pointer
}

func NewPointer(val unsafe.Pointer) *UnsafePointer {
	return &UnsafePointer{v: val}
}

// Load wrapper for atomic.LoadPointer.
func (p *UnsafePointer) Load() (val unsafe.Pointer) {
	return atomic.LoadPointer(&p.v)
}

// Store wrapper for atomic.StorePointer.
func (p *UnsafePointer) Store(val unsafe.Pointer) {
	atomic.StorePointer(&p.v, val)
}

// Swap wrapper for atomic.SwapPointer.
func (p *UnsafePointer) Swap(new unsafe.Pointer) (old unsafe.Pointer) {
	return atomic.SwapPointer(&p.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapPointer.
func (p *UnsafePointer) CompareAndSwap(old, new unsafe.Pointer) (swapped bool) {
	return atomic.CompareAndSwapPointer(&p.v, old, new)
}
