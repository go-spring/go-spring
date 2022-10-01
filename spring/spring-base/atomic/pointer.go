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

type MarshalPointer func(unsafe.Pointer) ([]byte, error)

// A Pointer is an atomic pointer value.
type Pointer struct {
	_ nocopy
	v unsafe.Pointer

	marshalJSON MarshalPointer
}

// Load atomically loads and returns the value stored in x.
func (p *Pointer) Load() (val unsafe.Pointer) {
	return atomic.LoadPointer(&p.v)
}

// Store atomically stores val into x.
func (p *Pointer) Store(val unsafe.Pointer) {
	atomic.StorePointer(&p.v, val)
}

// Swap atomically stores new into x and returns the old value.
func (p *Pointer) Swap(new unsafe.Pointer) (old unsafe.Pointer) {
	return atomic.SwapPointer(&p.v, new)
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (p *Pointer) CompareAndSwap(old, new unsafe.Pointer) (swapped bool) {
	return atomic.CompareAndSwapPointer(&p.v, old, new)
}

// SetMarshalJSON sets the JSON encoding handler for x.
func (p *Pointer) SetMarshalJSON(fn MarshalPointer) {
	p.marshalJSON = fn
}

// MarshalJSON returns the JSON encoding of x.
func (p *Pointer) MarshalJSON() ([]byte, error) {
	return p.marshalJSON(p.Load())
}
