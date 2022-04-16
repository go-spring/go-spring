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

package gs_test

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"testing"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/internal"
	pkg1 "github.com/go-spring/spring-core/gs/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/gs/testdata/pkg/foo"
)

// newBean 该方法是为了平衡调用栈的深度，一般情况下 gs.NewBean 不应该被直接使用。
func newBean(objOrCtor interface{}, ctorArgs ...arg.Arg) *gs.BeanDefinition {
	return gs.NewBean(objOrCtor, ctorArgs...)
}

func TestRefType(t *testing.T) {

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
		if r := internal.IsBeanType(typ); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
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
	}

	for typ, v := range data {
		typeName := internal.TypeName(typ)
		assert.Equal(t, typeName, v.typeName)
		assert.Equal(t, typ.String(), v.baseName)
	}

	i := 3
	iPtr := &i
	iPtrPtr := &iPtr
	iPtrPtrPtr := &iPtrPtr
	typ := reflect.TypeOf(iPtrPtrPtr)
	typeName := internal.TypeName(typ)
	assert.Equal(t, typeName, "int")
	assert.Equal(t, typ.String(), "***int")
}

//func TestParseSingletonTag(t *testing.T) {
//
//	data := map[string]SingletonTag{
//		"?":      {"", "", true},
//		"i":      {"", "i", false},
//		"i?":     {"", "i", true},
//		":i":     {"", "i", false},
//		":i?":    {"", "i", true},
//		"int:i":  {"int", "i", false},
//		"int:i?": {"int", "i", true},
//		"int:":   {"int", "", false},
//		"int:?":  {"int", "", true},
//	}
//
//	for k, v := range data {
//		tag := parseSingletonTag(k)
//		util.Equal(t, tag, v)
//	}
//}
//
//func TestParseBeanTag(t *testing.T) {
//
//	data := map[string]collectionTag{
//		"?":   {[]SingletonTag{}, true},
//	}
//
//	for k, v := range data {
//		tag := ParseCollectionTag(k)
//		util.Equal(t, tag, v)
//	}
//}

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
		ok := util.IsConstructor(k)
		assert.Equal(t, ok, v)
	}
}

func TestBeanDefinition_Match(t *testing.T) {

	data := []struct {
		bd       *gs.BeanDefinition
		typeName string
		beanName string
		expect   bool
	}{
		{newBean(new(int)), "int", "int", true},
		{newBean(new(int)), "", "int", true},
		{newBean(new(int)), "int", "", true},
		{newBean(new(int)).Name("i"), "int", "i", true},
		{newBean(new(int)).Name("i"), "", "i", true},
		{newBean(new(int)).Name("i"), "int", "", true},
		{newBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "SamePkg", true},
		{newBean(new(pkg2.SamePkg)), "", "SamePkg", true},
		{newBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "", true},
		{newBean(new(pkg2.SamePkg)).Name("pkg2"), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
		{newBean(new(pkg2.SamePkg)).Name("pkg2"), "", "pkg2", true},
		{newBean(new(pkg2.SamePkg)).Name("pkg2"), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
		{newBean(new(pkg1.SamePkg)), "github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "SamePkg", true},
		{newBean(new(pkg1.SamePkg)), "", "SamePkg", true},
		{newBean(new(pkg1.SamePkg)), "github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "", true},
		{newBean(new(pkg1.SamePkg)).Name("pkg1"), "github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "pkg1", true},
		{newBean(new(pkg1.SamePkg)).Name("pkg1"), "", "pkg1", true},
		{newBean(new(pkg1.SamePkg)).Name("pkg1"), "github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg", "pkg1", true},
	}

	for i, s := range data {
		if ok := s.bd.Match(s.typeName, s.beanName); ok != s.expect {
			t.Errorf("%d expect %v but %v", i, s.expect, ok)
		}
	}
}

type BeanZero struct {
	Int int
}

type BeanOne struct {
	Zero *BeanZero `autowire:""`
}

type BeanTwo struct {
	One *BeanOne `autowire:""`
}

func (t *BeanTwo) Group() {
}

type BeanThree struct {
	One *BeanTwo `autowire:""`
}

func (t *BeanThree) String() string {
	return ""
}

func TestObjectBean(t *testing.T) {

	t.Run("bean can't be nil", func(t *testing.T) {

		assert.Panic(t, func() {
			newBean(nil)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var i *int
			newBean(i)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var m map[string]string
			newBean(m)
		}, "bean can't be nil")
	})

	t.Run("bean must be ref type", func(t *testing.T) {

		data := []func(){
			func() { newBean([...]int{0}) },
			func() { newBean(false) },
			func() { newBean(3) },
			func() { newBean("3") },
			func() { newBean(BeanZero{}) },
			func() { newBean(pkg2.SamePkg{}) },
		}

		for _, fn := range data {
			assert.Panic(t, fn, "bean must be ref type")
		}
	})

	t.Run("valid bean", func(t *testing.T) {
		newBean(make(chan int))
		newBean(reflect.ValueOf(func() {}))
		newBean(new(int))
		newBean(&BeanZero{})
	})

	t.Run("check name && typename", func(t *testing.T) {

		data := map[*gs.BeanDefinition]struct {
			name     string
			typeName string
		}{
			newBean(io.Writer(os.Stdout)): {
				"File", "os/os.File",
			},

			newBean(newHistoryTeacher("")): {
				"historyTeacher",
				"github.com/go-spring/spring-core/gs_test/gs_test.historyTeacher",
			},

			newBean(new(int)): {
				"int", "int",
			},

			newBean(new(int)).Name("i"): {
				"i", "int",
			},

			newBean(new(pkg2.SamePkg)): {
				"SamePkg",
				"github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg",
			},

			newBean(new(pkg2.SamePkg)).Name("pkg2"): {
				"pkg2",
				"github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg",
			},
		}

		for bd, v := range data {
			assert.Equal(t, bd.BeanName(), v.name)
			assert.Equal(t, bd.TypeName(), v.typeName)
		}
	})
}

func TestConstructorBean(t *testing.T) {

	bd := newBean(NewStudent)
	assert.Equal(t, bd.Type().String(), "*gs_test.Student")

	bd = newBean(NewPtrStudent)
	assert.Equal(t, bd.Type().String(), "*gs_test.Student")

	//mapFn := func() map[int]string { return make(map[int]string) }
	//bd = newBean(mapFn)
	//assert.Equal(t, bd.Type().String(), "*map[int]string")

	//sliceFn := func() []int { return make([]int, 1) }
	//bd = newBean(sliceFn)
	//assert.Equal(t, bd.Type().String(), "*[]int")

	funcFn := func() func(int) { return nil }
	bd = newBean(funcFn)
	assert.Equal(t, bd.Type().String(), "func(int)")

	intFn := func() int { return 0 }
	bd = newBean(intFn)
	assert.Equal(t, bd.Type().String(), "*int")

	interfaceFn := func(name string) Teacher { return newHistoryTeacher(name) }
	bd = newBean(interfaceFn)
	assert.Equal(t, bd.Type().String(), "gs_test.Teacher")

	assert.Panic(t, func() {
		_ = newBean(func() (*int, *int) { return nil, nil })
	}, "constructor should be func\\(...\\)bean or func\\(...\\)\\(bean, error\\)")

	bd = newBean(func() (*int, error) { return nil, nil })
	assert.Equal(t, bd.Type().String(), "*int")
}

type Runner interface {
	Run()
}

type RunStringer struct {
}

func NewRunStringer() fmt.Stringer {
	return &RunStringer{}
}

func (rs *RunStringer) String() string {
	return "RunStringer"
}

func (rs *RunStringer) Run() {
	fmt.Println("RunStringer")
}

func TestInterface(t *testing.T) {

	t.Run("interface type", func(t *testing.T) {
		fnValue := reflect.ValueOf(NewRunStringer)
		fmt.Println(fnValue.Type())
		retValue := fnValue.Call([]reflect.Value{})[0]
		fmt.Println(retValue.Type(), retValue.Elem().Type())
		r := new(Runner)
		fmt.Println(reflect.TypeOf(r).Elem())
		ok := retValue.Elem().Type().AssignableTo(reflect.TypeOf(r).Elem())
		fmt.Println(ok)
	})

	fn := func() io.Reader {
		return os.Stdout
	}

	fnType := reflect.TypeOf(fn)
	// func() io.Reader
	fmt.Println(fnType)

	outType := fnType.Out(0)
	// io.Reader
	fmt.Println(outType)

	fnValue := reflect.ValueOf(fn)
	out := fnValue.Call([]reflect.Value{})

	outValue := out[0]
	// 0xc000010010 io.Reader
	fmt.Println(outValue, outValue.Type())
	// &{0xc0000a4000} *os.File
	fmt.Println(outValue.Elem(), outValue.Elem().Type())
}

type callable interface {
	Call() int
}

type caller struct {
	i int
}

func (c *caller) Call() int {
	return c.i
}

func TestInterfaceMethod(t *testing.T) {
	c := callable(&caller{3})
	fmt.Println(c.Call())
}

func TestVariadicFunction(t *testing.T) {

	fn := func(a string, i ...int) {
		fmt.Println(a, i)
	}

	typ := reflect.TypeOf(fn)
	fmt.Println(typ, typ.IsVariadic())

	for i := 0; i < typ.NumIn(); i++ {
		in := typ.In(i)
		fmt.Println(in)
	}

	fnValue := reflect.ValueOf(fn)
	fnValue.Call([]reflect.Value{
		reflect.ValueOf("string"),
		reflect.ValueOf(3),
		reflect.ValueOf(4),
	})

	c := caller{6}
	fmt.Println((*caller).Call(&c))

	typ = reflect.TypeOf((*caller).Call)
	fmt.Println(typ)

	var arr []int
	fmt.Println(len(arr))
}

type reCaller caller

func TestNumMethod(t *testing.T) {

	typ0 := reflect.TypeOf(new(caller))
	assert.Equal(t, typ0.NumMethod(), 1)

	typ1 := reflect.TypeOf(new(reCaller))
	assert.Equal(t, typ1.NumMethod(), 0)

	typ2 := reflect.TypeOf((*reCaller)(nil))
	assert.True(t, typ1 == typ2)
}
