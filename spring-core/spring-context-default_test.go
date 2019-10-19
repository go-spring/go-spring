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
	"strconv"
	"testing"

	"github.com/go-spring/go-spring/spring-core"
	pkg1 "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
)

func TestDefaultSpringContext(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()

	/////////////////////////////////////////
	// 基础数据类型，int，string，float，complex

	{
		e := int(3)
		a := []int{3}

		// 普通类型用属性注入
		// ctx.RegisterBean(e)

		ctx.RegisterBean(&e)

		// 相同类型的匿名 bean 不能重复注册
		// ctx.RegisterBean(&e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i3", &e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i4", &e)

		ctx.RegisterBean(a)
		ctx.RegisterBean(&a)

		// 找到多个符合条件的值
		if false {
			var i int
			ctx.GetBean(&i)
		}

		// 入参不是可赋值的对象
		if false {
			var i int
			ctx.GetBeanByName("i3", &i)
			fmt.Println(i)
		}

		{
			var i *int
			// 直接使用缓存
			ctx.GetBeanByName("i3", &i)
			fmt.Println(*i)
		}

		{
			var i []int
			// 直接使用缓存
			ctx.GetBean(&i)
			fmt.Println(i)
		}

		{
			var i *[]int
			// 直接使用缓存
			ctx.GetBean(&i)
			fmt.Println(i)
		}
	}

	/////////////////////////////////////////
	// 自定义数据类型

	{
		e := pkg1.Demo{}
		a := []pkg1.Demo{{}}
		p := []*pkg1.Demo{{}}

		// 栈上的对象不能注册
		// ctx.RegisterBean(e)

		ctx.RegisterBean(&e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i3", &e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i4", &e)

		ctx.RegisterBean(a)
		ctx.RegisterBean(&a)
		ctx.RegisterBean(p)
		ctx.RegisterBean(&p)
	}

	{
		e := pkg2.Demo{}
		a := []pkg2.Demo{{}}
		p := []*pkg2.Demo{{}}

		// 栈上的对象不能注册
		// ctx.RegisterBean(e)

		ctx.RegisterBean(&e)

		// 相同类型不同名称的 bean 都可注册
		// 不同类型相同名称的 bean 也可注册
		ctx.RegisterNameBean("i3", &e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i4", &e)

		ctx.RegisterBean(a)
		ctx.RegisterBean(&a)
		ctx.RegisterBean(p)
		ctx.RegisterBean(&p)

		ctx.RegisterNameBean("i5", &e)
	}

	if false {
		var i SpringCore.SpringBean
		ctx.GetBean(&i)
		fmt.Println(i)
	}

	{
		var i SpringCore.SpringBean
		ctx.GetBeanByName("i5", &i)
		fmt.Println(i)
	}
}

type Binding struct {
	i int
}

func (b *Binding) String() string {
	if b == nil {
		return ""
	} else {
		return strconv.Itoa(b.i)
	}
}

type Object struct {
	// 基础类型指针
	IntPtrByType *int `autowire:""`
	IntPtrByName *int `autowire:"int_ptr"`

	// 基础类型数组
	//IntSliceByType []int `autowire:""`
	IntCollection   []int `autowire:"[]"`
	IntSliceByName  []int `autowire:"int_slice"`
	IntSliceByName2 []int `autowire:"int_slice_2"`

	// 基础类型指针数组
	IntPtrSliceByType []*int `autowire:""`
	IntPtrCollection  []*int `autowire:"[]"`
	IntPtrSliceByName []*int `autowire:"int_ptr_slice"`

	// 自定义类型指针
	StructByType *Binding `autowire:""`
	StructByName *Binding `autowire:"struct_ptr"`

	// 自定义类型数组
	StructSliceByType []Binding `autowire:""`
	StructCollection  []Binding `autowire:"[]"`
	StructSliceByName []Binding `autowire:"struct_slice"`

	// 自定义类型指针数组
	StructPtrSliceByType []*Binding `autowire:""`
	StructPtrCollection  []*Binding `autowire:"[]"`
	StructPtrSliceByName []*Binding `autowire:"struct_ptr_slice"`

	// 接口
	InterfaceByType fmt.Stringer `autowire:""`
	InterfaceByName fmt.Stringer `autowire:"struct_ptr"`

	// 接口数组
	InterfaceSliceByType []fmt.Stringer `autowire:""`

	InterfaceCollection  []fmt.Stringer `autowire:"[]"`
	InterfaceCollection2 []fmt.Stringer `autowire:"[]"`

	// 指定名称时使用精确匹配模式，不对数组元素进行转换，即便能做到似乎也无意义
	InterfaceSliceByName []fmt.Stringer `autowire:"struct_ptr_slice"`
}

func TestDefaultSpringContext_AutoWireBeans(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	obj := &Object{}
	ctx.RegisterBean(obj)

	i := int(3)
	ctx.RegisterNameBean("int_ptr", &i)

	if false {
		i2 := int(3)
		ctx.RegisterNameBean("int_ptr_2", &i2)
	}

	is := []int{1, 2, 3}
	ctx.RegisterNameBean("int_slice", is)

	is2 := []int{2, 3, 4}
	ctx.RegisterNameBean("int_slice_2", is2)

	i2 := 4
	ips := []*int{&i2}
	ctx.RegisterNameBean("int_ptr_slice", ips)

	b := Binding{1}
	ctx.RegisterNameBean("struct_ptr", &b)

	bs := []Binding{{10}}
	ctx.RegisterNameBean("struct_slice", bs)

	b2 := Binding{2}
	bps := []*Binding{&b2}
	ctx.RegisterNameBean("struct_ptr_slice", bps)

	s := []fmt.Stringer{&Binding{3}}
	ctx.RegisterBean(s)

	ctx.AutoWireBeans()

	fmt.Printf("%+v", obj)
}
