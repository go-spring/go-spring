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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-spring/spring-core"
	pkg1 "github.com/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/testdata/pkg/foo"
	"github.com/go-spring/spring-utils"
)

// ToString 对象转 Json 字符串
func ToString(i interface{}) string {
	bytes, err := json.Marshal(i)
	if err != nil {
		return err.Error()
	}
	return string(bytes)
}

func TestApplicationContext_RegisterBeanFrozen(t *testing.T) {
	SpringUtils.AssertPanic(t, func() {
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(new(int)).Init(func(i *int) {
			// 不能在这里注册新的 Bean
			ctx.ObjBean(new(bool))
		})
		ctx.AutoWireBeans()
	}, "bean registration have been frozen")
}

func TestApplicationContext(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()

		e := int(3)
		a := []int{3}

		// 普通类型用属性注入
		SpringUtils.AssertPanic(t, func() {
			ctx.ObjBean(e)
		}, "bean must be ref type")

		ctx.ObjBean(&e)

		// 这种错误延迟到 AutoWireBeans 阶段
		// // 相同类型的匿名 bean 不能重复注册
		// SpringUtils.AssertPanic(t, func() {
		//	 ctx.ObjBean(&e)
		// }, "duplicate registration, bean: \"int:\\*int\"")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).WithName("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).WithName("i4")

		ctx.ObjBean(a)
		ctx.ObjBean(&a)

		ctx.AutoWireBeans()

		SpringUtils.AssertPanic(t, func() {
			var i int
			ctx.GetBean(&i)
		}, "receiver must be ref type, bean: \"\\?\" field: ")

		// 找到多个符合条件的值
		SpringUtils.AssertPanic(t, func() {
			var i *int
			ctx.GetBean(&i)
		}, "found 3 beans, bean: \"\\?\" field:  type: \\*int")

		// 入参不是可赋值的对象
		SpringUtils.AssertPanic(t, func() {
			var i int
			ctx.GetBean(&i, "i3")
			fmt.Println(i)
		}, "receiver must be ref type, bean: \"i3\\?\" field: ")

		{
			var i *int
			// 直接使用缓存
			ctx.GetBean(&i, "i3")
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
	})

	/////////////////////////////////////////
	// 自定义数据类型

	t.Run("pkg1.SamePkg", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()

		e := pkg1.SamePkg{}
		a := []pkg1.SamePkg{{}}
		p := []*pkg1.SamePkg{{}}

		// 栈上的对象不能注册
		SpringUtils.AssertPanic(t, func() {
			ctx.ObjBean(e)
		}, "bean must be ref type")

		ctx.ObjBean(&e)

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).WithName("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).WithName("i4")

		ctx.ObjBean(a)
		ctx.ObjBean(&a)
		ctx.ObjBean(p)
		ctx.ObjBean(&p)

		ctx.AutoWireBeans()
	})

	t.Run("pkg2.SamePkg", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()

		e := pkg2.SamePkg{}
		a := []pkg2.SamePkg{{}}
		p := []*pkg2.SamePkg{{}}

		// 栈上的对象不能注册
		SpringUtils.AssertPanic(t, func() {
			ctx.ObjBean(e)
		}, "bean must be ref type")

		ctx.ObjBean(&e)

		// 相同类型不同名称的 bean 都可注册
		// 不同类型相同名称的 bean 也可注册
		ctx.ObjBean(&e).WithName("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).WithName("i4")

		ctx.ObjBean(a)
		ctx.ObjBean(&a)
		ctx.ObjBean(p)
		ctx.ObjBean(&p)

		ctx.ObjBean(&e).WithName("i5")

		ctx.AutoWireBeans()
	})
}

type TestBinding struct {
	i int
}

func (b *TestBinding) String() string {
	if b == nil {
		return ""
	} else {
		return strconv.Itoa(b.i)
	}
}

type TestObject struct {
	// 基础类型指针
	IntPtrByType *int `inject:""`
	IntPtrByName *int `autowire:"${key_1:=int_ptr}"`

	// 基础类型数组
	// IntSliceByType []int `autowire:""` // 多实例
	IntSliceByName1 []int `autowire:"int_slice_1"`
	IntSliceByName2 []int `autowire:"int_slice_2"`

	// 基础类型指针数组
	IntPtrSliceByType []*int `inject:""`
	IntPtrCollection  []*int `autowire:"${key_2:=[int_ptr]}"`
	IntPtrSliceByName []*int `autowire:"int_ptr_slice"`

	// 自定义类型指针
	StructByType *TestBinding `inject:""`
	StructByName *TestBinding `autowire:"struct_ptr"`

	// 自定义类型数组
	StructSliceByType []TestBinding `inject:""`
	StructSliceByName []TestBinding `autowire:"struct_slice"`

	// 自定义类型指针数组
	StructPtrSliceByType []*TestBinding `inject:""`
	StructPtrCollection  []*TestBinding `autowire:"[]"`
	StructPtrSliceByName []*TestBinding `autowire:"struct_ptr_slice"`

	// 接口
	InterfaceByType fmt.Stringer `inject:""`
	InterfaceByName fmt.Stringer `autowire:"struct_ptr"`

	// 接口数组
	InterfaceSliceByType []fmt.Stringer `autowire:""`

	InterfaceCollection  []fmt.Stringer `inject:"[]"`
	InterfaceCollection2 []fmt.Stringer `autowire:"[]"`

	// 指定名称时使用精确匹配模式，不对数组元素进行转换，即便能做到似乎也无意义
	InterfaceSliceByName []fmt.Stringer `autowire:"struct_ptr_slice?"`

	MapTyType map[string]interface{} `inject:""`
	MapByName map[string]interface{} `autowire:"map"`
}

func TestApplicationContext_AutoWireBeans(t *testing.T) {

	t.Run("wired error", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()

		obj := &TestObject{}
		ctx.ObjBean(obj)

		i := int(3)
		ctx.ObjBean(&i).WithName("int_ptr")

		i2 := int(3)
		ctx.ObjBean(&i2).WithName("int_ptr_2")

		SpringUtils.AssertPanic(t, func() {
			ctx.AutoWireBeans()
		}, "found 2 beans, bean: \"\" field: TestObject.\\$IntPtrByType type: \\*int")
	})

	ctx := SpringCore.NewApplicationContext()

	obj := &TestObject{}
	ctx.ObjBean(obj)

	i := int(3)
	ctx.ObjBean(&i).WithName("int_ptr")

	is := []int{1, 2, 3}
	ctx.ObjBean(is).WithName("int_slice_1")

	is2 := []int{2, 3, 4}
	ctx.ObjBean(is2).WithName("int_slice_2")

	i2 := 4
	ips := []*int{&i2}
	ctx.ObjBean(ips).WithName("int_ptr_slice")

	b := TestBinding{1}
	ctx.ObjBean(&b).WithName("struct_ptr").Export((*fmt.Stringer)(nil))

	bs := []TestBinding{{10}}
	ctx.ObjBean(bs).WithName("struct_slice")

	b2 := TestBinding{2}
	bps := []*TestBinding{&b2}
	ctx.ObjBean(bps).WithName("struct_ptr_slice")

	s := []fmt.Stringer{&TestBinding{3}}
	ctx.ObjBean(s)

	m := map[string]interface{}{
		"5": 5,
	}

	ctx.ObjBean(m).WithName("map")

	f1 := float32(11.0)
	ctx.ObjBean(&f1).WithName("float_ptr_1")

	f2 := float32(12.0)
	ctx.ObjBean(&f2).WithName("float_ptr_2")

	ctx.AutoWireBeans()

	var ff []*float32
	ctx.CollectBeans(&ff, "float_ptr_2", "float_ptr_1")
	SpringUtils.AssertEqual(t, ff, []*float32{&f2, &f1})

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
	// IntPtr     *int `value:"${int}"` // 不支持指针

	Uint        uint `value:"${uint}"`
	DefaultUint uint `value:"${default.uint:=2}"`

	Float        float32 `value:"${float}"`
	DefaultFloat float32 `value:"${default.float:=2}"`

	// Complex complex64 `value:"${complex}"` // 不支持复数

	String        string `value:"${string}"`
	DefaultString string `value:"${default.string:=2}"`

	Bool        bool `value:"${bool}"`
	DefaultBool bool `value:"${default.bool:=false}"`

	SubSetting SubSetting `value:"${sub}"`
	// SubSettingPtr *SubSetting `value:"${sub}"` // 不支持指针

	SubSubSetting SubSubSetting `value:"${sub_sub}"`

	IntSlice    []int    `value:"${int_slice}"`
	StringSlice []string `value:"${string_slice}"`
	// FloatSlice  []float64 `value:"${float_slice}"`
}

func TestApplicationContext_ValueTag(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()

	ctx.SetProperty("int", int(3))
	ctx.SetProperty("uint", uint(3))
	ctx.SetProperty("float", float32(3))
	ctx.SetProperty("complex", complex(3, 0))
	ctx.SetProperty("string", "3")
	ctx.SetProperty("bool", true)

	setting := &Setting{}
	ctx.ObjBean(setting)

	ctx.SetProperty("sub.int", int(4))
	ctx.SetProperty("sub.sub.int", int(5))
	ctx.SetProperty("sub_sub.int", int(6))

	ctx.SetProperty("int_slice", []int{1, 2})
	ctx.SetProperty("string_slice", []string{"1", "2"})
	// ctx.SetProperty("float_slice", []float64{1, 2})

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
	Ctx SpringCore.ApplicationContext `autowire:""`
}

func (f *PrototypeBeanFactory) New(name string) *PrototypeBean {
	b := &PrototypeBean{
		name: name,
		t:    time.Now(),
	}

	// PrototypeBean 依赖的服务可以通过 ApplicationContext 注入
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

func TestApplicationContext_PrototypeBean(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.ObjBean(ctx).Export((*SpringCore.ApplicationContext)(nil))

	gs := &GreetingService{}
	ctx.ObjBean(gs)

	s := &PrototypeBeanService{}
	ctx.ObjBean(s)

	f := &PrototypeBeanFactory{}
	ctx.ObjBean(f)

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

type PointBean struct {
	Point        image.Point   `value:"${point}"`
	DefaultPoint image.Point   `value:"${default_point:=(3,4)}"`
	PointList    []image.Point `value:"${point.list}"`
}

func TestApplicationContext_TypeConverter(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.LoadProperties("testdata/config/application.yaml")

	b := &EnvEnumBean{}
	ctx.ObjBean(b)

	ctx.SetProperty("env.type", "test")

	p := &PointBean{}
	ctx.ObjBean(p)

	ctx.AddTypeConverter(PointConverter)
	ctx.SetProperty("point", "(7,5)")

	dbConfig := &DbConfig{}
	ctx.ObjBean(dbConfig)

	ctx.AutoWireBeans()

	SpringUtils.AssertEqual(t, b.EnvType, ENV_TEST)

	fmt.Printf("%+v\n", b)
	fmt.Printf("%+v\n", p)

	fmt.Printf("%+v\n", dbConfig)
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

func TestApplicationContext_NestedBean(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.ObjBean(new(MyGrouper)).Export((*Grouper)(nil))
	ctx.ObjBean(new(ProxyGrouper))
	ctx.AutoWireBeans()
}

type Pkg interface {
	Package()
}

type SamePkgHolder struct {
	// Pkg `autowire:""` // 这种方式会找到多个符合条件的 Bean
	Pkg `autowire:"github.com/go-spring/spring-core/testdata/pkg/bar/pkg.SamePkg:*pkg.SamePkg"`
}

func TestApplicationContext_SameNameBean(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.ObjBean(new(SamePkgHolder))
	ctx.ObjBean(&pkg1.SamePkg{}).Export((*Pkg)(nil))
	ctx.ObjBean(&pkg2.SamePkg{}).Export((*Pkg)(nil))
	ctx.AutoWireBeans()
}

type DiffPkgOne struct {
}

func (d *DiffPkgOne) Package() {
	fmt.Println("github.com/go-spring/spring-core_test/SpringCore_test.DiffPkgOne")
}

type DiffPkgTwo struct {
}

func (d *DiffPkgTwo) Package() {
	fmt.Println("github.com/go-spring/spring-core_test/SpringCore_test.DiffPkgTwo")
}

type DiffPkgHolder struct {
	// Pkg `autowire:"same"` // 如果两个 Bean 不小心重名了，也会找到多个符合条件的 Bean
	Pkg `autowire:"github.com/go-spring/spring-core_test/SpringCore_test.DiffPkgTwo:same"`
}

func TestApplicationContext_DiffNameBean(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.ObjBean(&DiffPkgOne{}).WithName("same").Export((*Pkg)(nil))
	ctx.ObjBean(&DiffPkgTwo{}).WithName("same").Export((*Pkg)(nil))
	ctx.ObjBean(new(DiffPkgHolder))
	ctx.AutoWireBeans()
}

func TestApplicationContext_LoadProperties(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.LoadProperties("testdata/config/application.yaml")
	ctx.LoadProperties("testdata/config/application.properties")

	val0 := ctx.GetProperty("spring.application.name")
	SpringUtils.AssertEqual(t, val0, "test")

	val1 := ctx.GetProperty("yaml.list")
	SpringUtils.AssertEqual(t, val1, []interface{}{1, 2})
}

type BeanZero struct {
	Int int
}

type BeanOne struct {
	Zero *BeanZero `autowire:""`
}

type BeanTwo struct {
	One *BeanOne `autowire:""`
}

func (t *BeanTwo) Group() {
}

type BeanThree struct {
	One *BeanTwo `autowire:""`
}

func (t *BeanThree) String() string {
	return ""
}

func TestApplicationContext_GetBean(t *testing.T) {

	t.Run("panic", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.AutoWireBeans()

		SpringUtils.AssertPanic(t, func() {
			var i int
			ctx.GetBean(i)
		}, "i must be pointer")

		SpringUtils.AssertPanic(t, func() {
			var i *int
			ctx.GetBean(i)
		}, "receiver must be ref type")

		SpringUtils.AssertPanic(t, func() {
			i := new(int)
			ctx.GetBean(i)
		}, "receiver must be ref type")

		var i *int
		ctx.GetBean(&i)

		SpringUtils.AssertPanic(t, func() {
			var is []int
			ctx.GetBean(is)
		}, "i must be pointer")

		var a []int
		ctx.GetBean(&a)

		SpringUtils.AssertPanic(t, func() {
			var s fmt.Stringer
			ctx.GetBean(s)
		}, "i can't be nil")

		var s fmt.Stringer
		ctx.GetBean(&s)
	})

	t.Run("success", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanTwo)).Export((*Grouper)(nil))
		ctx.AutoWireBeans()

		var two *BeanTwo
		ok := ctx.GetBean(&two)
		SpringUtils.AssertEqual(t, ok, true)

		var grouper Grouper
		ok = ctx.GetBean(&grouper)
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, (*BeanTwo)(nil))
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, (*BeanTwo)(nil))
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, "")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, "*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "BeanTwo")
		SpringUtils.AssertEqual(t, ok, false)

		ok = ctx.GetBean(&grouper, "BeanTwo")
		SpringUtils.AssertEqual(t, ok, false)

		ok = ctx.GetBean(&two, ":*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, ":*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "github.com/go-spring/spring-core_test/SpringCore_test.BeanTwo:*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, "github.com/go-spring/spring-core_test/SpringCore_test.BeanTwo:*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "xxx:*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, false)

		ok = ctx.GetBean(&grouper, "xxx:*SpringCore_test.BeanTwo")
		SpringUtils.AssertEqual(t, ok, false)

		var three *BeanThree
		ok = ctx.GetBean(&three, "")
		SpringUtils.AssertEqual(t, ok, false)

		fmt.Println(ToString(two))
	})
}

func TestApplicationContext_FindBeanByName(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()

	ctx.ObjBean(&BeanZero{5})
	ctx.ObjBean(new(BeanOne))
	ctx.ObjBean(new(BeanTwo))

	ctx.AutoWireBeans()

	SpringUtils.AssertPanic(t, func() {
		ctx.FindBean("")
	}, "found 3 beans, bean: \"\"")

	i, ok := ctx.FindBean("*SpringCore_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	SpringUtils.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean("BeanTwo")
	fmt.Println(ToString(i))
	SpringUtils.AssertEqual(t, ok, false)

	i, ok = ctx.FindBean(":*SpringCore_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	SpringUtils.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean("github.com/go-spring/spring-core_test/SpringCore_test.BeanTwo:*SpringCore_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	SpringUtils.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean("xxx:*SpringCore_test.BeanTwo")
	fmt.Println(ToString(i))
	SpringUtils.AssertEqual(t, ok, false)

	i, ok = ctx.FindBean("*SpringCore_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	SpringUtils.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean((*BeanTwo)(nil))
	fmt.Println(ToString(i.Bean()))
	SpringUtils.AssertEqual(t, ok, true)

	_, ok = ctx.FindBean((*fmt.Stringer)(nil))
	SpringUtils.AssertEqual(t, ok, false)

	_, ok = ctx.FindBean((*Grouper)(nil))
	SpringUtils.AssertEqual(t, ok, false)
}

func TestApplicationContext_RegisterBeanFn(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("room", "Class 3 Grade 1")

	// 用接口注册时实际使用的是原始类型
	ctx.ObjBean(Teacher(newHistoryTeacher(""))).Export((*Teacher)(nil))

	ctx.CtorBean(NewStudent, "", "${room}").WithName("st1")
	ctx.CtorBean(NewPtrStudent, "1:${room}").WithName("st2")
	ctx.CtorBean(NewStudent, "?", "${room:=http://}").WithName("st3")
	ctx.CtorBean(NewPtrStudent, "0:?", "1:${room:=4567}").WithName("st4")

	mapFn := func() map[int]string {
		return map[int]string{
			1: "ok",
		}
	}

	ctx.CtorBean(mapFn)

	sliceFn := func() []int {
		return []int{1, 2}
	}

	ctx.CtorBean(sliceFn)

	ctx.AutoWireBeans()

	var st1 *Student
	ok := ctx.GetBean(&st1, "st1")

	SpringUtils.AssertEqual(t, ok, true)
	fmt.Println(ToString(st1))
	SpringUtils.AssertEqual(t, st1.Room, ctx.GetProperty("room"))

	var st2 *Student
	ok = ctx.GetBean(&st2, "st2")

	SpringUtils.AssertEqual(t, ok, true)
	fmt.Println(ToString(st2))
	SpringUtils.AssertEqual(t, st2.Room, ctx.GetProperty("room"))

	fmt.Printf("%x\n", reflect.ValueOf(st1).Pointer())
	fmt.Printf("%x\n", reflect.ValueOf(st2).Pointer())

	var st3 *Student
	ok = ctx.GetBean(&st3, "st3")

	SpringUtils.AssertEqual(t, ok, true)
	fmt.Println(ToString(st3))
	SpringUtils.AssertEqual(t, st3.Room, ctx.GetProperty("room"))

	var st4 *Student
	ok = ctx.GetBean(&st4, "st4")

	SpringUtils.AssertEqual(t, ok, true)
	fmt.Println(ToString(st4))
	SpringUtils.AssertEqual(t, st4.Room, ctx.GetProperty("room"))

	var m map[int]string
	ok = ctx.GetBean(&m)

	SpringUtils.AssertEqual(t, ok, true)
	fmt.Println(ToString(m))
	SpringUtils.AssertEqual(t, m[1], "ok")

	var s []int
	ok = ctx.GetBean(&s)

	SpringUtils.AssertEqual(t, ok, true)
	fmt.Println(ToString(s))
	SpringUtils.AssertEqual(t, s[1], 2)
}

func TestApplicationContext_Profile(t *testing.T) {

	t.Run("bean:_ctx:", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		SpringUtils.AssertEqual(t, ok, true)
	})

	t.Run("bean:_ctx:test", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProfile("test")
		ctx.ObjBean(&BeanZero{5})
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		SpringUtils.AssertEqual(t, ok, true)
	})
}

type BeanFour struct{}

func TestApplicationContext_DependsOn(t *testing.T) {

	t.Run("random", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanFour))
		ctx.AutoWireBeans()
	})

	t.Run("dependsOn", func(t *testing.T) {

		dependsOn := []SpringCore.BeanSelector{
			(*BeanOne)(nil), // 通过类型定义查找
			"github.com/go-spring/spring-core_test/SpringCore_test.BeanZero:*SpringCore_test.BeanZero",
		}

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanFour)).DependsOn(dependsOn...)
		ctx.AutoWireBeans()
	})
}

func TestApplicationContext_Primary(t *testing.T) {

	t.Run("duplicate", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(&BeanZero{5})
			ctx.ObjBean(&BeanZero{6})
			ctx.ObjBean(new(BeanOne))
			ctx.ObjBean(new(BeanTwo))
			ctx.AutoWireBeans()
		}, "duplicate registration, bean: ")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(&BeanZero{5})
			// Primary 是在多个候选 bean 里面选择，而不是允许同名同类型的两个 bean
			ctx.ObjBean(&BeanZero{6}).Primary(true)
			ctx.ObjBean(new(BeanOne))
			ctx.ObjBean(new(BeanTwo))
			ctx.AutoWireBeans()
		}, "duplicate registration, bean: ")
	})

	t.Run("not primary", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanTwo))
		ctx.AutoWireBeans()

		var b *BeanTwo
		ctx.GetBean(&b)
		SpringUtils.AssertEqual(t, b.One.Zero.Int, 5)
	})

	t.Run("primary", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(&BeanZero{6}).WithName("zero_6").Primary(true)
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanTwo))
		ctx.AutoWireBeans()

		var b *BeanTwo
		ctx.GetBean(&b)
		SpringUtils.AssertEqual(t, b.One.Zero.Int, 6)
	})
}

type FuncObj struct {
	Fn func(int) int `autowire:""`
}

func TestDefaultProperties_WireFunc(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.ObjBean(func(int) int {
		return 6
	})
	obj := new(FuncObj)
	ctx.ObjBean(obj)
	ctx.AutoWireBeans()
	i := obj.Fn(3)
	SpringUtils.AssertEqual(t, i, 6)
}

type Manager interface {
	Cluster() string
}

func NewManager() Manager {
	return localManager{}
}

func NewManagerRetError() (Manager, error) {
	return localManager{}, errors.New("error")
}

func NewManagerRetErrorNil() (Manager, error) {
	return localManager{}, nil
}

func NewNullPtrManager() Manager {
	return nil
}

func NewPtrManager() Manager {
	return &localManager{}
}

type localManager struct {
	Version string `value:"${manager.version}"`
}

func (m localManager) Cluster() string {
	return "local"
}

func NewInt() int {
	return 32
}

func TestApplicationContext_RegisterBeanFn2(t *testing.T) {

	t.Run("ptr manager", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.CtorBean(NewPtrManager)
		ctx.CtorBean(NewInt)
		ctx.AutoWireBeans()

		var m Manager
		ok := ctx.GetBean(&m)
		SpringUtils.AssertEqual(t, ok, true)

		// 因为用户是按照接口注册的，所以理论上在依赖
		// 系统中用户并不关心接口对应的真实类型是什么。
		var lm *localManager
		ok = ctx.GetBean(&lm)
		SpringUtils.AssertEqual(t, ok, false)
	})

	t.Run("manager", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("manager.version", "1.0.0")

		bd := ctx.CtorBean(NewManager)
		SpringUtils.AssertEqual(t, bd.Name(), "SpringCore_test.Manager")

		bd = ctx.CtorBean(NewInt)
		SpringUtils.AssertEqual(t, bd.Name(), "*int")

		ctx.AutoWireBeans()

		var m Manager
		ok := ctx.GetBean(&m)
		SpringUtils.AssertEqual(t, ok, true)

		var lm *localManager
		ok = ctx.GetBean(&lm)
		SpringUtils.AssertEqual(t, ok, false)
	})

	t.Run("manager return error", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("manager.version", "1.0.0")
			ctx.CtorBean(NewManagerRetError)
			ctx.AutoWireBeans()
		}, "return error")
	})

	t.Run("manager return error nil", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.CtorBean(NewManagerRetErrorNil)
		ctx.AutoWireBeans()
	})

	t.Run("manager return nil", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("manager.version", "1.0.0")
			ctx.CtorBean(NewNullPtrManager)
			ctx.AutoWireBeans()
		}, "return nil")
	})
}

type destroyable interface {
	Init()
	Destroy()
	InitWithArg(s string)
	DestroyWithArg(s string)
	InitWithError(i int) error
	DestroyWithError(i int) error
}

type callDestroy struct {
	inited    bool
	destroyed bool
}

func (d *callDestroy) Init() {
	d.inited = true
}

func (d *callDestroy) Destroy() {
	d.destroyed = true
}

func (d *callDestroy) InitWithArg(s string) {
	d.inited = true
}

func (d *callDestroy) DestroyWithArg(s string) {
	d.destroyed = true
}

func (d *callDestroy) InitWithError(i int) error {
	if i == 0 {
		d.inited = true
		return nil
	}
	return errors.New("error")
}

func (d *callDestroy) DestroyWithError(i int) error {
	if i == 0 {
		d.destroyed = true
		return nil
	}
	return errors.New("error")
}

type nestedCallDestroy struct {
	callDestroy
}

type nestedDestroyable struct {
	destroyable
}

func TestRegisterBean_InitFunc(t *testing.T) {

	t.Run("int", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func() {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func() int { return 0 })
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func(int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func(int, int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(new(int)).Init(func(i *int) { *i = 3 })
		ctx.AutoWireBeans()

		var i *int
		ctx.GetBean(&i)
		SpringUtils.AssertEqual(t, *i, 3)
	})

	t.Run("call init method", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(new(callDestroy)).Init((*callDestroy).Init)
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.inited, true)
	})

	t.Run("call init method with arg", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.ObjBean(new(callDestroy)).Init((*callDestroy).InitWithArg, "${version}")
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.inited, true)
	})

	t.Run("call init method with error", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.ObjBean(new(callDestroy)).Init((*callDestroy).InitWithError, "${int}")
			ctx.AutoWireBeans()
		}, "error")

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("int", 0)
		ctx.ObjBean(new(callDestroy)).Init((*callDestroy).InitWithError, "${int}")
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.inited, true)
	})

	t.Run("call interface init method", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.Init)
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.(*callDestroy).inited, true)
	})

	t.Run("call interface init method with arg", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithArg, "${version}")
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.(*callDestroy).inited, true)
	})

	t.Run("call interface init method with error", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithError, "${int}")
			ctx.AutoWireBeans()
		}, "error")

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("int", 0)
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithError, "${int}")
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.(*callDestroy).inited, true)
	})

	t.Run("call nested init method", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(new(nestedCallDestroy)).Init((*nestedCallDestroy).Init)
		ctx.AutoWireBeans()

		var d *nestedCallDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.inited, true)
	})

	t.Run("call nested interface init method", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&nestedDestroyable{
			destroyable: new(callDestroy),
		}).Init((*nestedDestroyable).Init)
		ctx.AutoWireBeans()

		var d *nestedDestroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.destroyable.(*callDestroy).inited, true)
	})
}

type RedisCluster struct {
	Endpoints string `value:"${redis.endpoints}"`

	RedisConfig struct {
		Endpoints string `value:"${redis.endpoints}"`
	}

	Nested struct {
		RedisConfig struct {
			Endpoints string `value:"${redis.endpoints}"`
		}
	}
}

func TestApplicationContext_ValueBinding(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("redis.endpoints", "redis://localhost:6379")
	ctx.ObjBean(new(RedisCluster))
	ctx.AutoWireBeans()

	var cluster *RedisCluster
	ctx.GetBean(&cluster)
	fmt.Println(cluster)

	SpringUtils.AssertEqual(t, cluster.Endpoints, cluster.RedisConfig.Endpoints)
	SpringUtils.AssertEqual(t, cluster.Endpoints, cluster.Nested.RedisConfig.Endpoints)
}

func TestApplicationContext_CollectBeans(t *testing.T) {

	t.Run("more than one *", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("redis.endpoints", "redis://localhost:6379")
		ctx.ObjBean(new(RedisCluster)).WithName("one")
		ctx.ObjBean(new(RedisCluster))
		ctx.AutoWireBeans()

		SpringUtils.AssertPanic(t, func() {
			var rcs []*RedisCluster
			ctx.CollectBeans(&rcs, "*", "*")
		}, "more than one \\* in collection \\[\\*,\\*]\\?")
	})

	t.Run("before *", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("redis.endpoints", "redis://localhost:6379")
		d1 := ctx.ObjBean(new(RedisCluster)).WithName("one")
		d2 := ctx.ObjBean(new(RedisCluster))
		ctx.AutoWireBeans()

		var rcs []*RedisCluster
		ctx.CollectBeans(&rcs, "one", "*")

		SpringUtils.AssertEqual(t, len(rcs), 2)
		SpringUtils.AssertEqual(t, rcs[0], d1.Bean())
		SpringUtils.AssertEqual(t, rcs[1], d2.Bean())
	})

	t.Run("after *", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("redis.endpoints", "redis://localhost:6379")
		d1 := ctx.ObjBean(new(RedisCluster)).WithName("one")
		d2 := ctx.ObjBean(new(RedisCluster))
		ctx.AutoWireBeans()

		var rcs []*RedisCluster
		ctx.CollectBeans(&rcs, "one", "*")

		SpringUtils.AssertEqual(t, len(rcs), 2)
		SpringUtils.AssertEqual(t, rcs[1], d1.Bean())
		SpringUtils.AssertEqual(t, rcs[0], d2.Bean())
	})

	t.Run("only *", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("redis.endpoints", "redis://localhost:6379")
		ctx.ObjBean(new(RedisCluster)).WithName("one")
		ctx.ObjBean(new(RedisCluster))
		ctx.AutoWireBeans()

		var rcs []*RedisCluster
		ctx.CollectBeans(&rcs, "*")

		SpringUtils.AssertEqual(t, len(rcs), 2)
	})

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("redis.endpoints", "redis://localhost:6379")

	ctx.ObjBean([]*RedisCluster{new(RedisCluster)})
	ctx.ObjBean([]RedisCluster{{}})
	ctx.ObjBean(new(RedisCluster))

	intBean := ctx.ObjBean(new(int)).Init(func(*int) {

		var rcs []*RedisCluster
		ctx.CollectBeans(&rcs)
		fmt.Println(ToString(rcs))

		SpringUtils.AssertEqual(t, len(rcs), 2)
		SpringUtils.AssertEqual(t, rcs[0].Endpoints, "redis://localhost:6379")
	})
	SpringUtils.AssertEqual(t, intBean.Name(), "*int")

	ctx.AutoWireBeans()

	var rcs []RedisCluster
	ctx.GetBean(&rcs)
	fmt.Println(ToString(rcs))

	SpringUtils.AssertEqual(t, len(rcs), 1)
	SpringUtils.AssertEqual(t, rcs[0].Endpoints, "redis://localhost:6379")
}

func TestApplicationContext_WireSliceBean(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("redis.endpoints", "redis://localhost:6379")
	ctx.ObjBean([]*RedisCluster{new(RedisCluster)})
	ctx.ObjBean([]RedisCluster{{}})
	ctx.AutoWireBeans()

	{
		var rcs []*RedisCluster
		ctx.GetBean(&rcs)
		fmt.Println(ToString(rcs))

		SpringUtils.AssertEqual(t, rcs[0].Endpoints, "redis://localhost:6379")
	}

	{
		var rcs []RedisCluster
		ctx.GetBean(&rcs)
		fmt.Println(ToString(rcs))

		SpringUtils.AssertEqual(t, rcs[0].Endpoints, "redis://localhost:6379")
	}
}

var defaultClassOption = ClassOption{
	className: "default",
}

type ClassOption struct {
	className string
	students  []*Student
	floor     int
}

type ClassOptionFunc func(opt *ClassOption)

func withClassName(className string, floor int) ClassOptionFunc {
	return func(opt *ClassOption) {
		opt.className = className
		opt.floor = floor
	}
}

func withStudents(students []*Student) ClassOptionFunc {
	return func(opt *ClassOption) {
		opt.students = students
	}
}

type ClassRoom struct {
	President string `value:"${president}"`
	className string
	floor     int
	students  []*Student
	desktop   Desktop
}

type Desktop interface {
}

type MetalDesktop struct {
}

func (cls *ClassRoom) Desktop() Desktop {
	return cls.desktop
}

func NewClassRoom(options ...ClassOptionFunc) ClassRoom {
	opt := defaultClassOption
	for _, fn := range options {
		fn(&opt)
	}
	return ClassRoom{
		className: opt.className,
		students:  opt.students,
		floor:     opt.floor,
		desktop:   &MetalDesktop{},
	}
}

func TestOptionPattern(t *testing.T) {

	students := []*Student{
		new(Student), new(Student),
	}

	cls := NewClassRoom()
	SpringUtils.AssertEqual(t, cls.className, "default")

	cls = NewClassRoom(withClassName("二年级03班", 3))
	SpringUtils.AssertEqual(t, cls.floor, 3)
	SpringUtils.AssertEqual(t, len(cls.students), 0)
	SpringUtils.AssertEqual(t, cls.className, "二年级03班")

	cls = NewClassRoom(withStudents(students))
	SpringUtils.AssertEqual(t, cls.floor, 0)
	SpringUtils.AssertEqual(t, cls.students, students)
	SpringUtils.AssertEqual(t, cls.className, "default")

	cls = NewClassRoom(withClassName("二年级03班", 3), withStudents(students))
	SpringUtils.AssertEqual(t, cls.className, "二年级03班")
	SpringUtils.AssertEqual(t, cls.students, students)
	SpringUtils.AssertEqual(t, cls.floor, 3)

	cls = NewClassRoom(withStudents(students), withClassName("二年级03班", 3))
	SpringUtils.AssertEqual(t, cls.className, "二年级03班")
	SpringUtils.AssertEqual(t, cls.students, students)
	SpringUtils.AssertEqual(t, cls.floor, 3)
}

func TestOptionConstructorArg(t *testing.T) {

	t.Run("option default", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom).Options()
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		SpringUtils.AssertEqual(t, len(cls.students), 0)
		SpringUtils.AssertEqual(t, cls.className, "default")
		SpringUtils.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom).Options(
			SpringCore.NewOptionArg(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}"),
		)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		SpringUtils.AssertEqual(t, cls.floor, 3)
		SpringUtils.AssertEqual(t, len(cls.students), 0)
		SpringUtils.AssertEqual(t, cls.className, "二年级03班")
		SpringUtils.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("class_name", "二年级03班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom).Options(
			SpringCore.NewOptionArg(withStudents, ""),
		)
		ctx.ObjBean([]*Student{
			new(Student), new(Student),
		})
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		SpringUtils.AssertEqual(t, cls.floor, 0)
		SpringUtils.AssertEqual(t, len(cls.students), 2)
		SpringUtils.AssertEqual(t, cls.className, "default")
		SpringUtils.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents withClassName", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("class_name", "二年级06班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom).Options(
			SpringCore.NewOptionArg(withStudents, ""),
			SpringCore.NewOptionArg(
				withClassName, // 故意写反
				"1:${class_floor:=3}",
				"0:${class_name:=二年级03班}",
			),
		)
		ctx.ObjBean([]*Student{
			new(Student), new(Student),
		})
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		SpringUtils.AssertEqual(t, cls.floor, 3)
		SpringUtils.AssertEqual(t, len(cls.students), 2)
		SpringUtils.AssertEqual(t, cls.className, "二年级06班")
		SpringUtils.AssertEqual(t, cls.President, "CaiYuanPei")
	})
}

type ServerInterface interface {
	Consumer() *Consumer
	ConsumerT() *Consumer
	ConsumerArg(i int) *Consumer
}

type Server struct {
	Version string `value:"${server.version}"`
}

func NewServerInterface() ServerInterface {
	return new(Server)
}

type Consumer struct {
	s *Server
}

func (s *Server) Consumer() *Consumer {
	if nil == s {
		panic(errors.New("server is nil"))
	}
	return &Consumer{s}
}

func (s *Server) ConsumerT() *Consumer {
	return s.Consumer()
}

func (s *Server) ConsumerArg(i int) *Consumer {
	if nil == s {
		panic(errors.New("server is nil"))
	}
	return &Consumer{s}
}

type Service struct {
	Consumer *Consumer `autowire:""`
}

func TestApplicationContext_RegisterMethodBean(t *testing.T) {

	t.Run("method bean", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.ObjBean(new(Server))

		// Method Bean 的默认名称要等到 Bean 真正注册的时候才能获取到
		bd := ctx.MethodBean(parent, "Consumer")
		ctx.AutoWireBeans()
		SpringUtils.AssertEqual(t, bd.Name(), "*SpringCore_test.Consumer")

		var s *Server
		ok := ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean arg", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.ObjBean(new(Server))
		ctx.MethodBean(parent, "ConsumerArg", "${i:=9}")
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean wire to other bean", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")

		// Name is SpringCore_test.ServerInterface
		parent := ctx.CtorBean(NewServerInterface)

		// Name is *SpringCore_test.Consumer
		ctx.MethodBean(parent, "Consumer").
			DependsOn("SpringCore_test.ServerInterface")

		// Name is *SpringCore_test.Service
		ctx.ObjBean(new(Service))
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("circle autowire", func(t *testing.T) {
		okCount := 0
		errCount := 0
		for i := 0; i < 20; i++ { // 不要排序
			func() {

				defer func() {
					if err := recover(); err != nil {
						errCount++

						var v string
						switch e := err.(type) {
						case error:
							v = e.Error()
						case string:
							v = e
						}

						if !strings.Contains(v, "found circle autowire") {
							panic(errors.New("test error"))
						}
					} else {
						okCount++
					}
				}()

				ctx := SpringCore.NewApplicationContext()
				ctx.SetProperty("server.version", "1.0.0")

				parent := ctx.ObjBean(new(Server)).
					DependsOn("*SpringCore_test.Service")

				ctx.MethodBean(parent, "Consumer").
					DependsOn("*SpringCore_test.Server")

				ctx.ObjBean(new(Service))
				ctx.AutoWireBeans()
			}()
		}
		fmt.Printf("ok:%d err:%d\n", okCount, errCount)
	})

	t.Run("method bean autowire", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.ObjBean(new(Server))
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")
	})

	t.Run("method bean selector type", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.ObjBean(new(Server))
		ctx.MethodBean((*Server)(nil), "Consumer")
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean selector type error", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(Server))
			ctx.MethodBean((fmt.Stringer)(nil), "Consumer")
			ctx.AutoWireBeans()
		}, "selector can't be nil or empty")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(Server))
			ctx.MethodBean((*int)(nil), "Consumer")
			ctx.AutoWireBeans()
		}, "can't find parent bean: \"\\*int\"")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(int))
			ctx.ObjBean(new(Server))
			ctx.MethodBean((*int)(nil), "Consumer")
			ctx.AutoWireBeans()
		}, "can't find method:Consumer on type:\\*int")
	})

	t.Run("method bean selector beanId", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.ObjBean(new(Server))
		ctx.MethodBean("*SpringCore_test.Server", "Consumer")
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean selector beanId error", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(Server))
			ctx.MethodBean("NULL", "Consumer")
			ctx.AutoWireBeans()
		}, "can't find parent bean: \"NULL\"")
	})

	t.Run("found 2 parent bean", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(Server)).WithName("s1")
			ctx.ObjBean(new(Server)).WithName("s2")
			ctx.MethodBeanFn((*Server).Consumer)
			ctx.AutoWireBeans()
		}, "found 2 parent bean")
	})
}

func TestApplicationContext_RegisterMethodBeanFn(t *testing.T) {

	t.Run("fn method bean", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		// Name is SpringCore_test.ServerInterface
		ctx.CtorBean(NewServerInterface)

		// Method Bean 的默认名称要等到 Bean 真正注册的时候才能获取到
		bd := ctx.MethodBeanFn(ServerInterface.ConsumerT)
		ctx.AutoWireBeans()
		SpringUtils.AssertEqual(t, bd.Name(), "*SpringCore_test.Consumer")

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("fn method bean arg", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		// Name is SpringCore_test.ServerInterface
		ctx.CtorBean(NewServerInterface)
		ctx.MethodBeanFn(ServerInterface.ConsumerArg, "${i:=9}")
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("fn method bean wire to other bean", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		// Name is SpringCore_test.ServerInterface
		ctx.CtorBean(NewServerInterface)
		// Name is *SpringCore_test.Consumer
		ctx.MethodBeanFn(ServerInterface.Consumer).
			DependsOn("SpringCore_test.ServerInterface")
		// Name is *SpringCore_test.Service
		ctx.ObjBean(new(Service))
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("fn circle autowire", func(t *testing.T) {
		okCount := 0
		errCount := 0
		for i := 0; i < 20; i++ { // 不要排序
			func() {

				defer func() {
					if err := recover(); err != nil {
						errCount++

						var v string
						switch e := err.(type) {
						case error:
							v = e.Error()
						case string:
							v = e
						}

						if !strings.Contains(v, "found circle autowire") {
							panic(errors.New("test error"))
						}
					} else {
						okCount++
					}
				}()

				ctx := SpringCore.NewApplicationContext()
				ctx.SetProperty("server.version", "1.0.0")
				// Name is SpringCore_test.ServerInterface
				ctx.CtorBean(NewServerInterface).
					DependsOn("*SpringCore_test.Service")
				// Name is *SpringCore_test.Consumer
				ctx.MethodBeanFn(ServerInterface.Consumer).
					DependsOn("SpringCore_test.ServerInterface")
				// Name is *SpringCore_test.Service
				ctx.ObjBean(new(Service))
				ctx.AutoWireBeans()
			}()
		}
		fmt.Printf("ok:%d err:%d\n", okCount, errCount)
	})

	t.Run("fn method bean autowire", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		// Name is SpringCore_test.ServerInterface
		ctx.CtorBean(NewServerInterface)
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")
	})

	t.Run("fn method bean selector type", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		// Name is SpringCore_test.ServerInterface
		ctx.CtorBean(NewServerInterface)
		ctx.MethodBeanFn(ServerInterface.ConsumerT)
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("fn method bean selector type error", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			// Name is SpringCore_test.ServerInterface
			ctx.CtorBean(NewServerInterface)
			ctx.MethodBean((fmt.Stringer)(nil), "Consumer")
			ctx.AutoWireBeans()
		}, "selector can't be nil or empty")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			// Name is SpringCore_test.ServerInterface
			ctx.CtorBean(NewServerInterface)
			ctx.MethodBean((*int)(nil), "Consumer")
			ctx.AutoWireBeans()
		}, "can't find parent bean: \"\\*int\"")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(int))
			// Name is SpringCore_test.ServerInterface
			ctx.CtorBean(NewServerInterface)
			ctx.MethodBean((*int)(nil), "Consumer")
			ctx.AutoWireBeans()
		}, "can't find method:Consumer on type:\\*int")
	})

	t.Run("fn method bean selector beanId", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		// Name is SpringCore_test.ServerInterface
		ctx.CtorBean(NewServerInterface)
		ctx.MethodBean("SpringCore_test.ServerInterface", "Consumer")
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("fn method bean selector beanId error", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			// Name is SpringCore_test.ServerInterface
			ctx.CtorBean(NewServerInterface)
			ctx.MethodBean("NULL", "Consumer")
			ctx.AutoWireBeans()
		}, "can't find parent bean: \"NULL\"")
	})
}

func TestApplicationContext_UserDefinedTypeProperty(t *testing.T) {

	type level int

	var config struct {
		Duration time.Duration `value:"${duration}"`
		Level    level         `value:"${level}"`
		Time     time.Time     `value:"${time}"`
		Complex  complex64     // `value:"${complex}"`
	}

	ctx := SpringCore.NewApplicationContext()

	ctx.AddTypeConverter(func(v string) level {
		switch v {
		case "debug":
			return 1
		default:
			panic(errors.New("error level"))
		}
	})

	ctx.SetProperty("time", "2018-12-20")
	ctx.SetProperty("duration", "1h")
	ctx.SetProperty("level", "debug")
	ctx.SetProperty("complex", "1+i")
	ctx.ObjBean(&config)
	ctx.AutoWireBeans()

	fmt.Printf("%+v\n", config)
}

type CircleA struct {
	B *CircleB `autowire:""`
}

type CircleB struct {
	C *CircleC `autowire:""`
}

type CircleC struct {
	A *CircleA `autowire:""`
}

func TestApplicationContext_CircleAutowire(t *testing.T) {
	// 直接创建的 Bean 直接发生循环依赖是没有关系的。
	ctx := SpringCore.NewApplicationContext()
	ctx.ObjBean(new(CircleA))
	ctx.ObjBean(new(CircleB))
	ctx.ObjBean(new(CircleC))
	ctx.AutoWireBeans()
}

type VarInterfaceOptionFunc func(opt *VarInterfaceOption)

type VarInterfaceOption struct {
	v []interface{}
}

func withVarInterface(v ...interface{}) VarInterfaceOptionFunc {
	return func(opt *VarInterfaceOption) {
		opt.v = v
	}
}

type VarInterfaceObj struct {
	v []interface{}
}

func NewVarInterfaceObj(options ...VarInterfaceOptionFunc) *VarInterfaceObj {
	opt := new(VarInterfaceOption)
	for _, option := range options {
		option(opt)
	}
	return &VarInterfaceObj{opt.v}
}

type Var struct {
	name string
}

type VarOption struct {
	v []*Var
}

type VarOptionFunc func(opt *VarOption)

func withVar(v ...*Var) VarOptionFunc {
	return func(opt *VarOption) {
		opt.v = v
	}
}

type VarObj struct {
	v []*Var
	s string
}

func NewVarObj(s string, options ...VarOptionFunc) *VarObj {
	opt := new(VarOption)
	for _, option := range options {
		option(opt)
	}
	return &VarObj{opt.v, s}
}

func TestApplicationContext_RegisterOptionBean(t *testing.T) {

	t.Run("variadic option param 1", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("var.obj", "description")
		ctx.ObjBean(&Var{"v1"}).WithName("v1")
		ctx.ObjBean(&Var{"v2"}).WithName("v2")
		ctx.CtorBean(NewVarObj, "${var.obj}").Options(
			SpringCore.NewOptionArg(withVar, "v1"),
		)
		ctx.AutoWireBeans()

		var obj *VarObj
		ctx.GetBean(&obj)

		SpringUtils.AssertEqual(t, len(obj.v), 1)
		SpringUtils.AssertEqual(t, obj.v[0].name, "v1")
		SpringUtils.AssertEqual(t, obj.s, "description")
	})

	t.Run("variadic option param 2", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("var.obj", "description")
		ctx.ObjBean(&Var{"v1"}).WithName("v1")
		ctx.ObjBean(&Var{"v2"}).WithName("v2")
		ctx.CtorBean(NewVarObj, "${var.obj}").Options(
			SpringCore.NewOptionArg(withVar, "v1", "v2"),
		)
		ctx.AutoWireBeans()

		var obj *VarObj
		ctx.GetBean(&obj)

		SpringUtils.AssertEqual(t, len(obj.v), 2)
		SpringUtils.AssertEqual(t, obj.v[0].name, "v1")
		SpringUtils.AssertEqual(t, obj.v[1].name, "v2")
		SpringUtils.AssertEqual(t, obj.s, "description")
	})

	t.Run("variadic option interface param 1", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&Var{"v1"}).WithName("v1").Export((*interface{})(nil))
		ctx.ObjBean(&Var{"v2"}).WithName("v2").Export((*interface{})(nil))
		ctx.CtorBean(NewVarInterfaceObj).Options(
			SpringCore.NewOptionArg(withVarInterface, "v1"),
		)
		ctx.AutoWireBeans()

		var obj *VarInterfaceObj
		ctx.GetBean(&obj)

		SpringUtils.AssertEqual(t, len(obj.v), 1)
	})

	t.Run("variadic option interface param 1", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&Var{"v1"}).WithName("v1").Export((*interface{})(nil))
		ctx.ObjBean(&Var{"v2"}).WithName("v2").Export((*interface{})(nil))
		ctx.CtorBean(NewVarInterfaceObj).Options(
			SpringCore.NewOptionArg(withVarInterface, "v1", "v2"),
		)
		ctx.AutoWireBeans()

		var obj *VarInterfaceObj
		ctx.GetBean(&obj)

		SpringUtils.AssertEqual(t, len(obj.v), 2)
	})
}

func TestApplicationContext_Close(t *testing.T) {

	t.Run("destroy type", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func() {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func() int { return 0 })
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func(int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func(int, int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")
	})

	t.Run("call destroy fn", func(t *testing.T) {
		called := false

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(new(int)).Destroy(func(i *int) { called = true })
		ctx.AutoWireBeans()
		ctx.Close()

		SpringUtils.AssertEqual(t, called, true)
	})

	t.Run("call destroy method", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).Destroy)
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.destroyed, true)
	})

	t.Run("call destroy method with arg", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).DestroyWithArg, "${version}")
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.destroyed, true)
	})

	t.Run("call destroy method with error", func(t *testing.T) {

		// error
		{
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d *callDestroy
			ok := ctx.GetBean(&d)

			ctx.Close()

			SpringUtils.AssertEqual(t, ok, true)
			SpringUtils.AssertEqual(t, d.destroyed, false)
		}

		// nil
		{
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("int", 0)
			ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d *callDestroy
			ok := ctx.GetBean(&d)

			ctx.Close()

			SpringUtils.AssertEqual(t, ok, true)
			SpringUtils.AssertEqual(t, d.destroyed, true)
		}
	})

	t.Run("call interface destroy method", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.Destroy)
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.(*callDestroy).destroyed, true)
	})

	t.Run("call interface destroy method with arg", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithArg, "${version}")
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, d.(*callDestroy).destroyed, true)
	})

	t.Run("call interface destroy method with error", func(t *testing.T) {

		// error
		{
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d destroyable
			ok := ctx.GetBean(&d)

			ctx.Close()

			SpringUtils.AssertEqual(t, ok, true)
			SpringUtils.AssertEqual(t, d.(*callDestroy).destroyed, false)
		}

		// nil
		{
			ctx := SpringCore.NewApplicationContext()
			ctx.SetProperty("int", 0)
			ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d destroyable
			ok := ctx.GetBean(&d)

			ctx.Close()

			SpringUtils.AssertEqual(t, ok, true)
			SpringUtils.AssertEqual(t, d.(*callDestroy).destroyed, true)
		}
	})

	t.Run("context done", func(t *testing.T) {
		var wg sync.WaitGroup
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(new(int)).Init(func(i *int) {
			wg.Add(1)
			go func() {
				for {
					select {
					case <-ctx.Context().Done():
						wg.Done()
						return
					default:
						time.Sleep(time.Millisecond * 5)
					}
				}
			}()
		})
		ctx.AutoWireBeans()
		ctx.Close()
		wg.Wait()
	})
}

func TestApplicationContext_BeanNotFound(t *testing.T) {
	SpringUtils.AssertPanic(t, func() {
		ctx := SpringCore.NewApplicationContext()
		ctx.CtorBean(func(i *int) bool { return false })
		ctx.AutoWireBeans()
	}, "can't find bean, bean: \"\" field:  type: \\*int")
}

type SubNestedAutowireBean struct {
	Int *int `autowire:""`
}

type NestedAutowireBean struct {
	SubNestedAutowireBean
	_ *float32
	_ bool
}

type PtrNestedAutowireBean struct {
	*SubNestedAutowireBean // 不处理
	_                      *float32
	_                      bool
}

type FieldNestedAutowireBean struct {
	B SubNestedAutowireBean
	_ *float32
	_ bool
}

type PtrFieldNestedAutowireBean struct {
	B *SubNestedAutowireBean // 不处理
	_ *float32
	_ bool
}

func TestApplicationContext_NestedAutowireBean(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.CtorBean(func() int { return 3 })
	ctx.ObjBean(new(NestedAutowireBean))
	ctx.ObjBean(&PtrNestedAutowireBean{
		SubNestedAutowireBean: new(SubNestedAutowireBean),
	})
	ctx.ObjBean(new(FieldNestedAutowireBean))
	ctx.ObjBean(&PtrFieldNestedAutowireBean{
		B: new(SubNestedAutowireBean),
	})
	ctx.AutoWireBeans()

	var b *NestedAutowireBean
	ok := ctx.GetBean(&b)

	SpringUtils.AssertEqual(t, ok, true)
	SpringUtils.AssertEqual(t, *b.Int, 3)

	var b0 *PtrNestedAutowireBean
	ok = ctx.GetBean(&b0)

	SpringUtils.AssertEqual(t, ok, true)
	SpringUtils.AssertEqual(t, b0.Int, (*int)(nil))

	var b1 *FieldNestedAutowireBean
	ok = ctx.GetBean(&b1)

	SpringUtils.AssertEqual(t, ok, true)
	SpringUtils.AssertEqual(t, *b1.B.Int, 3)

	var b2 *PtrFieldNestedAutowireBean
	ok = ctx.GetBean(&b2)

	SpringUtils.AssertEqual(t, ok, true)
	SpringUtils.AssertEqual(t, b2.B.Int, (*int)(nil))
}

type BaseChannel struct {
	Int        *int `autowire:""`
	AutoCreate bool `value:"${auto-create}"`
	Enable     bool `value:"${enable:=false}"`
}

type WXChannel struct {
	BaseChannel `value:"${sdk.wx}"`
	Int         *int `autowire:""`
}

type baseChannel struct {
	Int        *int `autowire:""`
	AutoCreate bool `value:"${auto-create}"`

	// 支持对私有字段注入，但是不推荐！代码扫描请忽略这行。
	enable bool `value:"${enable:=false}"`
}

type wxChannel struct {
	baseChannel `value:"${sdk.wx}"`

	// 支持对私有字段注入，但是不推荐！代码扫描请忽略这行。
	int *int `autowire:""`
}

func TestApplicationContext_NestValueField(t *testing.T) {

	t.Run("private", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("sdk.wx.auto-create", true)
		ctx.SetProperty("sdk.wx.enable", true)

		bd := ctx.CtorBean(func() int { return 3 })
		SpringUtils.AssertEqual(t, bd.Name(), "*int")

		ctx.ObjBean(new(wxChannel))
		ctx.AutoWireBeans()

		var c *wxChannel
		ok := ctx.GetBean(&c)

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, *c.baseChannel.Int, 3)
		SpringUtils.AssertEqual(t, *c.int, 3)
		SpringUtils.AssertEqual(t, c.baseChannel.Int, c.int)
		SpringUtils.AssertEqual(t, c.enable, true)
		SpringUtils.AssertEqual(t, c.AutoCreate, true)
	})

	t.Run("public", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("sdk.wx.auto-create", true)
		ctx.SetProperty("sdk.wx.enable", true)
		ctx.CtorBean(func() int { return 3 })
		ctx.ObjBean(new(WXChannel))
		ctx.AutoWireBeans()

		var c *WXChannel
		ok := ctx.GetBean(&c)

		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, *c.BaseChannel.Int, 3)
		SpringUtils.AssertEqual(t, *c.Int, 3)
		SpringUtils.AssertEqual(t, c.BaseChannel.Int, c.Int)
		SpringUtils.AssertEqual(t, c.Enable, true)
		SpringUtils.AssertEqual(t, c.AutoCreate, true)
	})
}

func TestApplicationContext_FnArgCollectBean(t *testing.T) {

	t.Run("base type", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.CtorBean(func() int { return 3 }).WithName("i1")
		ctx.CtorBean(func() int { return 4 }).WithName("i2")
		ctx.CtorBean(func(i []*int) bool {
			nums := make([]int, 0)
			for _, e := range i {
				nums = append(nums, *e)
			}
			sort.Ints(nums)
			SpringUtils.AssertEqual(t, nums, []int{3, 4})
			return false
		}, "[]")
		ctx.AutoWireBeans()
	})

	t.Run("interface type", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(newHistoryTeacher("t1")).WithName("t1").Export((*Teacher)(nil))
		ctx.ObjBean(newHistoryTeacher("t2")).WithName("t2").Export((*Teacher)(nil))
		ctx.CtorBean(func(teachers []Teacher) bool {
			names := make([]string, 0)
			for _, teacher := range teachers {
				names = append(names, teacher.(*historyTeacher).name)
			}
			sort.Strings(names)
			SpringUtils.AssertEqual(t, names, []string{"t1", "t2"})
			return false
		}, "[]")
		ctx.AutoWireBeans()
	})
}

type filter interface {
	Filter(input string) string
}

type filterImpl struct {
}

func (_ *filterImpl) Filter(input string) string {
	return input
}

func TestApplicationContext_BeanCache(t *testing.T) {

	t.Run("not implement interface", func(t *testing.T) {
		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(new(int)).Export((*filter)(nil))
			ctx.AutoWireBeans()
		}, "not implement SpringCore_test.filter interface")
	})

	t.Run("implement interface", func(t *testing.T) {

		var server struct {
			F1 filter `autowire:"f1"`
			F2 filter `autowire:"f2"`
		}

		ctx := SpringCore.NewApplicationContext()
		ctx.CtorBean(func() filter { return new(filterImpl) }).WithName("f1")
		ctx.ObjBean(new(filterImpl)).Export((*filter)(nil)).WithName("f2")
		ctx.ObjBean(&server)

		ctx.AutoWireBeans()
	})
}

type IntInterface interface {
	Value() int
}

type Integer int

func (i Integer) Value() int {
	return int(i)
}

func TestApplicationContext_IntInterface(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.CtorBean(func() IntInterface { return Integer(5) })
	ctx.AutoWireBeans()
}

type ptrBaseInterface interface {
	PtrBase()
}

type ptrBaseContext struct {
	_ ptrBaseInterface `export:""`
}

func (_ *ptrBaseContext) PtrBase() {

}

type baseInterface interface {
	Base()
}

type baseContext struct {
	_ baseInterface `export:""`
}

func (_ *baseContext) Base() {

}

type AppContext struct {
	// 这种导出方式建议写在最上面
	_ fmt.Stringer `export:""`

	context.Context `export:""`

	*ptrBaseContext
	baseContext
}

func (_ *AppContext) String() string {
	return ""
}

func (_ *AppContext) Error() string {
	return ""
}

func TestApplicationContext_AutoExport(t *testing.T) {

	t.Run("auto export", func(t *testing.T) {

		b := &AppContext{
			Context:        context.TODO(),
			ptrBaseContext: &ptrBaseContext{},
		}

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(b)
		ctx.AutoWireBeans()

		var x context.Context
		ok := ctx.GetBean(&x)
		SpringUtils.AssertEqual(t, true, ok)
		SpringUtils.AssertEqual(t, b, x)

		var s fmt.Stringer
		ok = ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, true, ok)
		SpringUtils.AssertEqual(t, b, s)

		var pbi ptrBaseInterface
		ok = ctx.GetBean(&pbi)
		SpringUtils.AssertEqual(t, true, ok)
		SpringUtils.AssertEqual(t, b, pbi)

		var bi baseInterface
		ok = ctx.GetBean(&bi)
		SpringUtils.AssertEqual(t, true, ok)
		SpringUtils.AssertEqual(t, b, bi)
	})

	t.Run("auto export private", func(t *testing.T) {

		ctx := SpringCore.NewApplicationContext()
		ctx.CtorBean(pkg2.NewAppContext)
		ctx.AutoWireBeans()

		var x context.Context
		ok := ctx.GetBean(&x)
		SpringUtils.AssertEqual(t, true, ok)

		var s fmt.Stringer
		ok = ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, true, ok)
	})

	t.Run("auto export & export", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}

		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(b).Export((*fmt.Stringer)(nil))
		ctx.AutoWireBeans()

		var x context.Context
		ok := ctx.GetBean(&x)
		SpringUtils.AssertEqual(t, true, ok)
		SpringUtils.AssertEqual(t, b, x)

		var s fmt.Stringer
		ok = ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, true, ok)
		SpringUtils.AssertEqual(t, b, s)
	})

	t.Run("unexported but auto match", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&struct {
			Error error `autowire:"e"`
		}{})
		ctx.ObjBean(b).WithName("e")
		ctx.AutoWireBeans()
	})

	t.Run("export and match directly", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(&struct {
			Error error `autowire:"e"`
		}{})
		ctx.ObjBean(b).WithName("e").Export((*error)(nil))
		ctx.AutoWireBeans()
	})

	t.Run("panics", func(t *testing.T) {

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(&struct {
				_ *int `export:""`
			}{})
			ctx.AutoWireBeans()
		}, "export can only use on interface")

		SpringUtils.AssertPanic(t, func() {
			ctx := SpringCore.NewApplicationContext()
			ctx.ObjBean(&struct {
				_ Runner `export:"" autowire:""`
			}{})
			ctx.AutoWireBeans()
		}, "inject or autowire can't use with export")
	})
}

type ArrayProperties struct {
	Int      []int           `value:"${int.array:=1,2,3}"`
	Int8     []int8          `value:"${int8.array:=1,2,3}"`
	Int16    []int16         `value:"${int16.array:=1,2,3}"`
	Int32    []int32         `value:"${int32.array:=1,2,3}"`
	Int64    []int64         `value:"${int64.array:=1,2,3}"`
	UInt     []uint          `value:"${uint.array:=1,2,3}"`
	UInt8    []uint8         `value:"${uint8.array:=1,2,3}"`
	UInt16   []uint16        `value:"${uint16.array:=1,2,3}"`
	UInt32   []uint32        `value:"${uint32.array:=1,2,3}"`
	UInt64   []uint64        `value:"${uint64.array:=1,2,3}"`
	String   []string        `value:"${string.array:=s1,s2,s3}"`
	Bool     []bool          `value:"${bool.array:=0,1,false,true}"`
	Duration []time.Duration `value:"${duration.array:=1000ms,5s}"`
	Time     []time.Time     `value:"${time.array:=2006-01-02T15:04:05Z,01 Jan 2020,2020-01-01 00:00:00}"`
}

func TestApplicationContext_Properties(t *testing.T) {

	t.Run("array properties", func(t *testing.T) {
		ctx := SpringCore.NewApplicationContext()
		bd := ctx.ObjBean(new(ArrayProperties))
		ctx.AutoWireBeans()
		p := bd.Bean().(*ArrayProperties)
		SpringUtils.AssertEqual(t, p.Duration, []time.Duration{time.Second, 5 * time.Second})
	})

	t.Run("map default value ", func(t *testing.T) {

		obj := struct {
			Int  int               `value:"${int:=5}"`
			IntA int               `value:"${int.a:=5}"`
			Map  map[string]string `value:"${map:={}}"`
			MapA map[string]string `value:"${map_a:={}}"`
		}{}

		ctx := SpringCore.NewApplicationContext()
		ctx.SetProperty("map_a.nba", "nba")
		ctx.SetProperty("map_a.cba", "cba")
		ctx.SetProperty("int.a", "3")
		ctx.SetProperty("int.b", "4")
		ctx.ObjBean(&obj)
		ctx.AutoWireBeans()

		SpringUtils.AssertEqual(t, obj.Int, 5)
		SpringUtils.AssertEqual(t, obj.IntA, 3)
		SpringUtils.AssertEqual(t, obj.Map, map[string]string{})
		SpringUtils.AssertEqual(t, obj.MapA, map[string]string{
			"cba": "cba", "nba": "nba",
		})
	})
}

func TestFnStringBindingArg(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()
	ctx.CtorBean(func(i *int) bool {
		fmt.Printf("i=%d\n", *i)
		return false
	}, "${key.name:=*int}")
	i := 5
	ctx.ObjBean(&i)
	ctx.AutoWireBeans()
}

type FirstDestroy struct {
	T1 *Second1Destroy `autowire:""`
	T2 *Second2Destroy `autowire:""`
}

type Second1Destroy struct {
	T *ThirdDestroy `autowire:""`
}

type Second2Destroy struct {
	T *ThirdDestroy `autowire:""`
}

type ThirdDestroy struct {
}

func TestApplicationContext_Destroy(t *testing.T) {

	destroyIndex := 0
	destroyArray := []int{0, 0, 0, 0}

	ctx := SpringCore.NewApplicationContext()
	ctx.ObjBean(new(FirstDestroy)).Destroy(
		func(_ *FirstDestroy) {
			fmt.Println("::FirstDestroy")
			destroyArray[destroyIndex] = 1
			destroyIndex++
		})
	ctx.ObjBean(new(ThirdDestroy)).Destroy(
		func(_ *ThirdDestroy) {
			fmt.Println("::ThirdDestroy")
			destroyArray[destroyIndex] = 4
			destroyIndex++
		})
	ctx.ObjBean(new(Second2Destroy)).Destroy(
		func(_ *Second2Destroy) {
			fmt.Println("::Second2Destroy")
			destroyArray[destroyIndex] = 2
			destroyIndex++
		})
	ctx.ObjBean(new(Second1Destroy)).Destroy(
		func(_ *Second1Destroy) {
			fmt.Println("::Second1Destroy")
			destroyArray[destroyIndex] = 2
			destroyIndex++
		})
	ctx.AutoWireBeans()
	ctx.Close()

	SpringUtils.AssertEqual(t, destroyArray, []int{1, 2, 2, 4})
}

type Registry interface {
	got()
}

type registry struct{}

func (r *registry) got() {}

func NewRegistry() *registry {
	return &registry{}
}

func NewRegistryInterface() Registry {
	return &registry{}
}

var DefaultRegistry Registry = NewRegistry()

type registryFactory struct{}

func (f *registryFactory) Create() Registry { return NewRegistry() }

func TestApplicationContext_NameEquivalence(t *testing.T) {

	SpringUtils.AssertPanic(t, func() {
		ctx := SpringCore.NewApplicationContext()
		ctx.ObjBean(DefaultRegistry)
		ctx.CtorBean(NewRegistry)
		ctx.AutoWireBeans()
	}, `duplicate registration, bean: `)

	SpringUtils.AssertPanic(t, func() {
		ctx := SpringCore.NewApplicationContext()
		bd := ctx.ObjBean(&registryFactory{})
		ctx.MethodBean(bd, "Create")
		ctx.CtorBean(NewRegistryInterface)
		ctx.AutoWireBeans()
	}, `duplicate registration, bean: `)
}
