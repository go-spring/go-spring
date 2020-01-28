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
	"testing"

	pkg1 "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
)

func TestReflectType(t *testing.T) {

	///////////////////////////////////////
	// 基础数据类型

	t.Run("bool", func(t *testing.T) {

		// Bool
		{
			var b bool
			t := reflect.TypeOf(b)
			// bool bool bool
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Bool
		{
			var b bool
			t := reflect.TypeOf(&b)
			// *bool ptr bool bool
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Bool
		{
			var b []bool
			t := reflect.TypeOf(b)
			// []bool slice bool bool
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Bool
		{
			var b []bool
			t := reflect.TypeOf(&b)
			// *[]bool ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("int", func(t *testing.T) {

		// Int
		{
			var i int
			t := reflect.TypeOf(i)
			// int int int
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Int
		{
			var i int
			t := reflect.TypeOf(&i)
			// *int ptr int int
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Int
		{
			var i []int
			t := reflect.TypeOf(i)
			// []int slice int int
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Int
		{
			var i []int
			t := reflect.TypeOf(&i)
			// *[]int ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("uint", func(t *testing.T) {

		// Uint
		{
			var u uint
			t := reflect.TypeOf(u)
			// uint uint uint
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Uint
		{
			var u uint
			t := reflect.TypeOf(&u)
			// *uint ptr uint uint
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Uint
		{
			var u []uint
			t := reflect.TypeOf(u)
			// []uint slice uint uint
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Uint
		{
			var u []uint
			t := reflect.TypeOf(&u)
			// *[]uint ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("float32", func(t *testing.T) {

		// Float32
		{
			var f float32
			t := reflect.TypeOf(f)
			// float32 float32 float32
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Float32
		{
			var f float32
			t := reflect.TypeOf(&f)
			// *float32 ptr float32 float32
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Float32
		{
			var f []float32
			t := reflect.TypeOf(f)
			// []float32 slice float32 float32
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Float32
		{
			var f []float32
			t := reflect.TypeOf(&f)
			// *[]float32 ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("complex64", func(t *testing.T) {

		// Complex64
		{
			var c complex64
			t := reflect.TypeOf(c)
			// complex64 complex64 complex64
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Complex64
		{
			var c complex64
			t := reflect.TypeOf(&c)
			// *complex64 ptr complex64 complex64
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Complex64
		{
			var c []complex64
			t := reflect.TypeOf(c)
			// []complex64 slice complex64 complex64
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Complex64
		{
			var c []complex64
			t := reflect.TypeOf(&c)
			// *[]complex64 ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("string", func(t *testing.T) {

		// String
		{
			var s string
			t := reflect.TypeOf(s)
			// string string string
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *String
		{
			var s string
			t := reflect.TypeOf(&s)
			// *string ptr string string
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []String
		{
			var s []string
			t := reflect.TypeOf(s)
			// []string slice string string
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]String
		{
			var s []string
			t := reflect.TypeOf(&s)
			// *[]string ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	///////////////////////////////////////
	// map 数据类型

	t.Run("map", func(t *testing.T) {

		// map[string]string
		{
			var m map[string]string
			t := reflect.TypeOf(m)
			// map[string]string map
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *map[string]string
		{
			var m map[string]string
			t := reflect.TypeOf(&m)
			// *map[string]string ptr  map
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []map[string]string
		{
			var m []map[string]string
			t := reflect.TypeOf(m)
			// []map[string]string slice  map
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]map[string]string
		{
			var m []map[string]string
			t := reflect.TypeOf(&m)
			// *[]map[string]string ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	///////////////////////////////////////
	// 自定义数据类型

	t.Run("pkg1.SamePkg", func(t *testing.T) {

		{
			var o pkg1.SamePkg
			t := reflect.TypeOf(o)
			// pkg.SamePkg struct SamePkg
			fmt.Println(t, t.Kind(), t.Name())
		}

		{
			var o pkg1.SamePkg
			t := reflect.TypeOf(&o)
			// *pkg.SamePkg ptr SamePkg struct
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg1.SamePkg
			t := reflect.TypeOf(o)
			// []pkg.SamePkg slice SamePkg struct
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg1.SamePkg
			t := reflect.TypeOf(&o)
			// *[]pkg.SamePkg ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("pkg2.SamePkg", func(t *testing.T) {

		{
			var o pkg2.SamePkg
			t := reflect.TypeOf(o)
			// pkg.SamePkg struct SamePkg
			fmt.Println(t, t.Kind(), t.Name())
		}

		{
			var o pkg2.SamePkg
			t := reflect.TypeOf(&o)
			// *pkg.SamePkg ptr SamePkg struct
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg2.SamePkg
			t := reflect.TypeOf(o)
			// []pkg.SamePkg slice SamePkg struct
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg2.SamePkg
			t := reflect.TypeOf(&o)
			// *[]pkg.SamePkg ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("interface{}", func(t *testing.T) {

		{
			var r io.Reader
			t := reflect.TypeOf(r)
			// <nil>
			fmt.Println(t)
		}

		{
			var r io.Reader
			t := reflect.TypeOf(&r)
			// *io.Reader ptr Reader interface
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var r []io.Reader
			t := reflect.TypeOf(r)
			// []io.Reader slice Reader interface
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var r []io.Reader
			t := reflect.TypeOf(&r)
			// *[]io.Reader ptr  slice
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	{
		var o pkg1.SamePkg
		t := reflect.TypeOf(o)
		// pkg.SamePkg SamePkg github.com/go-spring/go-spring/spring-core/testdata/pkg/bar
		fmt.Println(t, t.Name(), t.PkgPath())
	}

	{
		var o pkg2.SamePkg
		t := reflect.TypeOf(o)
		// pkg.SamePkg SamePkg github.com/go-spring/go-spring/spring-core/testdata/pkg/foo
		fmt.Println(t, t.Name(), t.PkgPath())
	}
}

func TestRange(t *testing.T) {

	i := 0

	f := func() []int {
		i++
		return []int{5, 6, 7, 8}
	}

	// range 用法中的 f() 只调用一次
	for _, v := range f() {
		fmt.Println(v)
	}

	fmt.Println("count:", i)
}

func TestGetEnv(t *testing.T) {

	// \r 回车，从当前行的最前面开始输出，覆盖掉以前的内容
	fmt.Println("hello\rworld")

	fmt.Println(os.Getenv("GOPATH"), os.Getenv("GOROOT"))
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
	call() int
}

type caller struct {
	i int
}

func (c *caller) call() int {
	return c.i
}

func TestInterfaceMethod(t *testing.T) {
	c := callable(&caller{3})
	fmt.Println(c.call())
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
	fmt.Println((*caller).call(&c))

	typ = reflect.TypeOf((*caller).call)
	fmt.Println(typ)
}

func TestNilRecover(t *testing.T) {

	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
		}
	}()

	// 据说 nil 指针会报错，为什么我这里没有呢？
	//((*caller)(nil)).call()
}
