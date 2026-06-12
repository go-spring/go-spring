/*
 * Copyright 2025 The Go-Spring Authors.
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

package patchutil

import (
	"reflect"
	"unsafe"
)

const (
	flagStickyRO = 1 << 5
	flagEmbedRO  = 1 << 6
	flagRO       = flagStickyRO | flagEmbedRO
)

// PatchValue enables assignment to an unexported struct field by clearing the internal read-only flags.
//
// It takes a reflect.Value and returns the same Value after patching. This allows
// the caller to call Set on a Value that originally pointed to an unexported field.
//
// WARNING:
//   - This function relies on unsafe access to reflect.Value's internal 'flag' field.
//   - It is highly version-dependent and may break in future Go releases.
//   - Using this in production code is unsafe and may cause undefined behavior.
//   - Only use this in controlled internal tools where performance or testing requires it.
func PatchValue(v reflect.Value) reflect.Value {
	rv := reflect.ValueOf(&v)
	flag := rv.Elem().FieldByName("flag")
	ptrFlag := (*uintptr)(unsafe.Pointer(flag.UnsafeAddr()))
	*ptrFlag = *ptrFlag &^ flagRO
	return v
}
