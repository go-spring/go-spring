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

package util_test

import (
	"fmt"
	"io"
	"reflect"
	"sync"
	"testing"
	"unsafe"

	"github.com/go-spring/spring-base/assert"
	"github.com/go-spring/spring-base/util"
	pkg1 "github.com/go-spring/spring-base/util/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-base/util/testdata/pkg/foo"
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

func TestValue(t *testing.T) {

	{
		var i int // 默认值
		v := reflect.ValueOf(i)
		// int 0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		i = 3
		v = reflect.ValueOf(i)
		// int 3 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var pi *int // 未赋值
		v = reflect.ValueOf(pi)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		pi = &i
		v = reflect.ValueOf(pi)
		// ptr 0xc0000a4e60 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var a [3]int // 内存已分配
		v := reflect.ValueOf(a)
		// array [0 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		a = [3]int{0, 0, 0} // 全零值
		v = reflect.ValueOf(a)
		// array [0 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		a = [3]int{1, 0, 0} // 非全零值
		v = reflect.ValueOf(a)
		// array [1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var pa *[3]int // 未赋值
		v = reflect.ValueOf(pa)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		pa = &a
		v = reflect.ValueOf(pa)
		// ptr &[1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var c chan struct{} // 未赋值
		v := reflect.ValueOf(c)
		// chan <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		c = make(chan struct{})
		v = reflect.ValueOf(c)
		// chan 0xc000086360 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var pc *chan struct{} // 未赋值
		v = reflect.ValueOf(pc)
		// chan <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		pc = &c
		v = reflect.ValueOf(pc)
		// ptr 0xc0000a00d8 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var f func() // 未赋值
		v := reflect.ValueOf(f)
		// func <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		f = func() {}
		v = reflect.ValueOf(f)
		// func 0x16d8810 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var pf *func() // 未赋值
		v = reflect.ValueOf(pf)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		pf = &f
		v = reflect.ValueOf(pf)
		// ptr 0xc0000a00e0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var m map[string]string // 未赋值
		v := reflect.ValueOf(m)
		// map map[] true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		m = map[string]string{}
		v = reflect.ValueOf(m)
		// map map[] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		m = map[string]string{"a": "1"}
		v = reflect.ValueOf(m)
		// map map[a:1] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var pm *map[string]string // 未赋值
		v = reflect.ValueOf(pm)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		pm = &m
		v = reflect.ValueOf(pm)
		// ptr &map[a:1] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var b []int // 未赋值
		v := reflect.ValueOf(b)
		// slice [] true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		b = []int{}
		v = reflect.ValueOf(b)
		// slice [] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		b = []int{0, 0}
		v = reflect.ValueOf(b)
		// slice [0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		b = []int{1, 0, 0}
		v = reflect.ValueOf(b)
		// slice [1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var pb *[]int // 未赋值
		v = reflect.ValueOf(pb)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		pb = &b
		v = reflect.ValueOf(pb)
		// ptr &[1 0 0] true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var s string // 默认值
		v := reflect.ValueOf(s)
		// string  true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		s = "s"
		v = reflect.ValueOf(s)
		// string s true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var ps *string // 未赋值
		v = reflect.ValueOf(ps)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		ps = &s
		v = reflect.ValueOf(ps)
		// ptr 0xc0000974f0 true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var st struct{} // 默认值
		v := reflect.ValueOf(st)
		// struct {} true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		var pst *struct{} // 未赋值
		v = reflect.ValueOf(pst)
		// ptr <nil> true true ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		pst = &st
		v = reflect.ValueOf(pst)
		// ptr &{} true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}

	{
		var e error
		v := reflect.ValueOf(e)
		// invalid <invalid reflect.Value> false false ***
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))

		e = fmt.Errorf("e")
		v = reflect.ValueOf(e)
		// ptr e true false
		fmt.Println(v.Kind(), v, v.IsValid(), util.IsNil(v))
	}
}

func TestReflectType(t *testing.T) {
	// 测试结论：内置类型的 Name 和 PkgPath 都是空字符串。

	assert.Nil(t, reflect.TypeOf((io.Reader)(nil)))

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
			"github.com/go-spring/spring-base/util/testdata/pkg/bar",
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
			"github.com/go-spring/spring-base/util/testdata/pkg/foo",
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
		assert.Equal(t, d.typ.Kind(), d.kind)
		assert.Equal(t, d.typ.Name(), d.name)
		assert.Equal(t, d.typ.PkgPath(), d.pkgPath)
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
		{make(map[int]int), true},                     // Map
		{make(map[string]*int), false},                //
		{new(int), false},                             // Ptr
		{new(struct{}), false},                        //
		{[]int{0}, true},                              // Slice
		{[]*int{}, false},                             //
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
		if r := util.IsValueType(typ); d.v != r {
			t.Errorf("%v expect %v but %v", typ, d.v, r)
		}
	}
}
