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
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-spring/go-spring-parent/spring-utils"
	"github.com/go-spring/go-spring/spring-core"
	pkg1 "github.com/go-spring/go-spring/spring-core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/go-spring/spring-core/testdata/pkg/foo"
	"github.com/magiconair/properties/assert"
)

func TestDefaultSpringContext_RegisterBeanFrozen(t *testing.T) {
	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(int)).Init(func(i *int) {
			// 不能在这里注册新的 Bean
			ctx.RegisterBean(new(bool))
		})
		ctx.AutoWireBeans()
	}, "bean registration frozen")
}

func TestDefaultSpringContext(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()

		e := int(3)
		a := []int{3}

		// 普通类型用属性注入
		assert.Panic(t, func() {
			ctx.RegisterBean(e)
		}, "bean must be ref type")

		ctx.RegisterBean(&e)

		// 相同类型的匿名 bean 不能重复注册
		assert.Panic(t, func() {
			ctx.RegisterBean(&e)
		}, "duplicate bean registration int:\\*int")

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i3", &e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i4", &e)

		ctx.RegisterBean(a)
		ctx.RegisterBean(&a)

		ctx.AutoWireBeans()

		assert.Panic(t, func() {
			var i int
			ctx.GetBean(&i)
		}, "receiver \"\" must be ref type")

		// 找到多个符合条件的值
		assert.Panic(t, func() {
			var i *int
			ctx.GetBean(&i)
		}, "找到多个符合条件的值")

		// 入参不是可赋值的对象
		assert.Panic(t, func() {
			var i int
			ctx.GetBeanByName("i3", &i)
			fmt.Println(i)
		}, "receiver \"\" must be ref type")

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
	})

	/////////////////////////////////////////
	// 自定义数据类型

	t.Run("pkg1.SamePkg", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()

		e := pkg1.SamePkg{}
		a := []pkg1.SamePkg{{}}
		p := []*pkg1.SamePkg{{}}

		// 栈上的对象不能注册
		assert.Panic(t, func() {
			ctx.RegisterBean(e)
		}, "bean must be ref type")

		ctx.RegisterBean(&e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i3", &e)

		// 相同类型不同名称的 bean 都可注册
		ctx.RegisterNameBean("i4", &e)

		ctx.RegisterBean(a)
		ctx.RegisterBean(&a)
		ctx.RegisterBean(p)
		ctx.RegisterBean(&p)

		ctx.AutoWireBeans()
	})

	t.Run("pkg2.SamePkg", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()

		e := pkg2.SamePkg{}
		a := []pkg2.SamePkg{{}}
		p := []*pkg2.SamePkg{{}}

		// 栈上的对象不能注册
		assert.Panic(t, func() {
			ctx.RegisterBean(e)
		}, "bean must be ref type")

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
	StructByType *TestBinding `autowire:""`
	StructByName *TestBinding `autowire:"struct_ptr"`

	// 自定义类型数组
	StructSliceByType []TestBinding `autowire:""`
	StructCollection  []TestBinding `autowire:"[]"`
	StructSliceByName []TestBinding `autowire:"struct_slice"`

	// 自定义类型指针数组
	StructPtrSliceByType []*TestBinding `autowire:""`
	StructPtrCollection  []*TestBinding `autowire:"[]"`
	StructPtrSliceByName []*TestBinding `autowire:"struct_ptr_slice"`

	// 接口
	InterfaceByType fmt.Stringer `autowire:""`
	InterfaceByName fmt.Stringer `autowire:"struct_ptr"`

	// 接口数组
	InterfaceSliceByType []fmt.Stringer `autowire:""`

	InterfaceCollection  []fmt.Stringer `autowire:"[]"`
	InterfaceCollection2 []fmt.Stringer `autowire:"[]"`

	// 指定名称时使用精确匹配模式，不对数组元素进行转换，即便能做到似乎也无意义
	InterfaceSliceByName []fmt.Stringer `autowire:"struct_ptr_slice?"`

	MapTyType map[string]interface{} `autowire:""`
	MapByName map[string]interface{} `autowire:"map"`
}

func TestDefaultSpringContext_AutoWireBeans(t *testing.T) {

	t.Run("wired error", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()

		obj := &TestObject{}
		ctx.RegisterBean(obj)

		i := int(3)
		ctx.RegisterNameBean("int_ptr", &i)

		i2 := int(3)
		ctx.RegisterNameBean("int_ptr_2", &i2)

		assert.Panic(t, func() {
			ctx.AutoWireBeans()
		}, "TestObject.\\$IntPtrByType 找到多个符合条件的值")
	})

	ctx := SpringCore.NewDefaultSpringContext()

	obj := &TestObject{}
	ctx.RegisterBean(obj)

	i := int(3)
	ctx.RegisterNameBean("int_ptr", &i)

	is := []int{1, 2, 3}
	ctx.RegisterNameBean("int_slice", is)

	is2 := []int{2, 3, 4}
	ctx.RegisterNameBean("int_slice_2", is2)

	i2 := 4
	ips := []*int{&i2}
	ctx.RegisterNameBean("int_ptr_slice", ips)

	b := TestBinding{1}
	ctx.RegisterNameBean("struct_ptr", &b)

	bs := []TestBinding{{10}}
	ctx.RegisterNameBean("struct_slice", bs)

	b2 := TestBinding{2}
	bps := []*TestBinding{&b2}
	ctx.RegisterNameBean("struct_ptr_slice", bps)

	s := []fmt.Stringer{&TestBinding{3}}
	ctx.RegisterBean(s)

	m := map[string]interface{}{
		"5": 5,
	}

	ctx.RegisterNameBean("map", m)

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

	IntSlice    []int    `value:"${int_slice}"`
	StringSlice []string `value:"${string_slice}"`
	//FloatSlice  []float64 `value:"${float_slice}"`
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

	ctx.RegisterBean(setting).ConditionOn(
		SpringCore.NewConditional().OnProperty("bool").And().OnCondition(
			SpringCore.NewConditional().OnPropertyValue("int", "3"),
		),
	)

	ctx.SetProperty("sub.int", int(4))
	ctx.SetProperty("sub.sub.int", int(5))
	ctx.SetProperty("sub_sub.int", int(6))

	ctx.SetProperty("int_slice", []int{1, 2})
	ctx.SetProperty("string_slice", []string{"1", "2"})
	//ctx.SetProperty("float_slice", []float64{1, 2})

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

func TestDefaultSpringContext_TypeConverter(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()
	ctx.LoadProperties("testdata/config/application.yaml")

	b := &EnvEnumBean{}
	ctx.RegisterBean(b)

	ctx.SetProperty("env.type", "test")

	p := &PointBean{}
	ctx.RegisterBean(p)

	SpringCore.RegisterTypeConverter(PointConverter)

	ctx.SetProperty("point", "(7,5)")

	dbConfig := &DbConfig{}
	ctx.RegisterBean(dbConfig)

	ctx.AutoWireBeans()

	assert.Equal(t, b.EnvType, ENV_TEST)

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
	ctx.LoadProperties("testdata/config/application.yaml")
	ctx.LoadProperties("testdata/config/application.properties")

	val0 := ctx.GetStringProperty("spring.application.name")
	assert.Equal(t, val0, "test")

	val1 := ctx.GetProperty("yaml.list")
	assert.Equal(t, val1, []interface{}{1, 2})
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

type BeanThree struct {
	One *BeanTwo `autowire:""`
}

func TestDefaultSpringContext_GetBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))
	ctx.RegisterBean(new(BeanTwo))

	ctx.AutoWireBeans()

	var two *BeanTwo
	ok := ctx.GetBean(&two)
	assert.Equal(t, ok, true)

	var three *BeanThree
	ok = ctx.GetBean(&three)
	assert.Equal(t, ok, false)

	fmt.Println(SpringUtils.ToJson(two))
}

func TestDefaultSpringContext_GetBeanByName(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))
	ctx.RegisterBean(new(BeanTwo))

	ctx.AutoWireBeans()

	var two *BeanTwo
	ok := ctx.GetBeanByName("", &two)
	assert.Equal(t, ok, true)

	ok = ctx.GetBeanByName("*SpringCore_test.BeanTwo", &two)
	assert.Equal(t, ok, true)

	ok = ctx.GetBeanByName("BeanTwo", &two)
	assert.Equal(t, ok, false)

	ok = ctx.GetBeanByName(":*SpringCore_test.BeanTwo", &two)
	assert.Equal(t, ok, true)

	ok = ctx.GetBeanByName("github.com/go-spring/go-spring/spring-core_test/SpringCore_test.BeanTwo:*SpringCore_test.BeanTwo", &two)
	assert.Equal(t, ok, true)

	ok = ctx.GetBeanByName("xxx:*SpringCore_test.BeanTwo", &two)
	assert.Equal(t, ok, false)

	var three *BeanThree
	ok = ctx.GetBeanByName("", &three)
	assert.Equal(t, ok, false)

	fmt.Println(SpringUtils.ToJson(two))
}

func TestDefaultSpringContext_FindBeanByName(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))
	ctx.RegisterBean(new(BeanTwo))

	ctx.AutoWireBeans()

	assert.Panic(t, func() {
		ctx.FindBeanByName("")
	}, "找到多个符合条件的值")

	i, ok := ctx.FindBeanByName("*SpringCore_test.BeanTwo")
	fmt.Println(SpringUtils.ToJson(i))
	assert.Equal(t, ok, true)

	i, ok = ctx.FindBeanByName("BeanTwo")
	fmt.Println(SpringUtils.ToJson(i))
	assert.Equal(t, ok, false)

	i, ok = ctx.FindBeanByName(":*SpringCore_test.BeanTwo")
	fmt.Println(SpringUtils.ToJson(i))
	assert.Equal(t, ok, true)

	i, ok = ctx.FindBeanByName("github.com/go-spring/go-spring/spring-core_test/SpringCore_test.BeanTwo:*SpringCore_test.BeanTwo")
	fmt.Println(SpringUtils.ToJson(i))
	assert.Equal(t, ok, true)

	i, ok = ctx.FindBeanByName("xxx:*SpringCore_test.BeanTwo")
	fmt.Println(SpringUtils.ToJson(i))
	assert.Equal(t, ok, false)
}

func TestDefaultSpringContext_RegisterBeanFn(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("room", "Class 3 Grade 1")

	// 用接口注册时实际使用的是原始类型
	ctx.RegisterBean(Teacher(&historyTeacher{}))

	ctx.RegisterNameBeanFn("st1", NewStudent, "", "${room}")
	ctx.RegisterNameBeanFn("st2", NewPtrStudent, "1:${room}")
	ctx.RegisterNameBeanFn("st3", NewStudent, "?", "${room:=http://}")
	ctx.RegisterNameBeanFn("st4", NewPtrStudent, "0:?", "1:${room:=4567}")

	mapFn := func() map[int]string {
		return map[int]string{
			1: "ok",
		}
	}

	ctx.RegisterBeanFn(mapFn)

	sliceFn := func() []int {
		return []int{1, 2}
	}

	ctx.RegisterBeanFn(sliceFn)

	ctx.AutoWireBeans()

	var st1 *Student
	ok := ctx.GetBeanByName("st1", &st1)

	assert.Equal(t, ok, true)
	fmt.Println(SpringUtils.ToJson(st1))
	assert.Equal(t, st1.Room, ctx.GetStringProperty("room"))

	var st2 *Student
	ok = ctx.GetBeanByName("st2", &st2)

	assert.Equal(t, ok, true)
	fmt.Println(SpringUtils.ToJson(st2))
	assert.Equal(t, st2.Room, ctx.GetStringProperty("room"))

	fmt.Printf("%x\n", reflect.ValueOf(st1).Pointer())
	fmt.Printf("%x\n", reflect.ValueOf(st2).Pointer())

	var st3 *Student
	ok = ctx.GetBeanByName("st3", &st3)

	assert.Equal(t, ok, true)
	fmt.Println(SpringUtils.ToJson(st3))
	assert.Equal(t, st3.Room, ctx.GetStringProperty("room"))

	var st4 *Student
	ok = ctx.GetBeanByName("st4", &st4)

	assert.Equal(t, ok, true)
	fmt.Println(SpringUtils.ToJson(st4))
	assert.Equal(t, st4.Room, ctx.GetStringProperty("room"))

	var m map[int]string
	ok = ctx.GetBean(&m)

	assert.Equal(t, ok, true)
	fmt.Println(SpringUtils.ToJson(m))
	assert.Equal(t, m[1], "ok")

	var s []int
	ok = ctx.GetBean(&s)

	assert.Equal(t, ok, true)
	fmt.Println(SpringUtils.ToJson(s))
	assert.Equal(t, s[1], 2)
}

func TestDefaultSpringContext_Profile(t *testing.T) {

	t.Run("bean:_ctx:", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5})
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, true)
	})

	t.Run("bean:_ctx:test", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProfile("test")
		ctx.RegisterBean(&BeanZero{5})
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, true)
	})

	t.Run("bean:test_ctx:", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5}).
			ConditionOnProfile("test").
			And().
			ConditionOnMissingBean("null")
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, false)
	})

	t.Run("bean:test_ctx:test", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProfile("test")
		ctx.RegisterBean(&BeanZero{5}).ConditionOnProfile("test")
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, true)
	})

	t.Run("bean:test_ctx:stable", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProfile("stable")
		ctx.RegisterBean(&BeanZero{5}).ConditionOnProfile("test")
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, false)
	})
}

type BeanFour struct{}

func TestDefaultSpringContext_DependsOn(t *testing.T) {

	t.Run("random", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5})
		ctx.RegisterBean(new(BeanOne))
		ctx.RegisterBean(new(BeanFour))
		ctx.AutoWireBeans()
	})

	t.Run("dependsOn", func(t *testing.T) {

		dependsOn := []string{
			"github.com/go-spring/go-spring/spring-core_test/SpringCore_test.BeanZero:*SpringCore_test.BeanZero",
			"github.com/go-spring/go-spring/spring-core_test/SpringCore_test.BeanOne:*SpringCore_test.BeanOne",
		}

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5})
		ctx.RegisterBean(new(BeanOne))
		ctx.RegisterBean(new(BeanFour)).DependsOn(dependsOn...)
		ctx.AutoWireBeans()
	})
}

func TestDefaultSpringContext_Primary(t *testing.T) {

	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5})
		ctx.RegisterBean(&BeanZero{6})
		ctx.RegisterBean(new(BeanOne))
		ctx.RegisterBean(new(BeanTwo))
		ctx.AutoWireBeans()
	}, "duplicate bean registration github.com/go-spring/go-spring/spring-core_test/SpringCore_test.BeanZero:\\*SpringCore_test.BeanZero")

	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5})
		// Primary 是在多个候选 bean 里面选择，而不是允许同名同类型的两个 bean
		ctx.RegisterBean(&BeanZero{6}).Primary(true)
		ctx.RegisterBean(new(BeanOne))
		ctx.RegisterBean(new(BeanTwo))
		ctx.AutoWireBeans()
	}, "duplicate bean registration github.com/go-spring/go-spring/spring-core_test/SpringCore_test.BeanZero:\\*SpringCore_test.BeanZero")

	t.Run("not primary", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5})
		ctx.RegisterBean(new(BeanOne))
		ctx.RegisterBean(new(BeanTwo))
		ctx.AutoWireBeans()

		var b *BeanTwo
		ctx.GetBean(&b)
		assert.Equal(t, b.One.Zero.Int, 5)
	})

	t.Run("primary", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5})
		ctx.RegisterNameBean("zero_6", &BeanZero{6}).Primary(true)
		ctx.RegisterBean(new(BeanOne))
		ctx.RegisterBean(new(BeanTwo))
		ctx.AutoWireBeans()

		var b *BeanTwo
		ctx.GetBean(&b)
		assert.Equal(t, b.One.Zero.Int, 6)
	})
}

type FuncObj struct {
	Fn func(int) int `autowire:""`
}

func TestDefaultProperties_WireFunc(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()
	ctx.RegisterBean(func(int) int {
		return 6
	})
	obj := new(FuncObj)
	ctx.RegisterBean(obj)
	ctx.AutoWireBeans()
	i := obj.Fn(3)
	assert.Equal(t, i, 6)
}

func TestDefaultSpringContext_ConditionOnBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	c := SpringCore.NewConditional().
		OnMissingProperty("Null").
		Or().
		OnProfile("test")

	ctx.RegisterBean(&BeanZero{5}).
		ConditionOn(c).
		And().
		ConditionOnMissingBean("null")

	ctx.RegisterBean(new(BeanOne)).
		ConditionOn(c).
		And().
		ConditionOnMissingBean("null")

	ctx.RegisterBean(new(BeanTwo)).ConditionOnBean("*SpringCore_test.BeanOne")
	ctx.RegisterNameBean("another_two", new(BeanTwo)).ConditionOnBean("Null")

	ctx.AutoWireBeans()

	var two *BeanTwo
	ok := ctx.GetBeanByName("", &two)
	assert.Equal(t, ok, true)

	ok = ctx.GetBeanByName("another_two", &two)
	assert.Equal(t, ok, false)
}

func TestDefaultSpringContext_ConditionOnMissingBean(t *testing.T) {

	for i := 0; i < 50; i++ { // 测试 FindBeanByName 无需绑定
		ctx := SpringCore.NewDefaultSpringContext()

		ctx.RegisterBean(&BeanZero{5})
		ctx.RegisterBean(new(BeanOne))

		ctx.RegisterBean(new(BeanTwo)).ConditionOnMissingBean("*SpringCore_test.BeanOne")
		ctx.RegisterNameBean("another_two", new(BeanTwo)).ConditionOnMissingBean("Null")

		ctx.AutoWireBeans()

		var two *BeanTwo
		ok := ctx.GetBeanByName("", &two)
		assert.Equal(t, ok, true)

		ok = ctx.GetBeanByName("another_two", &two)
		assert.Equal(t, ok, true)
	}
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

func TestDefaultSpringContext_RegisterBeanFn2(t *testing.T) {

	t.Run("ptr manager", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.RegisterBeanFn(NewPtrManager)
		ctx.RegisterBeanFn(NewInt)
		ctx.AutoWireBeans()

		var m Manager
		ok := ctx.GetBean(&m)
		assert.Equal(t, ok, true)

		// 因为用户是按照接口注册的，所以理论上在依赖
		// 系统中用户并不关心接口对应的真实类型是什么。
		var lm *localManager
		ok = ctx.GetBean(&lm)
		assert.Equal(t, ok, false)
	})

	t.Run("manager", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.RegisterBeanFn(NewManager)
		ctx.RegisterBeanFn(NewInt)
		ctx.AutoWireBeans()

		var m Manager
		ok := ctx.GetBean(&m)
		assert.Equal(t, ok, true)

		var lm *localManager
		ok = ctx.GetBean(&lm)
		assert.Equal(t, ok, false)
	})

	t.Run("manager return error", func(t *testing.T) {
		assert.Panic(t, func() {
			ctx := SpringCore.NewDefaultSpringContext()
			ctx.SetProperty("manager.version", "1.0.0")
			ctx.RegisterBeanFn(NewManagerRetError)
			ctx.AutoWireBeans()
		}, "return error")
	})

	t.Run("manager return error nil", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.RegisterBeanFn(NewManagerRetErrorNil)
		ctx.AutoWireBeans()
	})

	t.Run("manager return nil", func(t *testing.T) {
		assert.Panic(t, func() {
			ctx := SpringCore.NewDefaultSpringContext()
			ctx.SetProperty("manager.version", "1.0.0")
			ctx.RegisterBeanFn(NewNullPtrManager)
			ctx.AutoWireBeans()
		}, "return nil")
	})
}

func TestRegisterBean_InitFunc(t *testing.T) {

	t.Run("int", func(t *testing.T) {

		assert.Panic(t, func() {
			ctx := SpringCore.NewDefaultSpringContext()
			ctx.RegisterBean(new(int)).Init(func() {})
		}, "init should be func\\(bean\\)")

		assert.Panic(t, func() {
			ctx := SpringCore.NewDefaultSpringContext()
			ctx.RegisterBean(new(int)).Init(func() int { return 0 })
		}, "init should be func\\(bean\\)")

		assert.Panic(t, func() {
			ctx := SpringCore.NewDefaultSpringContext()
			ctx.RegisterBean(new(int)).Init(func(int) {})
		}, "init should be func\\(bean\\)")

		assert.Panic(t, func() {
			ctx := SpringCore.NewDefaultSpringContext()
			ctx.RegisterBean(new(int)).Init(func(int, int) {})
		}, "init should be func\\(bean\\)")

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(int)).Init(func(i *int) { *i = 3 })
		ctx.AutoWireBeans()

		var i *int
		ctx.GetBean(&i)
		assert.Equal(t, *i, 3)
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

func TestDefaultSpringContext_ValueBinding(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("redis.endpoints", "redis://127.0.0.1:6379")
	ctx.RegisterBean(new(RedisCluster))
	ctx.AutoWireBeans()

	var cluster *RedisCluster
	ctx.GetBean(&cluster)
	fmt.Println(cluster)

	assert.Equal(t, cluster.Endpoints, cluster.RedisConfig.Endpoints)
	assert.Equal(t, cluster.Endpoints, cluster.Nested.RedisConfig.Endpoints)
}

func TestDefaultSpringContext_CollectBeans(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("redis.endpoints", "redis://127.0.0.1:6379")

	ctx.RegisterBean([]*RedisCluster{new(RedisCluster)})
	ctx.RegisterBean([]RedisCluster{{}})
	ctx.RegisterBean(new(RedisCluster))

	ctx.RegisterBean(new(int)).Init(func(*int) {

		var rcs []*RedisCluster
		ctx.CollectBeans(&rcs)
		fmt.Println(SpringUtils.ToJson(rcs))

		assert.Equal(t, len(rcs), 2)
		assert.Equal(t, rcs[0].Endpoints, "redis://127.0.0.1:6379")
	})

	ctx.RegisterBean(new(bool)).Init(func(*bool) {

		var rcs []RedisCluster
		ctx.CollectBeans(&rcs)
		fmt.Println(SpringUtils.ToJson(rcs))

		assert.Equal(t, len(rcs), 1)
		assert.Equal(t, rcs[0].Endpoints, "redis://127.0.0.1:6379")
	})

	ctx.AutoWireBeans()

	var rcs []RedisCluster
	ctx.GetBean(&rcs)
	fmt.Println(SpringUtils.ToJson(rcs))
}

func TestDefaultSpringContext_WireSliceBean(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("redis.endpoints", "redis://127.0.0.1:6379")
	ctx.RegisterBean([]*RedisCluster{new(RedisCluster)})
	ctx.RegisterBean([]RedisCluster{{}})
	ctx.AutoWireBeans()

	{
		var rcs []*RedisCluster
		ctx.GetBean(&rcs)
		fmt.Println(SpringUtils.ToJson(rcs))

		assert.Equal(t, rcs[0].Endpoints, "redis://127.0.0.1:6379")
	}

	{
		var rcs []RedisCluster
		ctx.GetBean(&rcs)
		fmt.Println(SpringUtils.ToJson(rcs))

		assert.Equal(t, rcs[0].Endpoints, "redis://127.0.0.1:6379")
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
	assert.Equal(t, cls.className, "default")

	cls = NewClassRoom(withClassName("二年级03班", 3))
	assert.Equal(t, cls.floor, 3)
	assert.Equal(t, len(cls.students), 0)
	assert.Equal(t, cls.className, "二年级03班")

	cls = NewClassRoom(withStudents(students))
	assert.Equal(t, cls.floor, 0)
	assert.Equal(t, cls.students, students)
	assert.Equal(t, cls.className, "default")

	cls = NewClassRoom(withClassName("二年级03班", 3), withStudents(students))
	assert.Equal(t, cls.className, "二年级03班")
	assert.Equal(t, cls.students, students)
	assert.Equal(t, cls.floor, 3)

	cls = NewClassRoom(withStudents(students), withClassName("二年级03班", 3))
	assert.Equal(t, cls.className, "二年级03班")
	assert.Equal(t, cls.students, students)
	assert.Equal(t, cls.floor, 3)
}

func TestOptionConstructorArg(t *testing.T) {

	t.Run("option default", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.RegisterBeanFn(NewClassRoom).Options()
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.RegisterBeanFn(NewClassRoom).Options(
			SpringCore.NewOptionArg(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}"),
		)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, cls.floor, 3)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "二年级03班")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName Condition", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.SetProperty("class_floor", 2)
		ctx.RegisterBeanFn(NewClassRoom).Options(
			SpringCore.NewOptionArg(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}",
			).ConditionOnProperty("class_name_enable"),
		)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName Apply", func(t *testing.T) {
		c := SpringCore.NewConditional().OnProperty("class_name_enable")

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.RegisterBeanFn(NewClassRoom).Options(
			SpringCore.NewOptionArg(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}",
			).ConditionOn(c),
		)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("class_name", "二年级03班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.RegisterBeanFn(NewClassRoom).Options(
			SpringCore.NewOptionArg(withStudents, ""),
		)
		ctx.RegisterBean([]*Student{
			new(Student), new(Student),
		})
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 2)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents withClassName", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("class_name", "二年级06班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.RegisterBeanFn(NewClassRoom).Options(
			SpringCore.NewOptionArg(withStudents, ""),
			SpringCore.NewOptionArg(
				withClassName, // 故意写反
				"1:${class_floor:=3}",
				"0:${class_name:=二年级03班}",
			),
		)
		ctx.RegisterBean([]*Student{
			new(Student), new(Student),
		})
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, cls.floor, 3)
		assert.Equal(t, len(cls.students), 2)
		assert.Equal(t, cls.className, "二年级06班")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})
}

type Server struct {
	Version string `value:"${server.version}"`
}

func NewServer() *Server {
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

type Service struct {
	Consumer *Consumer `autowire:""`
}

func TestDefaultSpringContext_RegisterMethodBean(t *testing.T) {

	t.Run("method bean", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.RegisterBean(new(Server))
		ctx.RegisterMethodBean(parent, "Consumer")
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		assert.Equal(t, ok, true)
		assert.Equal(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		assert.Equal(t, ok, true)
		assert.Equal(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean wire to other bean", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("server.version", "1.0.0")

		parent := ctx.RegisterBeanFn(NewServer)

		ctx.RegisterMethodBean(parent, "Consumer").
			DependsOn("*SpringCore_test.Server")

		ctx.RegisterBean(new(Service))
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		assert.Equal(t, ok, true)
		assert.Equal(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		assert.Equal(t, ok, true)
		assert.Equal(t, c.s.Version, "2.0.0")
	})

	t.Run("circle autowire", func(t *testing.T) {
		okCount := 0
		errCount := 0
		for i := 0; i < 20; i++ {
			func() {

				defer func() {
					if err := recover(); err != nil {
						errCount++

						var v string
						switch  e := err.(type) {
						case error:
							v = e.Error()
						case string:
							v = e
						}

						if !strings.Contains(v, "found circle autowire: ") {
							panic(errors.New("test error"))
						}
					} else {
						okCount++
					}
				}()

				ctx := SpringCore.NewDefaultSpringContext()
				ctx.SetProperty("server.version", "1.0.0")

				parent := ctx.RegisterBeanFn(NewServer).
					DependsOn("*SpringCore_test.Service")

				ctx.RegisterMethodBean(parent, "Consumer").
					DependsOn("*SpringCore_test.Server")

				ctx.RegisterBean(new(Service))
				ctx.AutoWireBeans()
			}()
		}
		fmt.Printf("ok:%d err:%d\n", okCount, errCount)
	})

	t.Run("method bean condition", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.RegisterBean(new(Server))
		ctx.RegisterMethodBean(parent, "Consumer").
			ConditionOnProperty("consumer.enable")
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		assert.Equal(t, ok, true)
		assert.Equal(t, s.Version, "1.0.0")

		var c *Consumer
		ok = ctx.GetBean(&c)
		assert.Equal(t, ok, false)
	})

	t.Run("method bean autowire", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.RegisterBean(new(Server))
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		assert.Equal(t, ok, true)
		assert.Equal(t, s.Version, "1.0.0")
	})
}

func TestDefaultSpringContext_ParentNotRegister(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	parent := ctx.RegisterBeanFn(NewServer).
		ConditionOnProperty("server.is.nil")
	ctx.RegisterMethodBean(parent, "Consumer")

	ctx.AutoWireBeans()

	var s *Server
	ok := ctx.GetBean(&s)
	assert.Equal(t, ok, false)

	var c *Consumer
	ok = ctx.GetBean(&c)
	assert.Equal(t, ok, false)
}

func TestDefaultSpringContext_UserDefinedTypeProperty(t *testing.T) {

	type level int

	SpringCore.RegisterTypeConverter(func(v string) level {
		switch v {
		case "debug":
			return 1
		default:
			panic(errors.New("error level"))
		}
	})

	var config struct {
		Duration time.Duration `value:"${duration}"`
		Level    level         `value:"${level}"`
		Time     time.Time     `value:"${time}"`
		Complex  complex64 // `value:"${complex}"`
	}

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("time", "2018-12-20")
	ctx.SetProperty("duration", "1h")
	ctx.SetProperty("level", "debug")
	ctx.SetProperty("complex", "1+i")
	ctx.RegisterBean(&config)
	ctx.AutoWireBeans()

	fmt.Println(config)
}

func TestDefaultSpringContext_ChainConditionOnBean(t *testing.T) {
	for i := 0; i < 20; i++ {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(string)).ConditionOnBean("*bool")
		ctx.RegisterBean(new(bool)).ConditionOnBean("*int")
		ctx.RegisterBean(new(int)).ConditionOnBean("*float")
		ctx.AutoWireBeans()
		assert.Equal(t, len(ctx.GetAllBeanDefinitions()), 0)
	}
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

func TestDefaultSpringContext_CircleAutowire(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()
	ctx.RegisterBean(new(CircleA))
	ctx.RegisterBean(new(CircleB))
	ctx.RegisterBean(new(CircleC))
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
}

func NewVarObj(options ...VarOptionFunc) *VarObj {
	opt := new(VarOption)
	for _, option := range options {
		option(opt)
	}
	return &VarObj{opt.v}
}

func TestDefaultSpringContext_RegisterOptionBean(t *testing.T) {

	t.Run("variadic option param 1", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterNameBean("v1", &Var{"v1"})
		ctx.RegisterNameBean("v2", &Var{"v2"})
		ctx.RegisterBeanFn(NewVarObj).Options(
			SpringCore.NewOptionArg(withVar, "v1"),
		)
		ctx.AutoWireBeans()

		var obj *VarObj
		ctx.GetBean(&obj)

		assert.Equal(t, len(obj.v), 1)
		assert.Equal(t, obj.v[0].name, "v1")
	})

	t.Run("variadic option param 2", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterNameBean("v1", &Var{"v1"})
		ctx.RegisterNameBean("v2", &Var{"v2"})
		ctx.RegisterBeanFn(NewVarObj).Options(
			SpringCore.NewOptionArg(withVar, "v1", "v2"),
		)
		ctx.AutoWireBeans()

		var obj *VarObj
		ctx.GetBean(&obj)

		assert.Equal(t, len(obj.v), 2)
		assert.Equal(t, obj.v[0].name, "v1")
		assert.Equal(t, obj.v[1].name, "v2")
	})

	t.Run("variadic option interface param 1", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterNameBean("v1", &Var{"v1"})
		ctx.RegisterNameBean("v2", &Var{"v2"})
		ctx.RegisterBeanFn(NewVarInterfaceObj).Options(
			SpringCore.NewOptionArg(withVarInterface, "v1"),
		)
		ctx.AutoWireBeans()

		var obj *VarInterfaceObj
		ctx.GetBean(&obj)

		assert.Equal(t, len(obj.v), 1)
	})

	t.Run("variadic option interface param 1", func(t *testing.T) {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterNameBean("v1", &Var{"v1"})
		ctx.RegisterNameBean("v2", &Var{"v2"})
		ctx.RegisterBeanFn(NewVarInterfaceObj).Options(
			SpringCore.NewOptionArg(withVarInterface, "v1", "v2"),
		)
		ctx.AutoWireBeans()

		var obj *VarInterfaceObj
		ctx.GetBean(&obj)

		assert.Equal(t, len(obj.v), 2)
	})
}

func TestDefaultSpringContext_Close(t *testing.T) {

	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(int)).Destroy(func() {})
	}, "destroy should be func\\(bean\\)")

	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(int)).Destroy(func() int { return 0 })
	}, "destroy should be func\\(bean\\)")

	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(int)).Destroy(func(int) {})
	}, "destroy should be func\\(bean\\)")

	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(int)).Destroy(func(int, int) {})
	}, "destroy should be func\\(bean\\)")

	called := false

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.RegisterBean(new(int)).Destroy(func(i *int) { called = true })
	ctx.AutoWireBeans()
	ctx.Close()

	assert.Equal(t, called, true)
}

func TestDefaultSpringContext_BeanNotFound(t *testing.T) {
	assert.Panic(t, func() {
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBeanFn(func(i *int) bool { return false })
		ctx.AutoWireBeans()
	}, "没有找到符合条件的 Bean: ")
}
