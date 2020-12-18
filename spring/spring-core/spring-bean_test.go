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

package SpringCore_test

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"sync"
	"testing"
	"unsafe"

	"github.com/go-spring/spring-core"
	pkg1 "github.com/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/testdata/pkg/foo"
	"github.com/go-spring/spring-utils"
)

type errorString struct {
	s string
}

func (e *errorString) Error() string {
	return e.s
}

func TestRef(t *testing.T) {
	// 基本测试方法，首先创建第一个目标，然后赋值给第二个目标，这时候如果修改第一个目标
	// 内部的状态，如果第二个值也能反映出这种修改，那么它是引用类型，否则它就是值类型。

	// valType // Bool
	var o1 bool
	o1 = true
	o2 := o1
	o1 = false
	if o2 == false {
		fmt.Printf("%v==%v bool is ref type\n", o1, o2)
	} else {
		fmt.Printf("%v!=%v bool is val type\n", o1, o2)
	}

	// valType // Int
	// valType // Int8
	// valType // Int16
	// valType // Int32
	// valType // Int64
	// valType // Uint
	// valType // Uint8
	// valType // Uint16
	// valType // Uint32
	// valType // Uint64
	// valType // Float32
	// valType // Float64
	var i1 int
	i1 = 3
	i2 := i1
	i1 = 5
	if i2 == 5 {
		fmt.Printf("%v==%v int is ref type\n", i1, i2)
	} else {
		fmt.Printf("%v!=%v int is val type\n", i1, i2)
	}

	// valType // Complex64
	// valType // Complex128
	var c1 complex64
	c1 = complex(1, 1)
	c2 := c1
	c1 = complex(0, 0)
	if real(c2) == 0 {
		fmt.Printf("%v==%v complex64 is ref type\n", c1, c2)
	} else {
		fmt.Printf("%v!=%v complex64 is val type\n", c1, c2)
	}

	// valType // Array
	var arr1 [1]int
	arr1[0] = 3
	arr2 := arr1
	arr1[0] = 5
	if arr2[0] == 5 {
		fmt.Printf("%v==%v array is ref type\n", arr1, arr2)
	} else {
		fmt.Printf("%v!=%v array is val type\n", arr1, arr2)
	}

	// refType // Chan
	var wg sync.WaitGroup
	ch1 := make(chan struct{})
	ch2 := ch1
	wg.Add(1)
	go func() {
		<-ch2
		wg.Done()
	}()
	ch1 <- struct{}{}
	wg.Wait()

	// refType // Func
	ret := new(int)
	f1 := func() int { return *ret }
	f2 := f1
	*ret = 3
	if f2() == 3 {
		fmt.Printf("%p==%p func is ref type\n", f1, f2)
	} else {
		fmt.Printf("%p!=%p func is val type\n", f1, f2)
	}

	// refType // Interface
	e1 := &errorString{"ok"}
	e2 := e1
	e1.s = "no"
	if e2.Error() == "no" {
		fmt.Printf("%v==%v error is ref type\n", e1, e2)
	} else {
		fmt.Printf("%v!=%v error is val type\n", e1, e2)
	}

	// refType // Map
	m1 := make(map[int]string)
	m1[1] = "1"
	m2 := m1
	m1[1] = "3"
	if m2[1] == "3" {
		fmt.Printf("%v==%v map is ref type\n", m1, m2)
	} else {
		fmt.Printf("%v!=%v map is val type\n", m1, m2)
	}

	type Int struct {
		i int
	}

	// refType // Ptr
	ptr1 := &Int{1}
	ptr2 := ptr1
	ptr1.i = 3
	if ptr2.i == 3 {
		fmt.Printf("%v==%v ptr is ref type\n", ptr1, ptr2)
	} else {
		fmt.Printf("%v!=%v ptr is val type\n", ptr1, ptr2)
	}

	// refType // Slice
	s1 := make([]int, 1)
	s1[0] = 1
	s2 := s1
	s1[0] = 3
	if s2[0] == 3 {
		fmt.Printf("%v==%v slice is ref type\n", s1, s2)
	} else {
		fmt.Printf("%v!=%v slice is val type\n", s1, s2)
	}

	// valType // String
	var str1 string
	str1 = "1"
	str2 := str1
	str1 = "3"
	if str2 == "3" {
		fmt.Printf("%v==%v string is ref type\n", str1, str2)
	} else {
		fmt.Printf("%v!=%v string is val type\n", str1, str2)
	}

	// valType // Struct
	t1 := Int{1}
	t2 := t1
	t1.i = 3
	if t2.i == 3 {
		fmt.Printf("%v==%v struct is ref type\n", t1, t2)
	} else {
		fmt.Printf("%v!=%v struct is val type\n", t1, t2)
	}
}

func TestIsRefType(t *testing.T) {

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
		{make(map[int]int), true},                    // Map
		{new(int), true},                             // Ptr
		{new(struct{}), true},                        // Ptr
		{[]int{0}, true},                             // Slice
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
		if r := SpringCore.IsRefType(typ.Kind()); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
}

func TestIsValueType(t *testing.T) {

	data := []struct {
		i interface{}
		v bool
	}{
		{true, true},                 // Bool
		{int(1), true},               // Int
		{int8(1), true},              // Int8
		{int16(1), true},             // Int16
		{int32(1), true},             // Int32
		{int64(1), true},             // Int64
		{uint(1), true},              // Uint
		{uint8(1), true},             // Uint8
		{uint16(1), true},            // Uint16
		{uint32(1), true},            // Uint32
		{uint64(1), true},            // Uint64
		{uintptr(0), false},          // Uintptr
		{float32(1), true},           // Float32
		{float64(1), true},           // Float64
		{complex64(1), true},         // Complex64
		{complex128(1), true},        // Complex128
		{[1]int{0}, true},            // Array
		{make(chan struct{}), false}, // Chan
		{func() {}, false},           // Func
		{reflect.TypeOf((*error)(nil)).Elem(), false}, // Interface
		{make(map[int]int), false},                    // Map
		{new(int), false},                             // Ptr
		{new(struct{}), false},                        // Ptr
		{[]int{0}, false},                             // Slice
		{"this is a string", true},                    // String
		{struct{}{}, true},                            // Struct
		{unsafe.Pointer(new(int)), false},             // UnsafePointer
	}

	for _, d := range data {
		var typ reflect.Type
		switch i := d.i.(type) {
		case reflect.Type:
			typ = i
		default:
			typ = reflect.TypeOf(i)
		}
		if r := SpringCore.IsValueType(typ.Kind()); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
}

func TestTypeName(t *testing.T) {

	t.Run("nil", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {
			SpringCore.TypeName(reflect.TypeOf(nil))
		}, "shouldn't be nil")
	})

	t.Run("type", func(t *testing.T) {

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

			reflect.TypeOf(pkg1.SamePkg{}):             {"github.com/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "pkg.SamePkg"},
			reflect.TypeOf(new(pkg1.SamePkg)):          {"github.com/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "*pkg.SamePkg"},
			reflect.TypeOf(make([]pkg1.SamePkg, 0)):    {"github.com/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "[]pkg.SamePkg"},
			reflect.TypeOf(&[]pkg1.SamePkg{}):          {"github.com/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "*[]pkg.SamePkg"},
			reflect.TypeOf(make(map[int]pkg1.SamePkg)): {"map[int]pkg.SamePkg", "map[int]pkg.SamePkg"},

			reflect.TypeOf(pkg2.SamePkg{}):             {"github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "pkg.SamePkg"},
			reflect.TypeOf(new(pkg2.SamePkg)):          {"github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg"},
			reflect.TypeOf(make([]pkg2.SamePkg, 0)):    {"github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "[]pkg.SamePkg"},
			reflect.TypeOf(&[]pkg2.SamePkg{}):          {"github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*[]pkg.SamePkg"},
			reflect.TypeOf(make(map[int]pkg2.SamePkg)): {"map[int]pkg.SamePkg", "map[int]pkg.SamePkg"},
		}

		for typ, v := range data {
			typeName := SpringCore.TypeName(typ)
			SpringUtils.AssertEqual(t, typeName, v.typeName)
			SpringUtils.AssertEqual(t, typ.String(), v.baseName)
		}

		i := 3
		iPtr := &i
		iPtrPtr := &iPtr
		iPtrPtrPtr := &iPtrPtr
		typ := reflect.TypeOf(iPtrPtrPtr)
		typeName := SpringCore.TypeName(typ)
		SpringUtils.AssertEqual(t, typeName, "int")
		SpringUtils.AssertEqual(t, typ.String(), "***int")
	})
}

func TestIsFuncBeanType(t *testing.T) {

	type S struct{}
	type OptionFunc func(*S)

	data := map[reflect.Type]bool{
		reflect.TypeOf((func())(nil)):            false,
		reflect.TypeOf((func(int))(nil)):         false,
		reflect.TypeOf((func(int, int))(nil)):    false,
		reflect.TypeOf((func(int, ...int))(nil)): false,

		reflect.TypeOf((func() int)(nil)):          true,
		reflect.TypeOf((func() (int, int))(nil)):   false,
		reflect.TypeOf((func() (int, error))(nil)): true,

		reflect.TypeOf((func(int) int)(nil)):         true,
		reflect.TypeOf((func(int, int) int)(nil)):    true,
		reflect.TypeOf((func(int, ...int) int)(nil)): true,

		reflect.TypeOf((func(int) (int, error))(nil)):         true,
		reflect.TypeOf((func(int, int) (int, error))(nil)):    true,
		reflect.TypeOf((func(int, ...int) (int, error))(nil)): true,

		reflect.TypeOf((func() S)(nil)):          true,
		reflect.TypeOf((func() *S)(nil)):         true,
		reflect.TypeOf((func() (S, error))(nil)): true,

		reflect.TypeOf((func(OptionFunc) (*S, error))(nil)):    true,
		reflect.TypeOf((func(...OptionFunc) (*S, error))(nil)): true,
	}

	for k, v := range data {
		ok := SpringCore.IsFuncBeanType(k)
		SpringUtils.AssertEqual(t, ok, v)
	}
}

func TestParseSingletonTag(t *testing.T) {

	data := map[string]SpringCore.SingletonTag{
		"[]":     {"", "[]", false},
		"[]?":    {"", "[]", true},
		"i":      {"", "i", false},
		"i?":     {"", "i", true},
		":i":     {"", "i", false},
		":i?":    {"", "i", true},
		"int:i":  {"int", "i", false},
		"int:i?": {"int", "i", true},
		"int:":   {"int", "", false},
		"int:?":  {"int", "", true},
	}

	for k, v := range data {
		tag := SpringCore.ParseSingletonTag(k)
		SpringUtils.AssertEqual(t, tag, v)
	}
}

func TestParseBeanTag(t *testing.T) {

	data := map[string]SpringCore.CollectionTag{
		"[]":  {[]SpringCore.SingletonTag{}, false},
		"[]?": {[]SpringCore.SingletonTag{}, true},
	}

	for k, v := range data {
		tag := SpringCore.ParseCollectionTag(k)
		SpringUtils.AssertEqual(t, tag, v)
	}
}

func TestBeanDefinition_Match(t *testing.T) {

	data := []struct {
		bd       *SpringCore.BeanDefinition
		typeName string
		beanName string
		expect   bool
	}{
		{SpringCore.ObjectBean(new(int)), "int", "*int", true},
		{SpringCore.ObjectBean(new(int)), "", "*int", true},
		{SpringCore.ObjectBean(new(int)), "int", "", true},
		{SpringCore.ObjectBean(new(int)).WithName("i"), "int", "i", true},
		{SpringCore.ObjectBean(new(int)).WithName("i"), "", "i", true},
		{SpringCore.ObjectBean(new(int)).WithName("i"), "int", "", true},
		{SpringCore.ObjectBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg", true},
		{SpringCore.ObjectBean(new(pkg2.SamePkg)), "", "*pkg.SamePkg", true},
		{SpringCore.ObjectBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "", true},
		{SpringCore.ObjectBean(new(pkg2.SamePkg)).WithName("pkg2"), "github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
		{SpringCore.ObjectBean(new(pkg2.SamePkg)).WithName("pkg2"), "", "pkg2", true},
		{SpringCore.ObjectBean(new(pkg2.SamePkg)).WithName("pkg2"), "github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
	}

	for i, s := range data {
		if ok := s.bd.Match(s.typeName, s.beanName); ok != s.expect {
			t.Errorf("%d expect %v but %v", i, s.expect, ok)
		}
	}
}

func TestObjectBean(t *testing.T) {

	t.Run("bean can't be nil", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			SpringCore.ObjectBean(nil)
		}, "bean can't be nil")

		SpringUtils.AssertPanic(t, func() {
			var i *int
			SpringCore.ObjectBean(i)
		}, "bean can't be nil")

		SpringUtils.AssertPanic(t, func() {
			var m map[string]string
			SpringCore.ObjectBean(m)
		}, "bean can't be nil")
	})

	t.Run("bean must be ref type", func(t *testing.T) {

		data := []func(){
			func() { SpringCore.ObjectBean([...]int{0}) },
			func() { SpringCore.ObjectBean(false) },
			func() { SpringCore.ObjectBean(3) },
			func() { SpringCore.ObjectBean("3") },
			func() { SpringCore.ObjectBean(BeanZero{}) },
			func() { SpringCore.ObjectBean(pkg2.SamePkg{}) },
		}

		for _, fn := range data {
			SpringUtils.AssertPanic(t, fn, "bean must be ref type")
		}
	})

	t.Run("valid bean", func(t *testing.T) {
		SpringCore.ObjectBean(make(chan int))
		SpringCore.ObjectBean(func() {})
		SpringCore.ObjectBean(make(map[string]int))
		SpringCore.ObjectBean(new(int))
		SpringCore.ObjectBean(&BeanZero{})
		SpringCore.ObjectBean(make([]int, 0))
	})

	t.Run("check name && typename", func(t *testing.T) {

		data := map[*SpringCore.BeanDefinition]struct {
			name     string
			typeName string
		}{
			SpringCore.ObjectBean(io.Writer(os.Stdout)): {
				"*os.File", "os/os.File",
			},

			SpringCore.ObjectBean(newHistoryTeacher("")): {
				"*SpringCore_test.historyTeacher",
				"github.com/go-spring/spring-core_test/SpringCore_test.historyTeacher",
			},

			SpringCore.ObjectBean(new(int)): {
				"*int", "int",
			},

			SpringCore.ObjectBean(new(int)).WithName("i"): {
				"i", "int",
			},

			SpringCore.ObjectBean(new(pkg2.SamePkg)): {
				"*pkg.SamePkg",
				"github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg",
			},

			SpringCore.ObjectBean(new(pkg2.SamePkg)).WithName("pkg2"): {
				"pkg2",
				"github.com/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg",
			},
		}

		for bd, v := range data {
			SpringUtils.AssertEqual(t, bd.Name(), v.name)
			SpringUtils.AssertEqual(t, bd.TypeName(), v.typeName)
		}
	})
}

type Teacher interface {
	Course() string
}

type historyTeacher struct {
	name string
}

func newHistoryTeacher(name string) *historyTeacher {
	return &historyTeacher{
		name: name,
	}
}

func (t *historyTeacher) Course() string {
	return "history"
}

type Student struct {
	Teacher Teacher
	Room    string
}

// 入参可以进行注入或者属性绑定，返回值可以是 struct、map、slice、func 等。
func NewStudent(teacher Teacher, room string) Student {
	return Student{
		Teacher: teacher,
		Room:    room,
	}
}

// 入参可以进行注入或者属性绑定，返回值可以是 struct、map、slice、func 等。
func NewPtrStudent(teacher Teacher, room string) *Student {
	return &Student{
		Teacher: teacher,
		Room:    room,
	}
}

func TestConstructorBean(t *testing.T) {

	bd := SpringCore.ConstructorBean(NewStudent)
	SpringUtils.AssertEqual(t, bd.Type().String(), "*SpringCore_test.Student")

	bd = SpringCore.ConstructorBean(NewPtrStudent)
	SpringUtils.AssertEqual(t, bd.Type().String(), "*SpringCore_test.Student")

	mapFn := func() map[int]string { return make(map[int]string) }
	bd = SpringCore.ConstructorBean(mapFn)
	SpringUtils.AssertEqual(t, bd.Type().String(), "map[int]string")

	sliceFn := func() []int { return make([]int, 1) }
	bd = SpringCore.ConstructorBean(sliceFn)
	SpringUtils.AssertEqual(t, bd.Type().String(), "[]int")

	funcFn := func() func(int) { return nil }
	bd = SpringCore.ConstructorBean(funcFn)
	SpringUtils.AssertEqual(t, bd.Type().String(), "func(int)")

	intFn := func() int { return 0 }
	bd = SpringCore.ConstructorBean(intFn)
	SpringUtils.AssertEqual(t, bd.Type().String(), "*int")
	SpringUtils.AssertEqual(t, bd.Value().Type().String(), "*int")

	interfaceFn := func(name string) Teacher { return newHistoryTeacher(name) }
	bd = SpringCore.ConstructorBean(interfaceFn)
	SpringUtils.AssertEqual(t, bd.Type().String(), "SpringCore_test.Teacher")
	SpringUtils.AssertEqual(t, bd.Value().Type().String(), "SpringCore_test.Teacher")

	SpringUtils.AssertPanic(t, func() {
		bd = SpringCore.ConstructorBean(func() (*int, *int) { return nil, nil })
		SpringUtils.AssertEqual(t, bd.Type().String(), "*int")
	}, "func bean must be func\\(...\\)bean or func\\(...\\)\\(bean, error\\)")

	bd = SpringCore.ConstructorBean(func() (*int, error) { return nil, nil })
	SpringUtils.AssertEqual(t, bd.Type().String(), "*int")
}

func TestValue(t *testing.T) {

	{
		var i int // 默认值
		v := reflect.ValueOf(i)
		// int 0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		i = 3
		v = reflect.ValueOf(i)
		// int 3 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var pi *int // 未赋值
		v = reflect.ValueOf(pi)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		pi = &i
		v = reflect.ValueOf(pi)
		// ptr 0xc0000a4e60 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var a [3]int // 内存已分配
		v := reflect.ValueOf(a)
		// array [0 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		a = [3]int{0, 0, 0} // 全零值
		v = reflect.ValueOf(a)
		// array [0 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		a = [3]int{1, 0, 0} // 非全零值
		v = reflect.ValueOf(a)
		// array [1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var pa *[3]int // 未赋值
		v = reflect.ValueOf(pa)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		pa = &a
		v = reflect.ValueOf(pa)
		// ptr &[1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var c chan struct{} // 未赋值
		v := reflect.ValueOf(c)
		// chan <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		c = make(chan struct{})
		v = reflect.ValueOf(c)
		// chan 0xc000086360 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var pc *chan struct{} // 未赋值
		v = reflect.ValueOf(pc)
		// chan <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		pc = &c
		v = reflect.ValueOf(pc)
		// ptr 0xc0000a00d8 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var f func() // 未赋值
		v := reflect.ValueOf(f)
		// func <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		f = func() {}
		v = reflect.ValueOf(f)
		// func 0x16d8810 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var pf *func() // 未赋值
		v = reflect.ValueOf(pf)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		pf = &f
		v = reflect.ValueOf(pf)
		// ptr 0xc0000a00e0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var m map[string]string // 未赋值
		v := reflect.ValueOf(m)
		// map map[] true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		m = map[string]string{}
		v = reflect.ValueOf(m)
		// map map[] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		m = map[string]string{"a": "1"}
		v = reflect.ValueOf(m)
		// map map[a:1] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var pm *map[string]string // 未赋值
		v = reflect.ValueOf(pm)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		pm = &m
		v = reflect.ValueOf(pm)
		// ptr &map[a:1] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var b []int // 未赋值
		v := reflect.ValueOf(b)
		// slice [] true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		b = []int{}
		v = reflect.ValueOf(b)
		// slice [] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		b = []int{0, 0}
		v = reflect.ValueOf(b)
		// slice [0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		b = []int{1, 0, 0}
		v = reflect.ValueOf(b)
		// slice [1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var pb *[]int // 未赋值
		v = reflect.ValueOf(pb)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		pb = &b
		v = reflect.ValueOf(pb)
		// ptr &[1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var s string // 默认值
		v := reflect.ValueOf(s)
		// string  true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		s = "s"
		v = reflect.ValueOf(s)
		// string s true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var ps *string // 未赋值
		v = reflect.ValueOf(ps)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		ps = &s
		v = reflect.ValueOf(ps)
		// ptr 0xc0000974f0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var st struct{} // 默认值
		v := reflect.ValueOf(st)
		// struct {} true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		var pst *struct{} // 未赋值
		v = reflect.ValueOf(pst)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		pst = &st
		v = reflect.ValueOf(pst)
		// ptr &{} true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}

	{
		var e error
		v := reflect.ValueOf(e)
		// invalid <invalid reflect.Value> false false ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))

		e = fmt.Errorf("e")
		v = reflect.ValueOf(e)
		// ptr e true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringUtils.IsNil(v))
	}
}
