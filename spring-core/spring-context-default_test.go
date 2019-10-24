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
	"strings"
	"testing"
	"time"

	"github.com/go-spring/go-spring/spring-core"
	pkg1 "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
	"github.com/spf13/cast"
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
		e := pkg1.SamePkg{}
		a := []pkg1.SamePkg{{}}
		p := []*pkg1.SamePkg{{}}

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
		e := pkg2.SamePkg{}
		a := []pkg2.SamePkg{{}}
		p := []*pkg2.SamePkg{{}}

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
	InterfaceSliceByName []fmt.Stringer `autowire:"struct_ptr_slice?"`
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

	fmt.Printf("%+v\n", obj)
}

type SubSubSetting struct {
	Int        int `value:"${int}"`
	DefaultInt int `value:"${default.int:=2}"`
}

type SubSetting struct {
	Int        int `value:"${int}"`
	DefaultInt int `value:"${default.int:=2}"`

	SubSubSetting SubSubSetting `value:"${sub}"`
}

type Setting struct {
	Int        int `value:"${int}"`
	DefaultInt int `value:"${default.int:=2}"`
	//IntPtr     *int `value:"${int}"` // 不支持指针

	Uint        uint `value:"${uint}"`
	DefaultUint uint `value:"${default.uint:=2}"`

	Float        float32 `value:"${float}"`
	DefaultFloat float32 `value:"${default.float:=2}"`

	//Complex complex64 `value:"${complex}"` // 不支持复数

	String        string `value:"${string}"`
	DefaultString string `value:"${default.string:=2}"`

	Bool        bool `value:"${bool}"`
	DefaultBool bool `value:"${default.bool:=false}"`

	SubSetting SubSetting `value:"${sub}"`
	//SubSettingPtr *SubSetting `value:"${sub}"` // 不支持指针

	SubSubSetting SubSubSetting `value:"${sub_sub}"`
}

func TestDefaultSpringContext_ValueTag(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.SetProperty("int", int(3))
	ctx.SetProperty("uint", uint(3))
	ctx.SetProperty("float", float32(3))
	ctx.SetProperty("complex", complex(3, 0))
	ctx.SetProperty("string", "3")
	ctx.SetProperty("bool", true)

	setting := &Setting{}
	ctx.RegisterBean(setting)

	ctx.SetProperty("sub.int", int(4))
	ctx.SetProperty("sub.sub.int", int(5))
	ctx.SetProperty("sub_sub.int", int(6))

	ctx.AutoWireBeans()

	fmt.Printf("%+v\n", setting)
}

type GreetingService struct {
}

func (gs *GreetingService) Greeting(name string) string {
	return "hello " + name
}

type PrototypeBean struct {
	Service *GreetingService `autowire:""`
	name    string
	t       time.Time
}

func (p *PrototypeBean) Greeting() string {
	return p.t.Format("15:04:05.000") + " " + p.Service.Greeting(p.name)
}

type PrototypeBeanFactory struct {
	Ctx SpringCore.SpringContext `autowire:""`
}

func (f *PrototypeBeanFactory) New(name string) *PrototypeBean {
	b := &PrototypeBean{
		name: name,
		t:    time.Now(),
	}

	// PrototypeBean 依赖的服务可以通过 SpringContext 注入
	f.Ctx.WireBean(b)

	return b
}

type PrototypeBeanService struct {
	Factory *PrototypeBeanFactory `autowire:""`
}

func (s *PrototypeBeanService) Service(name string) {
	// 通过 PrototypeBean 的工厂获取新的实例，并且每个实例都有自己的时间戳
	fmt.Println(s.Factory.New(name).Greeting())
}

func TestDefaultSpringContext_PrototypeBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()
	ctx.RegisterBean(ctx)

	gs := &GreetingService{}
	ctx.RegisterBean(gs)

	s := &PrototypeBeanService{}
	ctx.RegisterBean(s)

	f := &PrototypeBeanFactory{}
	ctx.RegisterBean(f)

	ctx.AutoWireBeans()

	s.Service("Li Lei")
	time.Sleep(50 * time.Millisecond)

	s.Service("Jim Green")
	time.Sleep(50 * time.Millisecond)

	s.Service("Han MeiMei")
}

type EnvEnum string

const (
	ENV_TEST    EnvEnum = "test"
	ENV_PRODUCT EnvEnum = "product"
)

type EnvEnumBean struct {
	EnvType EnvEnum `value:"${env.type}"`
}

type Point struct {
	x int
	y int
}

type PointBean struct {
	Point        Point `value:"${point}"`
	DefaultPoint Point `value:"${default_point:=(3,4)}"`
}

func PointConverter(val string) Point {
	if !(strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")")) {
		panic("数据格式错误")
	}
	ss := strings.Split(val[1:len(val)-1], ",")
	x := cast.ToInt(ss[0])
	y := cast.ToInt(ss[1])
	return Point{x, y}
}

func TestDefaultSpringContext_TypeConverter(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	b := &EnvEnumBean{}
	ctx.RegisterBean(b)

	ctx.SetProperty("env.type", "test")

	p := &PointBean{}
	ctx.RegisterBean(p)

	if false { // 不是函数
		ctx.RegisterTypeConverter(3)
	}

	if false { // 参数太多
		ctx.RegisterTypeConverter(func(_ string, _ string) Point {
			return Point{}
		})
	}

	if false { // 返回值太多
		ctx.RegisterTypeConverter(func(_ string) (Point, Point) {
			return Point{}, Point{}
		})
	}

	ctx.RegisterTypeConverter(PointConverter)

	ctx.SetProperty("point", "(7,5)")

	ctx.AutoWireBeans()

	if b.EnvType == ENV_TEST {
		fmt.Println("ok")
	}

	fmt.Printf("%+v\n", b)
	fmt.Printf("%+v\n", p)
}

type Grouper interface {
	Group()
}

type MyGrouper struct {
}

func (g *MyGrouper) Group() {

}

type ProxyGrouper struct {
	Grouper `autowire:""`
}

func TestDefaultSpringContext_NestedBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(new(MyGrouper))
	ctx.RegisterBean(new(ProxyGrouper))

	ctx.AutoWireBeans()
}

type Pkg interface {
	Package()
}

type SamePkgHolder struct {
	//Pkg `autowire:""` // 这种方式会找到多个符合条件的 Bean
	Pkg `autowire:"github.com/go-spring/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg:*pkg.SamePkg"`
}

func TestDefaultSpringContext_SameNameBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(new(SamePkgHolder))

	ctx.RegisterBean(&pkg1.SamePkg{})
	ctx.RegisterBean(&pkg2.SamePkg{})

	ctx.AutoWireBeans()
}

type DiffPkgOne struct {
}

func (d *DiffPkgOne) Package() {
	fmt.Println("github.com/go-spring/go-spring/spring-core_test/SpringCore_test.DiffPkgOne")
}

type DiffPkgTwo struct {
}

func (d *DiffPkgTwo) Package() {
	fmt.Println("github.com/go-spring/go-spring/spring-core_test/SpringCore_test.DiffPkgTwo")
}

type DiffPkgHolder struct {
	//Pkg `autowire:"same"` // 如果两个 Bean 不小心重名了，也会找到多个符合条件的 Bean
	Pkg `autowire:"github.com/go-spring/go-spring/spring-core_test/SpringCore_test.DiffPkgTwo:same"`
}

func TestDefaultSpringContext_DiffNameBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterNameBean("same", &DiffPkgOne{})
	ctx.RegisterNameBean("same", &DiffPkgTwo{})

	ctx.RegisterBean(new(DiffPkgHolder))

	ctx.AutoWireBeans()
}

func TestDefaultSpringContext_LoadProperties(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()
	ctx.LoadProperties("testdata/config/application.properties")
	appName := ctx.GetStringProperty("spring.application.name")
	fmt.Println("spring.application.name=" + appName)
}
