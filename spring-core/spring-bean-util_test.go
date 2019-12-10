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
