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

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/gs"
	pkg2 "github.com/go-spring/spring-core/gs/testdata/pkg/foo"
)

//func TestParseSingletonTag(t *testing.T) {
//
//	data := map[string]SingletonTag{
//		"[]":     {"", "[]", false},
//		"[]?":    {"", "[]", true},
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
//		"[]":  {[]SingletonTag{}, false},
//		"[]?": {[]SingletonTag{}, true},
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
		ok := bean.IsFactoryType(k)
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
		{gs.NewBean(new(int)), "int", "*int", true},
		{gs.NewBean(new(int)), "", "*int", true},
		{gs.NewBean(new(int)), "int", "", true},
		{gs.NewBean(new(int)).WithName("i"), "int", "i", true},
		{gs.NewBean(new(int)).WithName("i"), "", "i", true},
		{gs.NewBean(new(int)).WithName("i"), "int", "", true},
		{gs.NewBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg", true},
		{gs.NewBean(new(pkg2.SamePkg)), "", "*pkg.SamePkg", true},
		{gs.NewBean(new(pkg2.SamePkg)), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "", true},
		{gs.NewBean(new(pkg2.SamePkg)).WithName("pkg2"), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
		{gs.NewBean(new(pkg2.SamePkg)).WithName("pkg2"), "", "pkg2", true},
		{gs.NewBean(new(pkg2.SamePkg)).WithName("pkg2"), "github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg", "pkg2", true},
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
			gs.NewBean(nil)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var i *int
			gs.NewBean(i)
		}, "bean can't be nil")

		assert.Panic(t, func() {
			var m map[string]string
			gs.NewBean(m)
		}, "bean can't be nil")
	})

	t.Run("bean must be ref type", func(t *testing.T) {

		data := []func(){
			func() { gs.NewBean([...]int{0}) },
			func() { gs.NewBean(false) },
			func() { gs.NewBean(3) },
			func() { gs.NewBean("3") },
			func() { gs.NewBean(BeanZero{}) },
			func() { gs.NewBean(pkg2.SamePkg{}) },
		}

		for _, fn := range data {
			assert.Panic(t, fn, "bean must be ref type")
		}
	})

	t.Run("valid bean", func(t *testing.T) {
		gs.NewBean(make(chan int))
		gs.NewBean(reflect.ValueOf(func() {}))
		gs.NewBean(make(map[string]int))
		gs.NewBean(new(int))
		gs.NewBean(&BeanZero{})
		gs.NewBean(make([]int, 0))
	})

	t.Run("check name && typename", func(t *testing.T) {

		data := map[*gs.BeanDefinition]struct {
			name     string
			typeName string
		}{
			gs.NewBean(io.Writer(os.Stdout)): {
				"*os.File", "os/os.File",
			},

			gs.NewBean(newHistoryTeacher("")): {
				"*gs_test.historyTeacher",
				"github.com/go-spring/spring-core/gs_test/gs_test.historyTeacher",
			},

			gs.NewBean(new(int)): {
				"*int", "int",
			},

			gs.NewBean(new(int)).WithName("i"): {
				"i", "int",
			},

			gs.NewBean(new(pkg2.SamePkg)): {
				"*pkg.SamePkg",
				"github.com/go-spring/spring-core/gs/testdata/pkg/foo/pkg.SamePkg",
			},

			gs.NewBean(new(pkg2.SamePkg)).WithName("pkg2"): {
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

	bd := gs.NewBean(NewStudent)
	assert.Equal(t, bd.Type().String(), "*gs_test.Student")

	bd = gs.NewBean(NewPtrStudent)
	assert.Equal(t, bd.Type().String(), "*gs_test.Student")

	mapFn := func() map[int]string { return make(map[int]string) }
	bd = gs.NewBean(mapFn)
	assert.Equal(t, bd.Type().String(), "map[int]string")

	sliceFn := func() []int { return make([]int, 1) }
	bd = gs.NewBean(sliceFn)
	assert.Equal(t, bd.Type().String(), "[]int")

	funcFn := func() func(int) { return nil }
	bd = gs.NewBean(funcFn)
	assert.Equal(t, bd.Type().String(), "func(int)")

	intFn := func() int { return 0 }
	bd = gs.NewBean(intFn)
	assert.Equal(t, bd.Type().String(), "*int")

	interfaceFn := func(name string) Teacher { return newHistoryTeacher(name) }
	bd = gs.NewBean(interfaceFn)
	assert.Equal(t, bd.Type().String(), "gs_test.Teacher")

	assert.Panic(t, func() {
		_ = gs.NewBean(func() (*int, *int) { return nil, nil })
	}, "func bean must be func\\(...\\)bean or func\\(...\\)\\(bean, error\\)")

	bd = gs.NewBean(func() (*int, error) { return nil, nil })
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

	typ := reflect.TypeOf(new(caller))
	assert.Equal(t, typ.NumMethod(), 1)

	typ = reflect.TypeOf(new(reCaller))
	assert.Equal(t, typ.NumMethod(), 0)
}
