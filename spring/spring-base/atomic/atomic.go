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

// Package atomic 封装标准库 atomic 包的操作函数。
package atomic

import (
	"sync/atomic"
	"unsafe"
)

type Int32 struct {
	v int32
}

func NewInt32(val int32) *Int32 {
	return &Int32{v: val}
}

// Add wrapper for atomic.AddInt32.
func (i *Int32) Add(delta int32) (new int32) {
	return atomic.AddInt32(&i.v, delta)
}

// Store wrapper for atomic.StoreInt32.
func (i *Int32) Store(val int32) {
	atomic.StoreInt32(&i.v, val)
}

// Load wrapper for atomic.LoadInt32.
func (i *Int32) Load() (val int32) {
	return atomic.LoadInt32(&i.v)
}

// Swap wrapper for atomic.SwapInt32.
func (i *Int32) Swap(new int32) (old int32) {
	return atomic.SwapInt32(&i.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapInt32.
func (i *Int32) CompareAndSwap(old, new int32) (swapped bool) {
	return atomic.CompareAndSwapInt32(&i.v, old, new)
}

type Int64 struct {
	v int64
}

func NewInt64(val int64) *Int64 {
	return &Int64{v: val}
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

type Uint32 struct {
	v uint32
}

func NewUint32(val uint32) *Uint32 {
	return &Uint32{v: val}
}

// Add wrapper for atomic.AddUint32.
func (u *Uint32) Add(delta uint32) (new uint32) {
	return atomic.AddUint32(&u.v, delta)
}

// Load wrapper for atomic.LoadUint32.
func (u *Uint32) Load() (val uint32) {
	return atomic.LoadUint32(&u.v)
}

// Store wrapper for atomic.StoreUint32.
func (u *Uint32) Store(val uint32) {
	atomic.StoreUint32(&u.v, val)
}

// Swap wrapper for atomic.SwapUint32.
func (u *Uint32) Swap(new uint32) (old uint32) {
	return atomic.SwapUint32(&u.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint32.
func (u *Uint32) CompareAndSwap(old, new uint32) (swapped bool) {
	return atomic.CompareAndSwapUint32(&u.v, old, new)
}

type Uint64 struct {
	v uint64
}

func NewUint64(val uint64) *Uint64 {
	return &Uint64{v: val}
}

// Add wrapper for atomic.AddUint64.
func (u *Uint64) Add(delta uint64) (new uint64) {
	return atomic.AddUint64(&u.v, delta)
}

// Load wrapper for atomic.LoadUint64.
func (u *Uint64) Load() (val uint64) {
	return atomic.LoadUint64(&u.v)
}

// Store wrapper for atomic.StoreUint64.
func (u *Uint64) Store(val uint64) {
	atomic.StoreUint64(&u.v, val)
}

// Swap wrapper for atomic.SwapUint64.
func (u *Uint64) Swap(new uint64) (old uint64) {
	return atomic.SwapUint64(&u.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUint64.
func (u *Uint64) CompareAndSwap(old, new uint64) (swapped bool) {
	return atomic.CompareAndSwapUint64(&u.v, old, new)
}

type Uintptr struct {
	v uintptr
}

func NewUintptr(val uintptr) *Uintptr {
	return &Uintptr{v: val}
}

// Add wrapper for atomic.AddUintptr.
func (u *Uintptr) Add(delta uintptr) (new uintptr) {
	return atomic.AddUintptr(&u.v, delta)
}

// Load wrapper for atomic.LoadUintptr.
func (u *Uintptr) Load() (val uintptr) {
	return atomic.LoadUintptr(&u.v)
}

// Store wrapper for atomic.StoreUintptr.
func (u *Uintptr) Store(val uintptr) {
	atomic.StoreUintptr(&u.v, val)
}

// Swap wrapper for atomic.SwapUintptr.
func (u *Uintptr) Swap(new uintptr) (old uintptr) {
	return atomic.SwapUintptr(&u.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapUintptr.
func (u *Uintptr) CompareAndSwap(old, new uintptr) (swapped bool) {
	return atomic.CompareAndSwapUintptr(&u.v, old, new)
}

type Pointer struct {
	v unsafe.Pointer
}

func NewPointer(val unsafe.Pointer) *Pointer {
	return &Pointer{v: val}
}

// Load wrapper for atomic.LoadPointer.
func (p *Pointer) Load() (val unsafe.Pointer) {
	return atomic.LoadPointer(&p.v)
}

// Store wrapper for atomic.StorePointer.
func (p *Pointer) Store(val unsafe.Pointer) {
	atomic.StorePointer(&p.v, val)
}

// Swap wrapper for atomic.SwapPointer.
func (p *Pointer) Swap(new unsafe.Pointer) (old unsafe.Pointer) {
	return atomic.SwapPointer(&p.v, new)
}

// CompareAndSwap wrapper for atomic.CompareAndSwapPointer.
func (p *Pointer) CompareAndSwap(old, new unsafe.Pointer) (swapped bool) {
	return atomic.CompareAndSwapPointer(&p.v, old, new)
}

type Value = atomic.Value
