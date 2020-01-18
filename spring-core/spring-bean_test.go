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

	"github.com/go-spring/go-spring/spring-core"
	pkg1 "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
	"github.com/magiconair/properties/assert"
)

func TestBeanDefinition_Match(t *testing.T) {

	bd := SpringCore.ToBeanDefinition("", new(int))
	fmt.Println(bd.Caller())

	ok := bd.Match("int", "*int")
	assert.Equal(t, ok, true)

	ok = bd.Match("", "*int")
	assert.Equal(t, ok, true)

	ok = bd.Match("int", "")
	assert.Equal(t, ok, true)

	bd = SpringCore.ToBeanDefinition("i", new(int))
	fmt.Println(bd.Caller())

	ok = bd.Match("int", "i")
	assert.Equal(t, ok, true)

	ok = bd.Match("", "i")
	assert.Equal(t, ok, true)

	ok = bd.Match("int", "")
	assert.Equal(t, ok, true)

	bd = SpringCore.ToBeanDefinition("", new(pkg2.SamePkg))
	fmt.Println(bd.Caller())

	ok = bd.Match("github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg")
	assert.Equal(t, ok, true)

	ok = bd.Match("", "*pkg.SamePkg")
	assert.Equal(t, ok, true)

	ok = bd.Match("github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "")
	assert.Equal(t, ok, true)

	bd = SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg))
	fmt.Println(bd.Caller())

	ok = bd.Match("github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "pkg2")
	assert.Equal(t, ok, true)

	ok = bd.Match("", "pkg2")
	assert.Equal(t, ok, true)

	ok = bd.Match("github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "")
	assert.Equal(t, ok, true)
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

	mapFn := func() map[int]string {
		return make(map[int]string)
	}

	bd = SpringCore.FnToBeanDefinition("", mapFn)
	assert.Equal(t, bd.Type().String(), "map[int]string")

	sliceFn := func() []int {
		return make([]int, 1)
	}

	bd = SpringCore.FnToBeanDefinition("", sliceFn)
	assert.Equal(t, bd.Type().String(), "[]int")

	funcFn := func() func(int) {
		return nil
	}

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
		bd = SpringCore.FnToBeanDefinition("", func() (*int, *int) {
			return nil, nil
		})
		assert.Equal(t, bd.Type().String(), "*int")
	}, "func bean must be \"func\\(...\\) bean\" or \"func\\(...\\) \\(bean, error\\)\"")

	bd = SpringCore.FnToBeanDefinition("", func() (*int, error) {
		return nil, nil
	})
	assert.Equal(t, bd.Type().String(), "*int")
}

func TestToBeanDefinition(t *testing.T) {

	// nil
	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", nil)
	}, "bean can't be nil")

	// bool
	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", false)
	}, "bean must be ref type")

	// int
	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", 3)
	}, "bean must be ref type")

	// chan
	SpringCore.ToBeanDefinition("", make(chan int))

	// function
	SpringCore.ToBeanDefinition("", func() {})

	// map
	SpringCore.ToBeanDefinition("", make(map[string]int))

	// ptr
	SpringCore.ToBeanDefinition("", new(int))

	// func
	SpringCore.ToBeanDefinition("", &BeanZero{})

	// slice
	SpringCore.ToBeanDefinition("", make([]int, 0))

	// string
	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", "3")
	}, "bean must be ref type")

	// struct
	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", BeanZero{})
	}, "bean must be ref type")

	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", 3)
	}, "bean must be ref type")

	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", pkg2.SamePkg{})
	}, "bean must be ref type")

	// 用接口类型注册时实际使用的是原始类型
	bd := SpringCore.ToBeanDefinition("", io.Writer(os.Stdout))
	assert.Equal(t, bd.Name(), "*os.File")
	assert.Equal(t, bd.TypeName(), "os/os.File")

	bd = SpringCore.ToBeanDefinition("", newHistoryTeacher(""))
	assert.Equal(t, bd.Name(), "*SpringCore_test.historyTeacher")
	assert.Equal(t, bd.Type(), reflect.TypeOf(newHistoryTeacher("")))
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core_test/SpringCore_test.historyTeacher")

	// 用接口类型注册时实际使用的是原始类型
	bd = SpringCore.ToBeanDefinition("", Teacher(newHistoryTeacher("")))
	assert.Equal(t, bd.Name(), "*SpringCore_test.historyTeacher")
	assert.Equal(t, bd.Type(), reflect.TypeOf(newHistoryTeacher("")))
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core_test/SpringCore_test.historyTeacher")

	bd = SpringCore.ToBeanDefinition("", new(int))
	assert.Equal(t, bd.Name(), "*int")
	assert.Equal(t, bd.TypeName(), "int")

	bd = SpringCore.ToBeanDefinition("i", new(int))
	assert.Equal(t, bd.Name(), "i")
	assert.Equal(t, bd.TypeName(), "int")

	bd = SpringCore.ToBeanDefinition("", new(pkg2.SamePkg))
	assert.Equal(t, bd.Name(), "*pkg.SamePkg")
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")

	bd = SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg))
	assert.Equal(t, bd.Name(), "pkg2")
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")
}

func TestTypeName(t *testing.T) {

	assert.Panic(t, func() {
		SpringCore.TypeName(reflect.TypeOf(nil))
	}, "type shouldn't be nil")

	t.Run("int", func(t *testing.T) {

		// int
		typ := reflect.TypeOf(3)
		typeName := SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "int")
		assert.Equal(t, typ.String(), "int")

		// *int
		typ = reflect.TypeOf(new(int))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "int")
		assert.Equal(t, typ.String(), "*int")

		// []int
		typ = reflect.TypeOf(make([]int, 0))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "int")
		assert.Equal(t, typ.String(), "[]int")

		// *[]int
		typ = reflect.TypeOf(&[]int{3})
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "int")
		assert.Equal(t, typ.String(), "*[]int")

		// map[int]int
		typ = reflect.TypeOf(make(map[int]int))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "map[int]int")
		assert.Equal(t, typ.String(), "map[int]int")

		i := 3
		iPtr := &i
		iPtrPtr := &iPtr
		iPtrPtrPtr := &iPtrPtr
		typ = reflect.TypeOf(iPtrPtrPtr)
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "int")
		assert.Equal(t, typ.String(), "***int")
	})

	// bool
	typeName := SpringCore.TypeName(reflect.TypeOf(false))
	assert.Equal(t, typeName, "bool")

	t.Run("pkg1.SamePkg", func(t *testing.T) {

		// pkg1.SamePkg
		typ := reflect.TypeOf(pkg1.SamePkg{})
		typeName := SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg")
		assert.Equal(t, typ.String(), "pkg.SamePkg")

		// *pkg1.SamePkg
		typ = reflect.TypeOf(new(pkg1.SamePkg))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg")
		assert.Equal(t, typ.String(), "*pkg.SamePkg")

		// []pkg1.SamePkg
		typ = reflect.TypeOf(make([]pkg1.SamePkg, 0))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg")
		assert.Equal(t, typ.String(), "[]pkg.SamePkg")

		// *[]pkg1.SamePkg
		typ = reflect.TypeOf(&[]pkg1.SamePkg{})
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg")
		assert.Equal(t, typ.String(), "*[]pkg.SamePkg")

		// map[int]pkg1.SamePkg
		typ = reflect.TypeOf(make(map[int]pkg1.SamePkg))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "map[int]pkg.SamePkg")
		assert.Equal(t, typ.String(), "map[int]pkg.SamePkg")
	})

	t.Run("pkg2.SamePkg", func(t *testing.T) {

		// pkg2.SamePkg
		typ := reflect.TypeOf(pkg2.SamePkg{})
		typeName := SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")
		assert.Equal(t, typ.String(), "pkg.SamePkg")

		// *pkg2.SamePkg
		typ = reflect.TypeOf(new(pkg2.SamePkg))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")
		assert.Equal(t, typ.String(), "*pkg.SamePkg")

		// []pkg2.SamePkg
		typ = reflect.TypeOf(make([]pkg2.SamePkg, 0))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")
		assert.Equal(t, typ.String(), "[]pkg.SamePkg")

		// *[]pkg2.SamePkg
		typ = reflect.TypeOf(&[]pkg2.SamePkg{})
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")
		assert.Equal(t, typ.String(), "*[]pkg.SamePkg")

		// map[int]pkg2.SamePkg
		typ = reflect.TypeOf(make(map[int]pkg2.SamePkg))
		typeName = SpringCore.TypeName(typ)
		assert.Equal(t, typeName, "map[int]pkg.SamePkg")
		assert.Equal(t, typ.String(), "map[int]pkg.SamePkg")
	})
}

func TestParseBeanId(t *testing.T) {

	typeName, beanName, nullable := SpringCore.ParseBeanId("[]")
	assert.Equal(t, typeName, "")
	assert.Equal(t, beanName, "[]")
	assert.Equal(t, nullable, false)

	typeName, beanName, nullable = SpringCore.ParseBeanId("[]?")
	assert.Equal(t, typeName, "")
	assert.Equal(t, beanName, "[]")
	assert.Equal(t, nullable, true)

	assert.Panic(t, func() {
		SpringCore.ParseBeanId("int:[]?")
	}, "collection mode shouldn't have type")

	typeName, beanName, nullable = SpringCore.ParseBeanId("i")
	assert.Equal(t, typeName, "")
	assert.Equal(t, beanName, "i")
	assert.Equal(t, nullable, false)

	typeName, beanName, nullable = SpringCore.ParseBeanId("i?")
	assert.Equal(t, typeName, "")
	assert.Equal(t, beanName, "i")
	assert.Equal(t, nullable, true)

	typeName, beanName, nullable = SpringCore.ParseBeanId(":i")
	assert.Equal(t, typeName, "")
	assert.Equal(t, beanName, "i")
	assert.Equal(t, nullable, false)

	typeName, beanName, nullable = SpringCore.ParseBeanId(":i?")
	assert.Equal(t, typeName, "")
	assert.Equal(t, beanName, "i")
	assert.Equal(t, nullable, true)

	typeName, beanName, nullable = SpringCore.ParseBeanId("int:i")
	assert.Equal(t, typeName, "int")
	assert.Equal(t, beanName, "i")
	assert.Equal(t, nullable, false)

	typeName, beanName, nullable = SpringCore.ParseBeanId("int:i?")
	assert.Equal(t, typeName, "int")
	assert.Equal(t, beanName, "i")
	assert.Equal(t, nullable, true)

	typeName, beanName, nullable = SpringCore.ParseBeanId("int:")
	assert.Equal(t, typeName, "int")
	assert.Equal(t, beanName, "")
	assert.Equal(t, nullable, false)

	typeName, beanName, nullable = SpringCore.ParseBeanId("int:?")
	assert.Equal(t, typeName, "int")
	assert.Equal(t, beanName, "")
	assert.Equal(t, nullable, true)
}
