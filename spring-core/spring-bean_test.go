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
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/go-spring/go-spring/spring-core"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
	"github.com/magiconair/properties/assert"
)

func TestBeanDefinition_Match(t *testing.T) {

	bd := SpringCore.ToBeanDefinition("", new(int))

	ok := bd.Match("int", "*int")
	assert.Equal(t, ok, true)

	ok = bd.Match("", "*int")
	assert.Equal(t, ok, true)

	ok = bd.Match("int", "")
	assert.Equal(t, ok, true)

	bd = SpringCore.ToBeanDefinition("i", new(int))

	ok = bd.Match("int", "i")
	assert.Equal(t, ok, true)

	ok = bd.Match("", "i")
	assert.Equal(t, ok, true)

	ok = bd.Match("int", "")
	assert.Equal(t, ok, true)

	bd = SpringCore.ToBeanDefinition("", new(pkg2.SamePkg))

	ok = bd.Match("github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "*pkg.SamePkg")
	assert.Equal(t, ok, true)

	ok = bd.Match("", "*pkg.SamePkg")
	assert.Equal(t, ok, true)

	ok = bd.Match("github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg", "")
	assert.Equal(t, ok, true)

	bd = SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg))

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

func TestNewConstructorBean(t *testing.T) {

	SpringCore.NewConstructorBean(NewStudent, "")
	SpringCore.NewConstructorBean(NewStudent, "teacher")
	SpringCore.NewConstructorBean(NewStudent, "${room}")

	assert.Panic(t, func() {
		SpringCore.NewConstructorBean(NewStudent, "", "1:teacher")
	}, "tag \"1:teacher\" should no index")

	assert.Panic(t, func() {
		SpringCore.NewConstructorBean(NewStudent, "", "1:${room}")
	}, "tag \"1:\\${room}\" should no index")

	SpringCore.NewConstructorBean(NewStudent, "1:teacher")

	assert.Panic(t, func() {
		SpringCore.NewConstructorBean(NewStudent, "1:teacher", "")
	}, "tag \"\" should have index")

	assert.Panic(t, func() {
		SpringCore.NewConstructorBean(NewStudent, "1:teacher", "${room}")
	}, "tag \"\\${room}\" should have index")

	bean := SpringCore.NewConstructorBean(NewStudent)
	assert.Equal(t, bean.Type().String(), "*SpringCore_test.Student")

	bean = SpringCore.NewConstructorBean(NewPtrStudent)
	assert.Equal(t, bean.Type().String(), "*SpringCore_test.Student")

	mapFn := func() map[int]string {
		return make(map[int]string)
	}

	bean = SpringCore.NewConstructorBean(mapFn)
	assert.Equal(t, bean.Type().String(), "map[int]string")

	sliceFn := func() []int {
		return make([]int, 1)
	}

	bean = SpringCore.NewConstructorBean(sliceFn)
	assert.Equal(t, bean.Type().String(), "[]int")

	funcFn := func() func(int) {
		return nil
	}

	bean = SpringCore.NewConstructorBean(funcFn)
	assert.Equal(t, bean.Type().String(), "func(int)")
}

func TestToBeanDefinition(t *testing.T) {

	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", 3)
	}, "bean must be ptr or slice or map or func")

	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", pkg2.SamePkg{})
	}, "bean must be ptr or slice or map or func")

	// 用接口类型注册时实际使用的是原始类型
	bd := SpringCore.ToBeanDefinition("", io.Writer(os.Stdout))
	assert.Equal(t, bd.Name, "*os.File")
	assert.Equal(t, bd.TypeName(), "os/os.File")

	bd = SpringCore.ToBeanDefinition("", &historyTeacher{})
	assert.Equal(t, bd.Name, "*SpringCore_test.historyTeacher")
	assert.Equal(t, bd.Type(), reflect.TypeOf(&historyTeacher{}))
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core_test/SpringCore_test.historyTeacher")

	// 用接口类型注册时实际使用的是原始类型
	bd = SpringCore.ToBeanDefinition("", Teacher(&historyTeacher{}))
	assert.Equal(t, bd.Name, "*SpringCore_test.historyTeacher")
	assert.Equal(t, bd.Type(), reflect.TypeOf(&historyTeacher{}))
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core_test/SpringCore_test.historyTeacher")

	bd = SpringCore.ToBeanDefinition("", new(int))
	assert.Equal(t, bd.Name, "*int")
	assert.Equal(t, bd.TypeName(), "int")

	bd = SpringCore.ToBeanDefinition("i", new(int))
	assert.Equal(t, bd.Name, "i")
	assert.Equal(t, bd.TypeName(), "int")

	bd = SpringCore.ToBeanDefinition("", new(pkg2.SamePkg))
	assert.Equal(t, bd.Name, "*pkg.SamePkg")
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")

	bd = SpringCore.ToBeanDefinition("pkg2", new(pkg2.SamePkg))
	assert.Equal(t, bd.Name, "pkg2")
	assert.Equal(t, bd.TypeName(), "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo/pkg.SamePkg")
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

	interfaceFn := func() Teacher { return &historyTeacher{} }
	bd = SpringCore.FnToBeanDefinition("", interfaceFn)
	assert.Equal(t, bd.Type().String(), "SpringCore_test.Teacher")
	assert.Equal(t, bd.Value().Type().String(), "SpringCore_test.Teacher")
}

func TestIsValidBean(t *testing.T) {

	// nil
	assert.Panic(t, func() {
		SpringCore.NewOriginalBean(nil)
	}, "nil isn't valid bean")

	// bool
	assert.Panic(t, func() {
		SpringCore.NewOriginalBean(false)
	}, "bean must be ptr or slice or map or func")

	// int
	assert.Panic(t, func() {
		SpringCore.NewOriginalBean(3)
	}, "bean must be ptr or slice or map or func")

	// chan
	assert.Panic(t, func() {
		SpringCore.NewOriginalBean(make(chan int))
	}, "bean must be ptr or slice or map or func")

	// function
	SpringCore.NewOriginalBean(func() {})

	// map
	SpringCore.NewOriginalBean(make(map[string]int))

	// ptr
	SpringCore.NewOriginalBean(new(int))

	// func
	SpringCore.NewOriginalBean(&BeanZero{})

	// slice
	SpringCore.NewOriginalBean(make([]int, 0))

	// string
	assert.Panic(t, func() {
		SpringCore.NewOriginalBean("3")
	}, "bean must be ptr or slice or map or func")

	// struct
	assert.Panic(t, func() {
		SpringCore.NewOriginalBean(BeanZero{})
	}, "bean must be ptr or slice or map or func")
}
