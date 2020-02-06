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

	data := []reflect.Type{
		reflect.TypeOf(false),
		reflect.TypeOf(new(bool)),
		reflect.TypeOf(make([]bool, 0)),
		reflect.TypeOf(int(3)),
		reflect.TypeOf(new(int)),
		reflect.TypeOf(make([]int, 0)),
		reflect.TypeOf(uint(3)),
		reflect.TypeOf(new(uint)),
		reflect.TypeOf(make([]uint, 0)),
		reflect.TypeOf(float32(3)),
		reflect.TypeOf(new(float32)),
		reflect.TypeOf(make([]float32, 0)),
		reflect.TypeOf(complex64(3)),
		reflect.TypeOf(new(complex64)),
		reflect.TypeOf(make([]complex64, 0)),
		reflect.TypeOf("3"),
		reflect.TypeOf(new(string)),
		reflect.TypeOf(make([]string, 0)),
		reflect.TypeOf(map[int]int{}),
		reflect.TypeOf(new(map[int]int)),
		reflect.TypeOf(make([]map[int]int, 0)),
		reflect.TypeOf(pkg1.SamePkg{}),
		reflect.TypeOf(new(pkg1.SamePkg)),
		reflect.TypeOf(make([]pkg1.SamePkg, 0)),
		reflect.TypeOf(make([]*pkg1.SamePkg, 0)),
		reflect.TypeOf(pkg2.SamePkg{}),
		reflect.TypeOf(new(pkg2.SamePkg)),
		reflect.TypeOf(make([]pkg2.SamePkg, 0)),
		//reflect.TypeOf((io.Reader)(nil)),
		reflect.TypeOf((*io.Reader)(nil)),
		reflect.TypeOf((*io.Reader)(nil)).Elem(),
	}

	for _, d := range data {
		fmt.Println(d, d.Kind(), d.Name())
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

	var arr []int
	fmt.Println(len(arr))
}
