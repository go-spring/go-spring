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
	"io"
	"reflect"
	"testing"

	"github.com/go-spring/spring-base/assert"
	pkg1 "github.com/go-spring/spring-base/util/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-base/util/testdata/pkg/foo"
)

func TestPkgPath(t *testing.T) {
	// 测试结论：内置类型的 Name 和 PkgPath 都是空字符串。

	assert.Nil(t, reflect.TypeOf((io.Reader)(nil)))

	data := []struct {
		typ  reflect.Type
		kind reflect.Kind
		name string
		pkg  string
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
		assert.Equal(t, d.typ.PkgPath(), d.pkg)
	}
}
