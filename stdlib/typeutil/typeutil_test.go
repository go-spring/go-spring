/*
 * Copyright 2024 The Go-Spring Authors.
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

package typeutil_test

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"unsafe"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/stdlib/typeutil"
)

func TestIsErrorType(t *testing.T) {
	err := errutil.Explain(nil, "error")
	assert.That(t, typeutil.IsErrorType(reflect.TypeOf(err))).True()

	err = os.ErrClosed
	assert.That(t, typeutil.IsErrorType(reflect.TypeOf(err))).True()

	assert.That(t, typeutil.IsErrorType(nil)).False()
	assert.That(t, typeutil.IsErrorType(reflect.TypeFor[int]())).False()
	assert.That(t, typeutil.IsErrorType(reflect.TypeFor[error]())).True()
}

func TestReturnNothing(t *testing.T) {
	assert.That(t, typeutil.ReturnNothing(reflect.TypeFor[func()]())).True()
	assert.That(t, typeutil.ReturnNothing(reflect.TypeFor[func(key string)]())).True()
	assert.That(t, typeutil.ReturnNothing(reflect.TypeFor[func() string]())).False()
	assert.That(t, typeutil.ReturnNothing(reflect.TypeFor[func(key string, value int)]())).True()
	assert.That(t, typeutil.ReturnNothing(reflect.TypeFor[func() (int, error)]())).False()
}

func TestReturnOnlyError(t *testing.T) {
	assert.That(t, typeutil.ReturnOnlyError(reflect.TypeFor[func() error]())).True()
	assert.That(t, typeutil.ReturnOnlyError(reflect.TypeFor[func(string) error]())).True()
	assert.That(t, typeutil.ReturnOnlyError(reflect.TypeFor[func() (string, error)]())).False()
	assert.That(t, typeutil.ReturnOnlyError(reflect.TypeFor[func()]())).False()
	//nolint:staticcheck // reason: testing ReturnOnlyError with an invalid signature
	assert.That(t, typeutil.ReturnOnlyError(reflect.TypeFor[func() (error, string)]())).False()
	assert.That(t, typeutil.ReturnOnlyError(reflect.TypeFor[func(int, string) error]())).True()
}

// nolint: unused
func fnNoArgs() {}

// nolint: unused
func fnWithArgs(i int) {}

type receiver struct{}

// nolint: unused
func (r *receiver) ptrFnNoArgs() {}

// nolint: unused
func (r *receiver) ptrFnWithArgs(i int) {}

func TestIsConstructor(t *testing.T) {
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[int]())).False()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func()]())).False()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func() string]())).True()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func() *string]())).True()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func() *receiver]())).True()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func() (*receiver, error)]())).True()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func() (bool, *receiver, error)]())).False()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func() (int, error)]())).True()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func(string) *receiver]())).True()
	assert.That(t, typeutil.IsConstructor(reflect.TypeFor[func() (*receiver, *receiver)]())).False()
}

func TestIsPropBindingTarget(t *testing.T) {
	data := []struct {
		i any
		v bool
	}{
		{true, true},                           // Bool
		{int(1), true},                         // Int
		{int8(1), true},                        // Int8
		{int16(1), true},                       // Int16
		{int32(1), true},                       // Int32
		{int64(1), true},                       // Int64
		{uint(1), true},                        // Uint
		{uint8(1), true},                       // Uint8
		{uint16(1), true},                      // Uint16
		{uint32(1), true},                      // Uint32
		{uint64(1), true},                      // Uint64
		{uintptr(0), false},                    // Uintptr
		{float32(1), true},                     // Float32
		{float64(1), true},                     // Float64
		{complex64(1), false},                  // Complex64
		{complex128(1), false},                 // Complex128
		{[1]int{0}, true},                      // Array
		{make(chan struct{}), false},           // Chan
		{func() {}, false},                     // Func
		{reflect.TypeFor[error](), false},      // Interface
		{make(map[int]int), true},              // Map
		{make(map[string]*int), false},         //
		{new(int), false},                      // Ptr
		{new(struct{}), false},                 //
		{[]int{0}, true},                       // Slice
		{[]*int{}, false},                      //
		{"this is a string", true},             // String
		{struct{}{}, true},                     // Struct
		{unsafe.Pointer(new(int)), false},      // UnsafePointer
		{i: reflect.TypeFor[*int](), v: false}, // Pointer type directly
		{i: []string{}, v: true},               // Non-pointer slice
		{i: map[string]int{}, v: true},         // Non-pointer map
	}
	for _, d := range data {
		var typ reflect.Type
		switch i := d.i.(type) {
		case reflect.Type:
			typ = i
		default:
			typ = reflect.TypeOf(i)
		}
		if r := typeutil.IsPropBindingTarget(typ); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
}

func TestIsBeanType(t *testing.T) {
	data := []struct {
		i any
		v bool
	}{
		{true, false},                              // Bool
		{int(1), false},                            // Int
		{int8(1), false},                           // Int8
		{int16(1), false},                          // Int16
		{int32(1), false},                          // Int32
		{int64(1), false},                          // Int64
		{uint(1), false},                           // Uint
		{uint8(1), false},                          // Uint8
		{uint16(1), false},                         // Uint16
		{uint32(1), false},                         // Uint32
		{uint64(1), false},                         // Uint64
		{uintptr(0), false},                        // Uintptr
		{float32(1), false},                        // Float32
		{float64(1), false},                        // Float64
		{complex64(1), false},                      // Complex64
		{complex128(1), false},                     // Complex128
		{[1]int{0}, false},                         // Array
		{make(chan struct{}), true},                // Chan
		{func() {}, true},                          // Func
		{reflect.TypeFor[error](), true},           // Interface
		{make(map[int]int), false},                 // Map
		{make(map[string]*int), false},             //
		{new(int), false},                          //
		{new(struct{}), true},                      //
		{[]int{0}, false},                          // Slice
		{[]*int{}, false},                          //
		{"this is a string", false},                // String
		{struct{}{}, false},                        // Struct
		{unsafe.Pointer(new(int)), false},          // UnsafePointer
		{i: reflect.TypeFor[*struct{}](), v: true}, // Pointer to struct type
		{i: reflect.TypeFor[chan int](), v: true},  // Channel type directly
	}
	for _, d := range data {
		var typ reflect.Type
		switch i := d.i.(type) {
		case reflect.Type:
			typ = i
		default:
			typ = reflect.TypeOf(i)
		}
		if r := typeutil.IsBeanType(typ); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
}

func TestIsBeanInjectionTarget(t *testing.T) {
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[string]())).False()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[*string]())).False()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeOf(errutil.Explain(nil, "abc")))).True()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[[]string]())).False()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[[]*string]())).False()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[[]fmt.Stringer]())).True()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[map[string]string]())).False()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[map[string]*string]())).False()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[map[string]fmt.Stringer]())).True()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[fmt.Stringer]())).True()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[[]error]())).True()
	assert.That(t, typeutil.IsBeanInjectionTarget(reflect.TypeFor[map[string]error]())).True()
	assert.That(t, typeutil.IsBeanInjectionTarget(nil)).False()
}
