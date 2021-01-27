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

	pkg1 "github.com/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/testdata/pkg/foo"
	"github.com/go-spring/spring-utils"
)

func TestReflectType(t *testing.T) {
	// 测试结论：内置类型的 Name 和 PkgPath 都是空字符串。

	SpringUtils.AssertEqual(t, reflect.TypeOf((io.Reader)(nil)), nil)

	data := []struct {
		typ     reflect.Type
		kind    reflect.Kind
		name    string
		pkgPath string
	}{
		{
			reflect.TypeOf(false),
			reflect.Bool,
			"bool",
			"",
		},
		{
			reflect.TypeOf(new(bool)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]bool, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(int(3)),
			reflect.Int,
			"int",
			"",
		},
		{
			reflect.TypeOf(new(int)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]int, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(uint(3)),
			reflect.Uint,
			"uint",
			"",
		},
		{
			reflect.TypeOf(new(uint)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]uint, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(float32(3)),
			reflect.Float32,
			"float32",
			"",
		},
		{
			reflect.TypeOf(new(float32)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]float32, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(complex64(3)),
			reflect.Complex64,
			"complex64",
			"",
		},
		{
			reflect.TypeOf(new(complex64)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]complex64, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf("3"),
			reflect.String,
			"string",
			"",
		},
		{
			reflect.TypeOf(new(string)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]string, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(map[int]int{}),
			reflect.Map,
			"",
			"",
		},
		{
			reflect.TypeOf(new(map[int]int)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]map[int]int, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(pkg1.SamePkg{}),
			reflect.Struct,
			"SamePkg",
			"github.com/go-spring/spring-core/testdata/pkg/bar",
		},
		{
			reflect.TypeOf(new(pkg1.SamePkg)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]pkg1.SamePkg, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]*pkg1.SamePkg, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf(pkg2.SamePkg{}),
			reflect.Struct,
			"SamePkg",
			"github.com/go-spring/spring-core/testdata/pkg/foo",
		},
		{
			reflect.TypeOf(new(pkg2.SamePkg)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf(make([]pkg2.SamePkg, 0)),
			reflect.Slice,
			"",
			"",
		},
		{
			reflect.TypeOf((*error)(nil)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf((*error)(nil)).Elem(),
			reflect.Interface,
			"error",
			"",
		},
		{
			reflect.TypeOf((*io.Reader)(nil)),
			reflect.Ptr,
			"",
			"",
		},
		{
			reflect.TypeOf((*io.Reader)(nil)).Elem(),
			reflect.Interface,
			"Reader",
			"io",
		},
	}

	for _, d := range data {
		SpringUtils.AssertEqual(t, d.typ.Kind(), d.kind)
		SpringUtils.AssertEqual(t, d.typ.Name(), d.name)
		SpringUtils.AssertEqual(t, d.typ.PkgPath(), d.pkgPath)
	}
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
	SpringUtils.AssertEqual(t, typ.NumMethod(), 1)

	typ = reflect.TypeOf(new(reCaller))
	SpringUtils.AssertEqual(t, typ.NumMethod(), 0)
}
