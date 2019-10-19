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
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Bool
		{
			var b bool
			t := reflect.TypeOf(&b)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Bool
		{
			var b []bool
			t := reflect.TypeOf(b)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Bool
		{
			var b []bool
			t := reflect.TypeOf(&b)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("int", func(t *testing.T) {

		// Int
		{
			var i int
			t := reflect.TypeOf(i)
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Int
		{
			var i int
			t := reflect.TypeOf(&i)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Int
		{
			var i []int
			t := reflect.TypeOf(i)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Int
		{
			var i []int
			t := reflect.TypeOf(&i)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("uint", func(t *testing.T) {

		// Uint
		{
			var u uint
			t := reflect.TypeOf(u)
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Uint
		{
			var u uint
			t := reflect.TypeOf(&u)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Uint
		{
			var u []uint
			t := reflect.TypeOf(u)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Uint
		{
			var u []uint
			t := reflect.TypeOf(&u)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("float32", func(t *testing.T) {

		// Float32
		{
			var f float32
			t := reflect.TypeOf(f)
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Float32
		{
			var f float32
			t := reflect.TypeOf(&f)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Float32
		{
			var f []float32
			t := reflect.TypeOf(f)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Float32
		{
			var f []float32
			t := reflect.TypeOf(&f)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("complex64", func(t *testing.T) {

		// Complex64
		{
			var c complex64
			t := reflect.TypeOf(c)
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *Complex64
		{
			var c complex64
			t := reflect.TypeOf(&c)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []Complex64
		{
			var c []complex64
			t := reflect.TypeOf(c)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]Complex64
		{
			var c []complex64
			t := reflect.TypeOf(&c)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("string", func(t *testing.T) {

		// String
		{
			var s string
			t := reflect.TypeOf(s)
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *String
		{
			var s string
			t := reflect.TypeOf(&s)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []String
		{
			var s []string
			t := reflect.TypeOf(s)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]String
		{
			var s []string
			t := reflect.TypeOf(&s)
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
			fmt.Println(t, t.Kind(), t.Name())
		}

		// *map[string]string
		{
			var m map[string]string
			t := reflect.TypeOf(&m)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// []map[string]string
		{
			var m []map[string]string
			t := reflect.TypeOf(m)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		// *[]map[string]string
		{
			var m []map[string]string
			t := reflect.TypeOf(&m)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	///////////////////////////////////////
	// 自定义数据类型

	t.Run("pkg1.Demo", func(t *testing.T) {

		{
			var o pkg1.Demo
			t := reflect.TypeOf(o)
			fmt.Println(t, t.Kind(), t.Name())
		}

		{
			var o pkg1.Demo
			t := reflect.TypeOf(&o)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg1.Demo
			t := reflect.TypeOf(o)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg1.Demo
			t := reflect.TypeOf(&o)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("pkg2.Demo", func(t *testing.T) {

		{
			var o pkg2.Demo
			t := reflect.TypeOf(o)
			fmt.Println(t, t.Kind(), t.Name())
		}

		{
			var o pkg2.Demo
			t := reflect.TypeOf(&o)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg2.Demo
			t := reflect.TypeOf(o)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var o []pkg2.Demo
			t := reflect.TypeOf(&o)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})

	t.Run("interface{}", func(t *testing.T) {

		{
			var r io.Reader
			t := reflect.TypeOf(r)
			fmt.Println(t)
		}

		{
			var r io.Reader
			t := reflect.TypeOf(&r)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var r []io.Reader
			t := reflect.TypeOf(r)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}

		{
			var r []io.Reader
			t := reflect.TypeOf(&r)
			fmt.Println(t, t.Kind(), t.Elem().Name(), t.Elem().Kind())
		}
	})
}
