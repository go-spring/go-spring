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

package gsutil_test

import (
	"fmt"
	"reflect"
	"testing"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-core/gs/gsutil"
	pkg1 "github.com/go-spring/spring-core/gs/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/gs/testdata/pkg/foo"
)

// golang 允许不同的路径下存在相同的包，而且允许存在相同的包。
type SamePkg struct{}

func (p *SamePkg) Package() {
	fmt.Println("github.com/go-spring/spring-core/gs/gsutil/gsutil_test.SamePkg")
}

func TestTypeName(t *testing.T) {

	data := map[reflect.Type]struct {
		typeName string
		baseName string
	}{
		reflect.TypeOf(3):                 {"int", "int"},
		reflect.TypeOf(new(int)):          {"int", "*int"},
		reflect.TypeOf(make([]int, 0)):    {"int", "[]int"},
		reflect.TypeOf(&[]int{3}):         {"int", "*[]int"},
		reflect.TypeOf(make([]*int, 0)):   {"int", "[]*int"},
		reflect.TypeOf(make([][]int, 0)):  {"int", "[][]int"},
		reflect.TypeOf(make(map[int]int)): {"map[int]int", "map[int]int"},

		reflect.TypeOf(int8(3)):             {"int8", "int8"},
		reflect.TypeOf(new(int8)):           {"int8", "*int8"},
		reflect.TypeOf(make([]int8, 0)):     {"int8", "[]int8"},
		reflect.TypeOf(&[]int8{3}):          {"int8", "*[]int8"},
		reflect.TypeOf(make(map[int8]int8)): {"map[int8]int8", "map[int8]int8"},

		reflect.TypeOf(int16(3)):              {"int16", "int16"},
		reflect.TypeOf(new(int16)):            {"int16", "*int16"},
		reflect.TypeOf(make([]int16, 0)):      {"int16", "[]int16"},
		reflect.TypeOf(&[]int16{3}):           {"int16", "*[]int16"},
		reflect.TypeOf(make(map[int16]int16)): {"map[int16]int16", "map[int16]int16"},

		reflect.TypeOf(int32(3)):              {"int32", "int32"},
		reflect.TypeOf(new(int32)):            {"int32", "*int32"},
		reflect.TypeOf(make([]int32, 0)):      {"int32", "[]int32"},
		reflect.TypeOf(&[]int32{3}):           {"int32", "*[]int32"},
		reflect.TypeOf(make(map[int32]int32)): {"map[int32]int32", "map[int32]int32"},

		reflect.TypeOf(int64(3)):              {"int64", "int64"},
		reflect.TypeOf(new(int64)):            {"int64", "*int64"},
		reflect.TypeOf(make([]int64, 0)):      {"int64", "[]int64"},
		reflect.TypeOf(&[]int64{3}):           {"int64", "*[]int64"},
		reflect.TypeOf(make(map[int64]int64)): {"map[int64]int64", "map[int64]int64"},

		reflect.TypeOf(uint(3)):             {"uint", "uint"},
		reflect.TypeOf(new(uint)):           {"uint", "*uint"},
		reflect.TypeOf(make([]uint, 0)):     {"uint", "[]uint"},
		reflect.TypeOf(&[]uint{3}):          {"uint", "*[]uint"},
		reflect.TypeOf(make(map[uint]uint)): {"map[uint]uint", "map[uint]uint"},

		reflect.TypeOf(uint8(3)):              {"uint8", "uint8"},
		reflect.TypeOf(new(uint8)):            {"uint8", "*uint8"},
		reflect.TypeOf(make([]uint8, 0)):      {"uint8", "[]uint8"},
		reflect.TypeOf(&[]uint8{3}):           {"uint8", "*[]uint8"},
		reflect.TypeOf(make(map[uint8]uint8)): {"map[uint8]uint8", "map[uint8]uint8"},

		reflect.TypeOf(uint16(3)):               {"uint16", "uint16"},
		reflect.TypeOf(new(uint16)):             {"uint16", "*uint16"},
		reflect.TypeOf(make([]uint16, 0)):       {"uint16", "[]uint16"},
		reflect.TypeOf(&[]uint16{3}):            {"uint16", "*[]uint16"},
		reflect.TypeOf(make(map[uint16]uint16)): {"map[uint16]uint16", "map[uint16]uint16"},

		reflect.TypeOf(uint32(3)):               {"uint32", "uint32"},
		reflect.TypeOf(new(uint32)):             {"uint32", "*uint32"},
		reflect.TypeOf(make([]uint32, 0)):       {"uint32", "[]uint32"},
		reflect.TypeOf(&[]uint32{3}):            {"uint32", "*[]uint32"},
		reflect.TypeOf(make(map[uint32]uint32)): {"map[uint32]uint32", "map[uint32]uint32"},

		reflect.TypeOf(uint64(3)):               {"uint64", "uint64"},
		reflect.TypeOf(new(uint64)):             {"uint64", "*uint64"},
		reflect.TypeOf(make([]uint64, 0)):       {"uint64", "[]uint64"},
		reflect.TypeOf(&[]uint64{3}):            {"uint64", "*[]uint64"},
		reflect.TypeOf(make(map[uint64]uint64)): {"map[uint64]uint64", "map[uint64]uint64"},

		reflect.TypeOf(true):                {"bool", "bool"},
		reflect.TypeOf(new(bool)):           {"bool", "*bool"},
		reflect.TypeOf(make([]bool, 0)):     {"bool", "[]bool"},
		reflect.TypeOf(&[]bool{true}):       {"bool", "*[]bool"},
		reflect.TypeOf(make(map[bool]bool)): {"map[bool]bool", "map[bool]bool"},

		reflect.TypeOf(float32(3)):                {"float32", "float32"},
		reflect.TypeOf(new(float32)):              {"float32", "*float32"},
		reflect.TypeOf(make([]float32, 0)):        {"float32", "[]float32"},
		reflect.TypeOf(&[]float32{3}):             {"float32", "*[]float32"},
		reflect.TypeOf(make(map[float32]float32)): {"map[float32]float32", "map[float32]float32"},

		reflect.TypeOf(float64(3)):                {"float64", "float64"},
		reflect.TypeOf(new(float64)):              {"float64", "*float64"},
		reflect.TypeOf(make([]float64, 0)):        {"float64", "[]float64"},
		reflect.TypeOf(&[]float64{3}):             {"float64", "*[]float64"},
		reflect.TypeOf(make(map[float64]float64)): {"map[float64]float64", "map[float64]float64"},

		reflect.TypeOf(complex64(3)):                  {"complex64", "complex64"},
		reflect.TypeOf(new(complex64)):                {"complex64", "*complex64"},
		reflect.TypeOf(make([]complex64, 0)):          {"complex64", "[]complex64"},
		reflect.TypeOf(&[]complex64{3}):               {"complex64", "*[]complex64"},
		reflect.TypeOf(make(map[complex64]complex64)): {"map[complex64]complex64", "map[complex64]complex64"},

		reflect.TypeOf(complex128(3)):                   {"complex128", "complex128"},
		reflect.TypeOf(new(complex128)):                 {"complex128", "*complex128"},
		reflect.TypeOf(make([]complex128, 0)):           {"complex128", "[]complex128"},
		reflect.TypeOf(&[]complex128{3}):                {"complex128", "*[]complex128"},
		reflect.TypeOf(make(map[complex128]complex128)): {"map[complex128]complex128", "map[complex128]complex128"},

		reflect.TypeOf(make(chan int)):      {"chan int", "chan int"},
		reflect.TypeOf(make(chan struct{})): {"chan struct {}", "chan struct {}"},
		reflect.TypeOf(func() {}):           {"func()", "func()"},

		reflect.TypeOf((*error)(nil)).Elem():        {"error", "error"},
		reflect.TypeOf((*fmt.Stringer)(nil)).Elem(): {"fmt/fmt.Stringer", "fmt.Stringer"},

		reflect.TypeOf("string"):                {"string", "string"},
		reflect.TypeOf(new(string)):             {"string", "*string"},
		reflect.TypeOf(make([]string, 0)):       {"string", "[]string"},
		reflect.TypeOf(&[]string{"string"}):     {"string", "*[]string"},
		reflect.TypeOf(make(map[string]string)): {"map[string]string", "map[string]string"},

		reflect.TypeOf(pkg1.SamePkg{}):             {"github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "pkg.SamePkg"},
		reflect.TypeOf(new(pkg1.SamePkg)):          {"github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "*pkg.SamePkg"},
		reflect.TypeOf(make([]pkg1.SamePkg, 0)):    {"github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "[]pkg.SamePkg"},
		reflect.TypeOf(&[]pkg1.SamePkg{}):          {"github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "*[]pkg.SamePkg"},
		reflect.TypeOf(make(map[int]pkg1.SamePkg)): {"map[int]pkg.SamePkg", "map[int]pkg.SamePkg"},

		reflect.TypeOf(pkg2.SamePkg{}):             {"github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "pkg.SamePkg"},
		reflect.TypeOf(new(pkg2.SamePkg)):          {"github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg"},
		reflect.TypeOf(make([]pkg2.SamePkg, 0)):    {"github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "[]pkg.SamePkg"},
		reflect.TypeOf(&[]pkg2.SamePkg{}):          {"github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "*[]pkg.SamePkg"},
		reflect.TypeOf(make(map[int]pkg2.SamePkg)): {"map[int]pkg.SamePkg", "map[int]pkg.SamePkg"},

		reflect.TypeOf(SamePkg{}):             {"github.com/go-spring/spring-core/gs/gsutil/gsutil_test.SamePkg", "gsutil_test.SamePkg"},
		reflect.TypeOf(new(SamePkg)):          {"github.com/go-spring/spring-core/gs/gsutil/gsutil_test.SamePkg", "*gsutil_test.SamePkg"},
		reflect.TypeOf(make([]SamePkg, 0)):    {"github.com/go-spring/spring-core/gs/gsutil/gsutil_test.SamePkg", "[]gsutil_test.SamePkg"},
		reflect.TypeOf(&[]SamePkg{}):          {"github.com/go-spring/spring-core/gs/gsutil/gsutil_test.SamePkg", "*[]gsutil_test.SamePkg"},
		reflect.TypeOf(make(map[int]SamePkg)): {"map[int]gsutil_test.SamePkg", "map[int]gsutil_test.SamePkg"},
	}

	for typ, v := range data {
		typeName := gsutil.TypeName(typ)
		assert.Equal(t, typeName, v.typeName)
		assert.Equal(t, typ.String(), v.baseName)
	}

	i := 3
	iPtr := &i
	iPtrPtr := &iPtr
	iPtrPtrPtr := &iPtrPtr
	typ := reflect.TypeOf(iPtrPtrPtr)
	typeName := gsutil.TypeName(typ)
	assert.Equal(t, typeName, "int")
	assert.Equal(t, typ.String(), "***int")
}

func TestIsBeanType(t *testing.T) {

	data := []struct {
		i interface{}
		v bool
	}{
		{true, false},                                // Bool
		{int(1), false},                              // Int
		{int8(1), false},                             // Int8
		{int16(1), false},                            // Int16
		{int32(1), false},                            // Int32
		{int64(1), false},                            // Int64
		{uint(1), false},                             // Uint
		{uint8(1), false},                            // Uint8
		{uint16(1), false},                           // Uint16
		{uint32(1), false},                           // Uint32
		{uint64(1), false},                           // Uint64
		{uintptr(0), false},                          // Uintptr
		{float32(1), false},                          // Float32
		{float64(1), false},                          // Float64
		{complex64(1), false},                        // Complex64
		{complex128(1), false},                       // Complex128
		{[1]int{0}, false},                           // Array
		{make(chan struct{}), true},                  // Chan
		{func() {}, true},                            // Func
		{reflect.TypeOf((*error)(nil)).Elem(), true}, // Interface
		{make(map[int]int), false},                   // Map
		{make(map[string]*int), false},               //
		{new(int), true},                             // Ptr
		{new(struct{}), true},                        //
		{[]int{0}, false},                            // Slice
		{[]*int{}, false},                            //
		{"this is a string", false},                  // String
		{struct{}{}, false},                          // Struct
		{unsafe.Pointer(new(int)), false},            // UnsafePointer
	}

	for _, d := range data {
		var typ reflect.Type
		switch i := d.i.(type) {
		case reflect.Type:
			typ = i
		default:
			typ = reflect.TypeOf(i)
		}
		if r := gsutil.IsBeanType(typ); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
}
