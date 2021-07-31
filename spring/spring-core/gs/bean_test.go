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

	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/gs/arg"
	pkg1 "github.com/go-spring/spring-core/gs/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/gs/testdata/pkg/foo"
	"github.com/go-spring/spring-stl/assert"
	"github.com/go-spring/spring-stl/util"
)

// newBean 该方法是为了平衡调用栈的深度，一般情况下 gs.NewBean 不应该被直接使用。
func newBean(objOrCtor interface{}, ctorArgs ...arg.Arg) *gs.BeanDefinition {
	return gs.NewBean(objOrCtor, ctorArgs...)
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
		{newBean(new(int)), "int", "*int", true},
		{newBean(new(int)), "", "*int", true},
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
				"*int", "int",
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

	mapFn := func() map[int]string { return make(map[int]string) }
	bd = newBean(mapFn)
	assert.Equal(t, bd.Type().String(), "*map[int]string")

	sliceFn := func() []int { return make([]int, 1) }
	bd = newBean(sliceFn)
	assert.Equal(t, bd.Type().String(), "*[]int")

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

	typ := reflect.TypeOf(new(caller))
	assert.Equal(t, typ.NumMethod(), 1)

	typ = reflect.TypeOf(new(reCaller))
	assert.Equal(t, typ.NumMethod(), 0)
}
