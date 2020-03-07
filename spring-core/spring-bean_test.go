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

	"github.com/go-spring/go-spring/spring-core"
	pkg1 "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
	"github.com/magiconair/properties/assert"
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
		fmt.Println("bool is ref type")
	} else {
		fmt.Println("bool is val type")
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
		fmt.Println("int is ref type")
	} else {
		fmt.Println("int is val type")
	}

	// valType // Complex64
	// valType // Complex128
	var c1 complex64
	c1 = complex(1, 1)
	c2 := c1
	c1 = complex(0, 0)
	if real(c2) == 0 {
		fmt.Println("complex64 is ref type")
	} else {
		fmt.Println("complex64 is val type")
	}

	// valType // Array
	var arr1 [1]int
	arr1[0] = 3
	arr2 := arr1
	arr1[0] = 5
	if arr2[0] == 5 {
		fmt.Println("array is ref type")
	} else {
		fmt.Println("array is val type")
	}

	// refType // Chan
	var wg sync.WaitGroup
	var ch1 chan struct{}
	ch1 = make(chan struct{})
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
	var f1 func() int
	f1 = func() int { return *ret }
	f2 := f1
	*ret = 3
	if f2() == 3 {
		fmt.Println("func is ref type")
	} else {
		fmt.Println("func is val type")
	}

	// refType // Interface
	var e1 error
	e1 = &errorString{"ok"}
	e2 := e1
	(e1.(*errorString)).s = "no"
	if e2.Error() == "no" {
		fmt.Println("error is ref type")
	} else {
		fmt.Println("error is val type")
	}

	// refType // Map
	m1 := make(map[int]string)
	m1[1] = "1"
	m2 := m1
	m1[1] = "3"
	if m2[1] == "3" {
		fmt.Println("map is ref type")
	} else {
		fmt.Println("map is val type")
	}

	type Int struct {
		i int
	}

	// refType // Ptr
	var ptr1 *Int
	ptr1 = &Int{1}
	ptr2 := ptr1
	ptr1.i = 3
	if ptr2.i == 3 {
		fmt.Println("ptr is ref type")
	} else {
		fmt.Println("ptr is val type")
	}

	// refType // Slice
	s1 := make([]int, 1)
	s1[0] = 1
	s2 := s1
	s1[0] = 3
	if s2[0] == 3 {
		fmt.Println("slice is ref type")
	} else {
		fmt.Println("slice is val type")
	}

	// valType // String
	var str1 string
	str1 = "1"
	str2 := str1
	str1 = "3"
	if str2 == "3" {
		fmt.Println("string is ref type")
	} else {
		fmt.Println("string is val type")
	}

	// valType // Struct
	var t1 Int
	t1 = Int{1}
	t2 := t1
	t1.i = 3
	if t2.i == 3 {
		fmt.Println("struct is ref type")
	} else {
		fmt.Println("struct is val type")
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
		assert.Panic(t, func() {
			SpringCore.TypeName(reflect.TypeOf(nil))
		}, "type shouldn't be nil")
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

			reflect.TypeOf(pkg1.SamePkg{}):             {"github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "pkg.SamePkg"},
			reflect.TypeOf(new(pkg1.SamePkg)):          {"github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "*pkg.SamePkg"},
			reflect.TypeOf(make([]pkg1.SamePkg, 0)):    {"github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "[]pkg.SamePkg"},
			reflect.TypeOf(&[]pkg1.SamePkg{}):          {"github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg", "*[]pkg.SamePkg"},
			reflect.TypeOf(make(map[int]pkg1.SamePkg)): {"map[int]pkg.SamePkg", "map[int]pkg.SamePkg"},

			reflect.TypeOf(pkg2.SamePkg{}):             {"github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "pkg.SamePkg"},
			reflect.TypeOf(new(pkg2.SamePkg)):          {"github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg"},
			reflect.TypeOf(make([]pkg2.SamePkg, 0)):    {"github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "[]pkg.SamePkg"},
			reflect.TypeOf(&[]pkg2.SamePkg{}):          {"github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*[]pkg.SamePkg"},
			reflect.TypeOf(make(map[int]pkg2.SamePkg)): {"map[int]pkg.SamePkg", "map[int]pkg.SamePkg"},
		}

		for typ, v := range data {
			typeName := SpringCore.TypeName(typ)
			assert.Equal(t, typeName, v.typeName)
			assert.Equal(t, typ.String(), v.baseName)
		}

		i := 3
		iPtr := &i
		iPtrPtr := &iPtr
		iPtrPtrPtr := &iPtrPtr
		typ := reflect.TypeOf(iPtrPtrPtr)
		typeName := SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "int")
		assert.Equal(t, typ.String(), "***int")
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
		assert.Equal(t, ok, v)
	}
}

func TestParseBeanId(t *testing.T) {

	data := map[string]struct {
		typeName string
		beanName string
		nullable bool
	}{
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
		typeName, beanName, nullable := SpringCore.ParseBeanId(k)
		assert.Equal(t, typeName, v.typeName)
		assert.Equal(t, beanName, v.beanName)
		assert.Equal(t, nullable, v.nullable)
	}

	assert.Panic(t, func() {
		SpringCore.ParseBeanId("int:[]?")
	}, "collection mode shouldn't have type")
}

func TestBeanDefinition_Match(t *testing.T) {

	data := []struct {
		bd       *SpringCore.BeanDefinition
		typeName string
		beanName string
		expect   bool
	}{
		{SpringCore.ToBeanDefinition("", new(int)), "int", "*int", true},
		{SpringCore.ToBeanDefinition("", new(int)), "", "*int", true},
		{SpringCore.ToBeanDefinition("", new(int)), "int", "", true},

		{SpringCore.ToBeanDefinition("i", new(int)), "int", "i", true},
		{SpringCore.ToBeanDefinition("i", new(int)), "", "i", true},
		{SpringCore.ToBeanDefinition("i", new(int)), "int", "", true},

		{SpringCore.ToBeanDefinition("", new(pkg2.SamePkg)), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg", true},
		{SpringCore.ToBeanDefinition("", new(pkg2.SamePkg)), "", "*pkg.SamePkg", true},
		{SpringCore.ToBeanDefinition("", new(pkg2.SamePkg)), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "", true},

		{SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg)), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
		{SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg)), "", "pkg2", true},
		{SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg)), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
	}

	for i, s := range data {
		if ok := s.bd.Match(s.typeName, s.beanName); ok != s.expect {
			t.Errorf("%d expect %v but %v", i, s.expect, ok)
		}
	}
}

func TestToBeanDefinition(t *testing.T) {

	t.Run("bean can't be nil", func(t *testing.T) {

		assert.Panic(t, func() {
			SpringCore.ToBeanDefinition("", nil)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var i *int
			SpringCore.ToBeanDefinition("", i)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var m map[string]string
			SpringCore.ToBeanDefinition("", m)
		}, "bean can't be nil")
	})

	t.Run("bean must be ref type", func(t *testing.T) {

		data := []func(){
			func() { SpringCore.ToBeanDefinition("", [...]int{0}) },
			func() { SpringCore.ToBeanDefinition("", false) },
			func() { SpringCore.ToBeanDefinition("", 3) },
			func() { SpringCore.ToBeanDefinition("", "3") },
			func() { SpringCore.ToBeanDefinition("", BeanZero{}) },
			func() { SpringCore.ToBeanDefinition("", pkg2.SamePkg{}) },
		}

		for _, fn := range data {
			assert.Panic(t, fn, "bean must be ref type")
		}
	})

	t.Run("valid bean", func(t *testing.T) {
		SpringCore.ToBeanDefinition("", make(chan int))
		SpringCore.ToBeanDefinition("", func() {})
		SpringCore.ToBeanDefinition("", make(map[string]int))
		SpringCore.ToBeanDefinition("", new(int))
		SpringCore.ToBeanDefinition("", &BeanZero{})
		SpringCore.ToBeanDefinition("", make([]int, 0))
	})

	t.Run("check name && typename", func(t *testing.T) {

		data := map[*SpringCore.BeanDefinition]struct {
			name     string
			typeName string
		}{
			SpringCore.ToBeanDefinition("", io.Writer(os.Stdout)): {
				"*os.File", "os/os.File",
			},

			SpringCore.ToBeanDefinition("", newHistoryTeacher("")): {
				"*SpringCore_test.historyTeacher",
				"github.com/go-spring/go-spring/spring-core_test/SpringCore_test.historyTeacher",
			},

			SpringCore.ToBeanDefinition("", new(int)): {
				"*int", "int",
			},

			SpringCore.ToBeanDefinition("i", new(int)): {
				"i", "int",
			},

			SpringCore.ToBeanDefinition("", new(pkg2.SamePkg)): {
				"*pkg.SamePkg",
				"github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg",
			},

			SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg)): {
				"pkg2",
				"github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg",
			},
		}

		for bd, v := range data {
			assert.Equal(t, bd.Name(), v.name)
			assert.Equal(t, bd.TypeName(), v.typeName)
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

func TestFnToBeanDefinition(t *testing.T) {

	bd := SpringCore.FnToBeanDefinition("", NewStudent)
	assert.Equal(t, bd.Type().String(), "*SpringCore_test.Student")

	bd = SpringCore.FnToBeanDefinition("", NewPtrStudent)
	assert.Equal(t, bd.Type().String(), "*SpringCore_test.Student")

	mapFn := func() map[int]string { return make(map[int]string) }
	bd = SpringCore.FnToBeanDefinition("", mapFn)
	assert.Equal(t, bd.Type().String(), "map[int]string")

	sliceFn := func() []int { return make([]int, 1) }
	bd = SpringCore.FnToBeanDefinition("", sliceFn)
	assert.Equal(t, bd.Type().String(), "[]int")

	funcFn := func() func(int) { return nil }
	bd = SpringCore.FnToBeanDefinition("", funcFn)
	assert.Equal(t, bd.Type().String(), "func(int)")

	intFn := func() int { return 0 }
	bd = SpringCore.FnToBeanDefinition("", intFn)
	assert.Equal(t, bd.Type().String(), "*int")
	assert.Equal(t, bd.Value().Type().String(), "*int")

	interfaceFn := func(name string) Teacher { return newHistoryTeacher(name) }
	bd = SpringCore.FnToBeanDefinition("", interfaceFn)
	assert.Equal(t, bd.Type().String(), "SpringCore_test.Teacher")
	assert.Equal(t, bd.Value().Type().String(), "SpringCore_test.Teacher")

	assert.Panic(t, func() {
		bd = SpringCore.FnToBeanDefinition("", func() (*int, *int) { return nil, nil })
		assert.Equal(t, bd.Type().String(), "*int")
	}, "func bean must be func\\(...\\)bean or func\\(...\\)\\(bean, error\\)")

	bd = SpringCore.FnToBeanDefinition("", func() (*int, error) { return nil, nil })
	assert.Equal(t, bd.Type().String(), "*int")
}

func TestValue(t *testing.T) {

	{
		var i int // 默认值
		v := reflect.ValueOf(i)
		// int 0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		i = 3
		v = reflect.ValueOf(i)
		// int 3 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var pi *int // 未赋值
		v = reflect.ValueOf(pi)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		pi = &i
		v = reflect.ValueOf(pi)
		// ptr 0xc0000a4e60 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var a [3]int // 内存已分配
		v := reflect.ValueOf(a)
		// array [0 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		a = [3]int{0, 0, 0} // 全零值
		v = reflect.ValueOf(a)
		// array [0 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		a = [3]int{1, 0, 0} // 非全零值
		v = reflect.ValueOf(a)
		// array [1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var pa *[3]int // 未赋值
		v = reflect.ValueOf(pa)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		pa = &a
		v = reflect.ValueOf(pa)
		// ptr &[1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var c chan struct{} // 未赋值
		v := reflect.ValueOf(c)
		// chan <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		c = make(chan struct{})
		v = reflect.ValueOf(c)
		// chan 0xc000086360 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var pc *chan struct{} // 未赋值
		v = reflect.ValueOf(pc)
		// chan <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		pc = &c
		v = reflect.ValueOf(pc)
		// ptr 0xc0000a00d8 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var f func() // 未赋值
		v := reflect.ValueOf(f)
		// func <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		f = func() {}
		v = reflect.ValueOf(f)
		// func 0x16d8810 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var pf *func() // 未赋值
		v = reflect.ValueOf(pf)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		pf = &f
		v = reflect.ValueOf(pf)
		// ptr 0xc0000a00e0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var m map[string]string // 未赋值
		v := reflect.ValueOf(m)
		// map map[] true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		m = map[string]string{}
		v = reflect.ValueOf(m)
		// map map[] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		m = map[string]string{"a": "1"}
		v = reflect.ValueOf(m)
		// map map[a:1] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var pm *map[string]string // 未赋值
		v = reflect.ValueOf(pm)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		pm = &m
		v = reflect.ValueOf(pm)
		// ptr &map[a:1] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var b []int // 未赋值
		v := reflect.ValueOf(b)
		// slice [] true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		b = []int{}
		v = reflect.ValueOf(b)
		// slice [] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		b = []int{0, 0}
		v = reflect.ValueOf(b)
		// slice [0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		b = []int{1, 0, 0}
		v = reflect.ValueOf(b)
		// slice [1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var pb *[]int // 未赋值
		v = reflect.ValueOf(pb)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		pb = &b
		v = reflect.ValueOf(pb)
		// ptr &[1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var s string // 默认值
		v := reflect.ValueOf(s)
		// string  true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		s = "s"
		v = reflect.ValueOf(s)
		// string s true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var ps *string // 未赋值
		v = reflect.ValueOf(ps)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		ps = &s
		v = reflect.ValueOf(ps)
		// ptr 0xc0000974f0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var st struct{} // 默认值
		v := reflect.ValueOf(st)
		// struct {} true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		var pst *struct{} // 未赋值
		v = reflect.ValueOf(pst)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		pst = &st
		v = reflect.ValueOf(pst)
		// ptr &{} true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}

	{
		var e error
		v := reflect.ValueOf(e)
		// invalid <invalid reflect.Value> false false ***
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))

		e = fmt.Errorf("e")
		v = reflect.ValueOf(e)
		// ptr e true false
		fmt.Println(v.Kind(), v, v.IsValid(), SpringCore.IsNil(v))
	}
}
