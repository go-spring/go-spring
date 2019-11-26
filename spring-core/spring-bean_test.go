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
	"reflect"
	"testing"

	"github.com/go-spring/go-spring/spring-core"
	pkg1 "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
	"github.com/magiconair/properties/assert"
)

func TestIsValidBean(t *testing.T) {

	// nil
	_, ok := SpringCore.IsValidBean(nil)
	assert.Equal(t, ok, false)

	// bool
	_, ok = SpringCore.IsValidBean(false)
	assert.Equal(t, ok, false)

	// int
	_, ok = SpringCore.IsValidBean(3)
	assert.Equal(t, ok, false)

	// chan
	_, ok = SpringCore.IsValidBean(make(chan int))
	assert.Equal(t, ok, false)

	// function
	_, ok = SpringCore.IsValidBean(SpringCore.IsValidBean)
	assert.Equal(t, ok, false)

	// map
	_, ok = SpringCore.IsValidBean(make(map[string]int))
	assert.Equal(t, ok, true)

	// ptr
	_, ok = SpringCore.IsValidBean(new(int))
	assert.Equal(t, ok, true)

	_, ok = SpringCore.IsValidBean(&BeanZero{})
	assert.Equal(t, ok, true)

	// slice
	_, ok = SpringCore.IsValidBean(make([]int, 0))
	assert.Equal(t, ok, true)

	// string
	_, ok = SpringCore.IsValidBean("3")
	assert.Equal(t, ok, false)

	// struct
	_, ok = SpringCore.IsValidBean(BeanZero{})
	assert.Equal(t, ok, false)
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

func TestToBeanDefinition(t *testing.T) {

	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", 3)
	}, "bean must be pointer or slice or map")

	assert.Panic(t, func() {
		SpringCore.ToBeanDefinition("", pkg2.SamePkg{})
	}, "bean must be pointer or slice or map")

	bd := SpringCore.ToBeanDefinition("", new(int))
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

type Teacher struct {
	Name string
}

type Student struct {
	Teacher *Teacher
	Room    string
}

// 入参可以进行注入或者属性绑定，返回值可以是 struct、map、slice 等。
func NewStudent(teacher *Teacher, room string) Student {
	return Student{
		Teacher: teacher,
		Room:    room,
	}
}

// 入参可以进行注入或者属性绑定，返回值可以是 struct、map、slice 等。
func NewPtrStudent(teacher *Teacher, room string) *Student {
	return &Student{
		Teacher: teacher,
		Room:    room,
	}
}

func TestNewConstructorBean(t *testing.T) {

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
}
