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
 * corestributed under the License is corestributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package core_test

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

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/core"
	pkg1 "github.com/go-spring/spring-core/core/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/core/testdata/pkg/foo"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
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
	util.AssertPanic(t, func() {
		ctx := core.NewApplicationContext()
		ctx.ObjBean(new(int)).Init(func(i *int) {
			// 不能在这里注册新的 RegisterBean
			ctx.ObjBean(new(bool))
		})
		ctx.AutoWireBeans()
	}, "bean registration have been frozen")
}

func TestApplicationContext(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		ctx := core.NewApplicationContext()

		e := int(3)
		a := []int{3}

		// 普通类型用属性注入
		util.AssertPanic(t, func() {
			ctx.ObjBean(e)
		}, "bean must be ref type")

		ctx.ObjBean(&e)

		// 这种错误延迟到 AutoWireBeans 阶段
		// // 相同类型的匿名 bean 不能重复注册
		// util.AssertPanic(t, func() {
		//	 ctx.RegisterBean(bean.ObjBean(&e))
		// }, "duplicate registration, bean: \"int:\\*int\"")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).Name("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).Name("i4")

		ctx.ObjBean(a)
		ctx.ObjBean(&a)
		ctx.AutoWireBeans()

		util.AssertPanic(t, func() {
			var i int
			ctx.GetBean(&i)
		}, "receiver must be ref type, bean: \"\\?\" field: ")

		// 找到多个符合条件的值
		util.AssertPanic(t, func() {
			var i *int
			ctx.GetBean(&i)
		}, "found 3 beans, bean: \"\\?\" field:  type: \\*int")

		// 入参不是可赋值的对象
		util.AssertPanic(t, func() {
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
		ctx := core.NewApplicationContext()

		e := pkg1.SamePkg{}
		a := []pkg1.SamePkg{{}}
		p := []*pkg1.SamePkg{{}}

		// 栈上的对象不能注册
		util.AssertPanic(t, func() {
			ctx.ObjBean(e)
		}, "bean must be ref type")

		ctx.ObjBean(&e)

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).Name("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).Name("i4")

		ctx.ObjBean(a)
		ctx.ObjBean(&a)
		ctx.ObjBean(p)
		ctx.ObjBean(&p)

		ctx.AutoWireBeans()
	})

	t.Run("pkg2.SamePkg", func(t *testing.T) {
		ctx := core.NewApplicationContext()

		e := pkg2.SamePkg{}
		a := []pkg2.SamePkg{{}}
		p := []*pkg2.SamePkg{{}}

		// 栈上的对象不能注册
		util.AssertPanic(t, func() {
			ctx.ObjBean(e)
		}, "bean must be ref type")

		ctx.ObjBean(&e)

		// 相同类型不同名称的 bean 都可注册
		// 不同类型相同名称的 bean 也可注册
		ctx.ObjBean(&e).Name("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.ObjBean(&e).Name("i4")

		ctx.ObjBean(a)
		ctx.ObjBean(&a)
		ctx.ObjBean(p)
		ctx.ObjBean(&p)

		ctx.ObjBean(&e).Name("i5")

		ctx.AutoWireBeans()
	})
}

type TestBincoreng struct {
	i int
}

func (b *TestBincoreng) String() string {
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
	StructByType *TestBincoreng `inject:""`
	StructByName *TestBincoreng `autowire:"struct_ptr"`

	// 自定义类型数组
	StructSliceByType []TestBincoreng `inject:""`
	StructSliceByName []TestBincoreng `autowire:"struct_slice"`

	// 自定义类型指针数组
	StructPtrSliceByType []*TestBincoreng `inject:""`
	StructPtrCollection  []*TestBincoreng `autowire:"[]"`
	StructPtrSliceByName []*TestBincoreng `autowire:"struct_ptr_slice"`

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
		ctx := core.NewApplicationContext()

		obj := &TestObject{}
		ctx.ObjBean(obj)

		i := int(3)
		ctx.ObjBean(&i).Name("int_ptr")

		i2 := int(3)
		ctx.ObjBean(&i2).Name("int_ptr_2")

		util.AssertPanic(t, func() {
			ctx.AutoWireBeans()
		}, "found 2 beans, bean: \"\" field: TestObject.\\$IntPtrByType type: \\*int")
	})

	ctx := core.NewApplicationContext()

	obj := &TestObject{}
	ctx.ObjBean(obj)

	i := int(3)
	ctx.ObjBean(&i).Name("int_ptr")

	is := []int{1, 2, 3}
	ctx.ObjBean(is).Name("int_slice_1")

	is2 := []int{2, 3, 4}
	ctx.ObjBean(is2).Name("int_slice_2")

	i2 := 4
	ips := []*int{&i2}
	ctx.ObjBean(ips).Name("int_ptr_slice")

	b := TestBincoreng{1}
	ctx.ObjBean(&b).Name("struct_ptr").Export((*fmt.Stringer)(nil))

	bs := []TestBincoreng{{10}}
	ctx.ObjBean(bs).Name("struct_slice")

	b2 := TestBincoreng{2}
	bps := []*TestBincoreng{&b2}
	ctx.ObjBean(bps).Name("struct_ptr_slice")

	s := []fmt.Stringer{&TestBincoreng{3}}
	ctx.ObjBean(s)

	m := map[string]interface{}{
		"5": 5,
	}

	ctx.ObjBean(m).Name("map")

	f1 := float32(11.0)
	ctx.ObjBean(&f1).Name("float_ptr_1")

	f2 := float32(12.0)
	ctx.ObjBean(&f2).Name("float_ptr_2")

	ctx.AutoWireBeans()

	var ff []*float32
	ctx.CollectBeans(&ff, "float_ptr_2", "float_ptr_1")
	util.AssertEqual(t, ff, []*float32{&f2, &f1})

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
	ctx := core.NewApplicationContext()

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
	Ctx core.ApplicationContext `autowire:""`
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
	ctx := core.NewApplicationContext()
	ctx.ObjBean(ctx).Export((*core.ApplicationContext)(nil))

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

func PointConverter(val string) (image.Point, error) {
	if !(strings.HasPrefix(val, "(") && strings.HasSuffix(val, ")")) {
		return image.Point{}, errors.New("数据格式错误")
	}
	ss := strings.Split(val[1:len(val)-1], ",")
	x := cast.ToInt(ss[0])
	y := cast.ToInt(ss[1])
	return image.Point{X: x, Y: y}, nil
}

type DB struct {
	UserName string `value:"${username}"`
	Password string `value:"${password}"`
	Url      string `value:"${url}"`
	Port     string `value:"${port}"`
	DB       string `value:"${db}"`
}

type DbConfig struct {
	DB []DB `value:"${db}"`
}

func TestApplicationContext_TypeConverter(t *testing.T) {
	ctx := core.NewApplicationContext()
	ctx.LoadProperties("testdata/config/application.yaml")

	b := &EnvEnumBean{}
	ctx.ObjBean(b)

	ctx.SetProperty("env.type", "test")

	p := &PointBean{}
	ctx.ObjBean(p)

	conf.Convert(PointConverter)
	ctx.SetProperty("point", "(7,5)")

	dbConfig := &DbConfig{}
	ctx.ObjBean(dbConfig)

	ctx.AutoWireBeans()

	util.AssertEqual(t, b.EnvType, ENV_TEST)

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
	ctx := core.NewApplicationContext()
	ctx.ObjBean(new(MyGrouper)).Export((*Grouper)(nil))
	ctx.ObjBean(new(ProxyGrouper))
	ctx.AutoWireBeans()
}

type Pkg interface {
	Package()
}

type SamePkgHolder struct {
	// Pkg `autowire:""` // 这种方式会找到多个符合条件的 RegisterBean
	Pkg `autowire:"github.com/go-spring/spring-core/core/testdata/pkg/bar/pkg.SamePkg:*pkg.SamePkg"`
}

func TestApplicationContext_SameNameBean(t *testing.T) {
	ctx := core.NewApplicationContext()
	ctx.ObjBean(new(SamePkgHolder))
	ctx.ObjBean(&pkg1.SamePkg{}).Export((*Pkg)(nil))
	ctx.ObjBean(&pkg2.SamePkg{}).Export((*Pkg)(nil))
	ctx.AutoWireBeans()
}

type DiffPkgOne struct {
}

func (d *DiffPkgOne) Package() {
	fmt.Println("github.com/go-spring/spring-core/core_test.DiffPkgOne")
}

type DiffPkgTwo struct {
}

func (d *DiffPkgTwo) Package() {
	fmt.Println("github.com/go-spring/spring-core/core_test.DiffPkgTwo")
}

type DiffPkgHolder struct {
	// Pkg `autowire:"same"` // 如果两个 RegisterBean 不小心重名了，也会找到多个符合条件的 RegisterBean
	Pkg `autowire:"github.com/go-spring/spring-core/core_test/core_test.DiffPkgTwo:same"`
}

func TestApplicationContext_DiffNameBean(t *testing.T) {
	ctx := core.NewApplicationContext()
	ctx.ObjBean(&DiffPkgOne{}).Name("same").Export((*Pkg)(nil))
	ctx.ObjBean(&DiffPkgTwo{}).Name("same").Export((*Pkg)(nil))
	ctx.ObjBean(new(DiffPkgHolder))
	ctx.AutoWireBeans()
}

func TestApplicationContext_LoadProperties(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.LoadProperties("testdata/config/application.yaml")
	ctx.LoadProperties("testdata/config/application.properties")

	val0 := ctx.GetProperty("spring.application.name")
	util.AssertEqual(t, val0, "test")

	val1 := ctx.GetProperty("yaml.list")
	util.AssertEqual(t, val1, []interface{}{1, 2})
}

func TestApplicationContext_GetBean(t *testing.T) {

	t.Run("panic", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.AutoWireBeans()

		util.AssertPanic(t, func() {
			var i int
			ctx.GetBean(i)
		}, "i must be pointer")

		util.AssertPanic(t, func() {
			var i *int
			ctx.GetBean(i)
		}, "receiver must be ref type")

		util.AssertPanic(t, func() {
			i := new(int)
			ctx.GetBean(i)
		}, "receiver must be ref type")

		var i *int
		ctx.GetBean(&i)

		util.AssertPanic(t, func() {
			var is []int
			ctx.GetBean(is)
		}, "i must be pointer")

		var a []int
		ctx.GetBean(&a)

		util.AssertPanic(t, func() {
			var s fmt.Stringer
			ctx.GetBean(s)
		}, "i can't be nil")

		var s fmt.Stringer
		ctx.GetBean(&s)
	})

	t.Run("success", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanTwo)).Export((*Grouper)(nil))
		ctx.AutoWireBeans()

		var two *BeanTwo
		ok := ctx.GetBean(&two)
		util.AssertEqual(t, ok, true)

		var grouper Grouper
		ok = ctx.GetBean(&grouper)
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, (*BeanTwo)(nil))
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, (*BeanTwo)(nil))
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, "")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "*core_test.BeanTwo")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, "*core_test.BeanTwo")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "BeanTwo")
		util.AssertEqual(t, ok, false)

		ok = ctx.GetBean(&grouper, "BeanTwo")
		util.AssertEqual(t, ok, false)

		ok = ctx.GetBean(&two, ":*core_test.BeanTwo")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, ":*core_test.BeanTwo")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "github.com/go-spring/spring-core/core_test/core_test.BeanTwo:*core_test.BeanTwo")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&grouper, "github.com/go-spring/spring-core/core_test/core_test.BeanTwo:*core_test.BeanTwo")
		util.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "xxx:*core_test.BeanTwo")
		util.AssertEqual(t, ok, false)

		ok = ctx.GetBean(&grouper, "xxx:*core_test.BeanTwo")
		util.AssertEqual(t, ok, false)

		var three *BeanThree
		ok = ctx.GetBean(&three, "")
		util.AssertEqual(t, ok, false)

		fmt.Println(ToString(two))
	})
}

func TestApplicationContext_FindBeanByName(t *testing.T) {
	ctx := core.NewApplicationContext()

	ctx.ObjBean(&BeanZero{5})
	ctx.ObjBean(new(BeanOne))
	ctx.ObjBean(new(BeanTwo))

	ctx.AutoWireBeans()

	util.AssertPanic(t, func() {
		ctx.FindBean("")
	}, "found 3 beans, bean: \"\"")

	i, ok := ctx.FindBean("*core_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	util.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean("BeanTwo")
	fmt.Println(ToString(i))
	util.AssertEqual(t, ok, false)

	i, ok = ctx.FindBean(":*core_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	util.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean("github.com/go-spring/spring-core/core_test/core_test.BeanTwo:*core_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	util.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean("xxx:*core_test.BeanTwo")
	fmt.Println(ToString(i))
	util.AssertEqual(t, ok, false)

	i, ok = ctx.FindBean("*core_test.BeanTwo")
	fmt.Println(ToString(i.Bean()))
	util.AssertEqual(t, ok, true)

	i, ok = ctx.FindBean((*BeanTwo)(nil))
	fmt.Println(ToString(i.Bean()))
	util.AssertEqual(t, ok, true)

	_, ok = ctx.FindBean((*fmt.Stringer)(nil))
	util.AssertEqual(t, ok, false)

	_, ok = ctx.FindBean((*Grouper)(nil))
	util.AssertEqual(t, ok, false)
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

func TestApplicationContext_RegisterBeanFn(t *testing.T) {
	ctx := core.NewApplicationContext()
	ctx.SetProperty("room", "Class 3 Grade 1")

	// 用接口注册时实际使用的是原始类型
	ctx.ObjBean(Teacher(newHistoryTeacher(""))).Export((*Teacher)(nil))

	ctx.CtorBean(NewStudent, "", "${room}").Name("st1")
	ctx.CtorBean(NewPtrStudent, "", "${room}").Name("st2")
	ctx.CtorBean(NewStudent, "?", "${room:=http://}").Name("st3")
	ctx.CtorBean(NewPtrStudent, "?", "${room:=4567}").Name("st4")

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

	util.AssertEqual(t, ok, true)
	fmt.Println(ToString(st1))
	util.AssertEqual(t, st1.Room, ctx.GetProperty("room"))

	var st2 *Student
	ok = ctx.GetBean(&st2, "st2")

	util.AssertEqual(t, ok, true)
	fmt.Println(ToString(st2))
	util.AssertEqual(t, st2.Room, ctx.GetProperty("room"))

	fmt.Printf("%x\n", reflect.ValueOf(st1).Pointer())
	fmt.Printf("%x\n", reflect.ValueOf(st2).Pointer())

	var st3 *Student
	ok = ctx.GetBean(&st3, "st3")

	util.AssertEqual(t, ok, true)
	fmt.Println(ToString(st3))
	util.AssertEqual(t, st3.Room, ctx.GetProperty("room"))

	var st4 *Student
	ok = ctx.GetBean(&st4, "st4")

	util.AssertEqual(t, ok, true)
	fmt.Println(ToString(st4))
	util.AssertEqual(t, st4.Room, ctx.GetProperty("room"))

	var m map[int]string
	ok = ctx.GetBean(&m)

	util.AssertEqual(t, ok, true)
	fmt.Println(ToString(m))
	util.AssertEqual(t, m[1], "ok")

	var s []int
	ok = ctx.GetBean(&s)

	util.AssertEqual(t, ok, true)
	fmt.Println(ToString(s))
	util.AssertEqual(t, s[1], 2)
}

func TestApplicationContext_Profile(t *testing.T) {

	t.Run("bean:_ctx:", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		util.AssertEqual(t, ok, true)
	})

	t.Run("bean:_ctx:test", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProfile("test")
		ctx.ObjBean(&BeanZero{5})
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		util.AssertEqual(t, ok, true)
	})
}

type BeanFour struct{}

func TestApplicationContext_DependsOn(t *testing.T) {

	t.Run("random", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanFour))
		ctx.AutoWireBeans()
	})

	t.Run("dependsOn", func(t *testing.T) {

		dependsOn := []bean.Selector{
			(*BeanOne)(nil), // 通过类型定义查找
			"github.com/go-spring/spring-core/core_test/core_test.BeanZero:*core_test.BeanZero",
		}

		ctx := core.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanFour)).DependsOn(dependsOn...)
		ctx.AutoWireBeans()
	})
}

func TestApplicationContext_Primary(t *testing.T) {

	t.Run("duplicate", func(t *testing.T) {

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(&BeanZero{5})
			ctx.ObjBean(&BeanZero{6})
			ctx.ObjBean(new(BeanOne))
			ctx.ObjBean(new(BeanTwo))
			ctx.AutoWireBeans()
		}, "duplicate registration, bean: ")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(&BeanZero{5})
			// primary 是在多个候选 bean 里面选择，而不是允许同名同类型的两个 bean
			ctx.ObjBean(&BeanZero{6}).SetPrimary(true)
			ctx.ObjBean(new(BeanOne))
			ctx.ObjBean(new(BeanTwo))
			ctx.AutoWireBeans()
		}, "duplicate registration, bean: ")
	})

	t.Run("not primary", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanTwo))
		ctx.AutoWireBeans()

		var b *BeanTwo
		ctx.GetBean(&b)
		util.AssertEqual(t, b.One.Zero.Int, 5)
	})

	t.Run("primary", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(&BeanZero{6}).Name("zero_6").SetPrimary(true)
		ctx.ObjBean(new(BeanOne))
		ctx.ObjBean(new(BeanTwo))
		ctx.AutoWireBeans()

		var b *BeanTwo
		ctx.GetBean(&b)
		util.AssertEqual(t, b.One.Zero.Int, 6)
	})
}

type FuncObj struct {
	Fn func(int) int `autowire:""`
}

func TestDefaultProperties_WireFunc(t *testing.T) {
	ctx := core.NewApplicationContext()
	ctx.ObjBean(func(int) int {
		return 6
	})
	obj := new(FuncObj)
	ctx.ObjBean(obj)
	ctx.AutoWireBeans()
	i := obj.Fn(3)
	util.AssertEqual(t, i, 6)
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

		ctx := core.NewApplicationContext()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.CtorBean(NewPtrManager)
		ctx.CtorBean(NewInt)
		ctx.AutoWireBeans()

		var m Manager
		ok := ctx.GetBean(&m)
		util.AssertEqual(t, ok, true)

		// 因为用户是按照接口注册的，所以理论上在依赖
		// 系统中用户并不关心接口对应的真实类型是什么。
		var lm *localManager
		ok = ctx.GetBean(&lm)
		util.AssertEqual(t, ok, false)
	})

	t.Run("manager", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("manager.version", "1.0.0")

		bd := ctx.CtorBean(NewManager)
		util.AssertEqual(t, bd.BeanName(), "core_test.Manager")

		bd = ctx.CtorBean(NewInt)
		util.AssertEqual(t, bd.BeanName(), "*int")

		ctx.AutoWireBeans()

		var m Manager
		ok := ctx.GetBean(&m)
		util.AssertEqual(t, ok, true)

		var lm *localManager
		ok = ctx.GetBean(&lm)
		util.AssertEqual(t, ok, false)
	})

	t.Run("manager return error", func(t *testing.T) {
		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.SetProperty("manager.version", "1.0.0")
			ctx.CtorBean(NewManagerRetError)
			ctx.AutoWireBeans()
		}, "return error")
	})

	t.Run("manager return error nil", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.CtorBean(NewManagerRetErrorNil)
		ctx.AutoWireBeans()
	})

	t.Run("manager return nil", func(t *testing.T) {
		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
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

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func() {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func() int { return 0 })
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func(int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Init(func(int, int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		ctx := core.NewApplicationContext()
		ctx.ObjBean(new(int)).Init(func(i *int) { *i = 3 })
		ctx.AutoWireBeans()

		var i *int
		ctx.GetBean(&i)
		util.AssertEqual(t, *i, 3)
	})

	t.Run("call init", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(new(callDestroy)).Init((*callDestroy).Init)
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.inited, true)
	})

	t.Run("call init with arg", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.ObjBean(new(callDestroy)).Init((*callDestroy).InitWithArg, "${version}")
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.inited, true)
	})

	t.Run("call init with error", func(t *testing.T) {

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.ObjBean(new(callDestroy)).Init((*callDestroy).InitWithError, "${int}")
			ctx.AutoWireBeans()
		}, "error")

		ctx := core.NewApplicationContext()
		ctx.SetProperty("int", 0)
		ctx.ObjBean(new(callDestroy)).Init((*callDestroy).InitWithError, "${int}")
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.inited, true)
	})

	t.Run("call interface init", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.Init)
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.(*callDestroy).inited, true)
	})

	t.Run("call interface init with arg", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithArg, "${version}")
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.(*callDestroy).inited, true)
	})

	t.Run("call interface init with error", func(t *testing.T) {

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithError, "${int}")
			ctx.AutoWireBeans()
		}, "error")

		ctx := core.NewApplicationContext()
		ctx.SetProperty("int", 0)
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithError, "${int}")
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.(*callDestroy).inited, true)
	})

	t.Run("call nested init", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(new(nestedCallDestroy)).Init((*nestedCallDestroy).Init)
		ctx.AutoWireBeans()

		var d *nestedCallDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.inited, true)
	})

	t.Run("call nested interface init", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(&nestedDestroyable{
			destroyable: new(callDestroy),
		}).Init((*nestedDestroyable).Init)
		ctx.AutoWireBeans()

		var d *nestedDestroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.destroyable.(*callDestroy).inited, true)
	})
}

type RecoresCluster struct {
	Endpoints string `value:"${recores.endpoints}"`

	RecoresConfig struct {
		Endpoints string `value:"${recores.endpoints}"`
	}

	Nested struct {
		RecoresConfig struct {
			Endpoints string `value:"${recores.endpoints}"`
		}
	}
}

func TestApplicationContext_ValueBincoreng(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
	ctx.ObjBean(new(RecoresCluster))
	ctx.AutoWireBeans()

	var cluster *RecoresCluster
	ctx.GetBean(&cluster)
	fmt.Println(cluster)

	util.AssertEqual(t, cluster.Endpoints, cluster.RecoresConfig.Endpoints)
	util.AssertEqual(t, cluster.Endpoints, cluster.Nested.RecoresConfig.Endpoints)
}

func TestApplicationContext_CollectBeans(t *testing.T) {

	t.Run("more than one *", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.ObjBean(new(RecoresCluster)).Name("one")
		ctx.ObjBean(new(RecoresCluster))
		ctx.AutoWireBeans()

		util.AssertPanic(t, func() {
			var rcs []*RecoresCluster
			ctx.CollectBeans(&rcs, "*", "*")
		}, "more than one \\* in collection \\[\\*,\\*]\\?")
	})

	t.Run("before *", func(t *testing.T) {

		d1 := new(RecoresCluster)
		d2 := new(RecoresCluster)

		ctx := core.NewApplicationContext()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.ObjBean(d1).Name("one")
		ctx.ObjBean(d2)
		ctx.AutoWireBeans()

		var rcs []*RecoresCluster
		ctx.CollectBeans(&rcs, "one", "*")

		util.AssertEqual(t, len(rcs), 2)
		util.AssertEqual(t, rcs[0], d1)
		util.AssertEqual(t, rcs[1], d2)
	})

	t.Run("after *", func(t *testing.T) {

		d1 := new(RecoresCluster)
		d2 := new(RecoresCluster)

		ctx := core.NewApplicationContext()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.ObjBean(d1).Name("one")
		ctx.ObjBean(d2)
		ctx.AutoWireBeans()

		var rcs []*RecoresCluster
		ctx.CollectBeans(&rcs, "one", "*")

		util.AssertEqual(t, len(rcs), 2)
		util.AssertEqual(t, rcs[1], d1)
		util.AssertEqual(t, rcs[0], d2)
	})

	t.Run("only *", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.ObjBean(new(RecoresCluster)).Name("one")
		ctx.ObjBean(new(RecoresCluster))
		ctx.AutoWireBeans()

		var rcs []*RecoresCluster
		ctx.CollectBeans(&rcs, "*")

		util.AssertEqual(t, len(rcs), 2)
	})

	ctx := core.NewApplicationContext()
	ctx.SetProperty("recores.endpoints", "recores://localhost:6379")

	ctx.ObjBean([]*RecoresCluster{new(RecoresCluster)})
	ctx.ObjBean([]RecoresCluster{{}})
	ctx.ObjBean(new(RecoresCluster))

	intBean := ctx.ObjBean(new(int)).Init(func(*int) {

		var rcs []*RecoresCluster
		ctx.CollectBeans(&rcs)
		fmt.Println(ToString(rcs))

		util.AssertEqual(t, len(rcs), 2)
		util.AssertEqual(t, rcs[0].Endpoints, "recores://localhost:6379")
	})
	util.AssertEqual(t, intBean.BeanName(), "*int")

	ctx.AutoWireBeans()

	var rcs []RecoresCluster
	ctx.GetBean(&rcs)
	fmt.Println(ToString(rcs))

	util.AssertEqual(t, len(rcs), 1)
	util.AssertEqual(t, rcs[0].Endpoints, "recores://localhost:6379")
}

func TestApplicationContext_WireSliceBean(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
	ctx.ObjBean([]*RecoresCluster{new(RecoresCluster)})
	ctx.ObjBean([]RecoresCluster{{}})
	ctx.AutoWireBeans()

	{
		var rcs []*RecoresCluster
		ctx.GetBean(&rcs)
		fmt.Println(ToString(rcs))

		util.AssertEqual(t, rcs[0].Endpoints, "recores://localhost:6379")
	}

	{
		var rcs []RecoresCluster
		ctx.GetBean(&rcs)
		fmt.Println(ToString(rcs))

		util.AssertEqual(t, rcs[0].Endpoints, "recores://localhost:6379")
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
	util.AssertEqual(t, cls.className, "default")

	cls = NewClassRoom(withClassName("二年级03班", 3))
	util.AssertEqual(t, cls.floor, 3)
	util.AssertEqual(t, len(cls.students), 0)
	util.AssertEqual(t, cls.className, "二年级03班")

	cls = NewClassRoom(withStudents(students))
	util.AssertEqual(t, cls.floor, 0)
	util.AssertEqual(t, cls.students, students)
	util.AssertEqual(t, cls.className, "default")

	cls = NewClassRoom(withClassName("二年级03班", 3), withStudents(students))
	util.AssertEqual(t, cls.className, "二年级03班")
	util.AssertEqual(t, cls.students, students)
	util.AssertEqual(t, cls.floor, 3)

	cls = NewClassRoom(withStudents(students), withClassName("二年级03班", 3))
	util.AssertEqual(t, cls.className, "二年级03班")
	util.AssertEqual(t, cls.students, students)
	util.AssertEqual(t, cls.floor, 3)
}

func TestOptionConstructorArg(t *testing.T) {

	t.Run("option default", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		util.AssertEqual(t, len(cls.students), 0)
		util.AssertEqual(t, cls.className, "default")
		util.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom, arg.Option(withClassName, "${class_name:=二年级03班}", "${class_floor:=3}"))
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		util.AssertEqual(t, cls.floor, 3)
		util.AssertEqual(t, len(cls.students), 0)
		util.AssertEqual(t, cls.className, "二年级03班")
		util.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("class_name", "二年级03班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom, arg.Option(withStudents, ""))
		ctx.ObjBean([]*Student{
			new(Student), new(Student),
		})
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		util.AssertEqual(t, cls.floor, 0)
		util.AssertEqual(t, len(cls.students), 2)
		util.AssertEqual(t, cls.className, "default")
		util.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents withClassName", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("class_name", "二年级06班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom,
			arg.Option(withStudents, ""),
			arg.Option(withClassName, "${class_name:=二年级03班}", "${class_floor:=3}"),
		)
		ctx.ObjBean([]*Student{
			new(Student), new(Student),
		})
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		util.AssertEqual(t, cls.floor, 3)
		util.AssertEqual(t, len(cls.students), 2)
		util.AssertEqual(t, cls.className, "二年级06班")
		util.AssertEqual(t, cls.President, "CaiYuanPei")
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

		ctx := core.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.ObjBean(new(Server))
		bd := ctx.CtorBean((*Server).Consumer, parent.BeanId())
		ctx.AutoWireBeans()
		util.AssertEqual(t, bd.BeanName(), "*core_test.Consumer")

		var s *Server
		ok := ctx.GetBean(&s)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean arg", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.ObjBean(new(Server))
		// ctx.CtorBean((*Server).ConsumerArg, "", "${i:=9}")
		ctx.CtorBean((*Server).ConsumerArg, parent.BeanId(), "${i:=9}")
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean wire to other bean", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.CtorBean(NewServerInterface)
		// ctx.CtorBean(ServerInterface.Consumer, "").DependsOn("core_test.ServerInterface")
		ctx.CtorBean(ServerInterface.Consumer, parent.BeanId()).DependsOn("core_test.ServerInterface")
		ctx.ObjBean(new(Service))
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		util.AssertEqual(t, ok, true)

		s := si.(*Server)
		util.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, c.s.Version, "2.0.0")
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

				ctx := core.NewApplicationContext()
				ctx.SetProperty("server.version", "1.0.0")
				parent := ctx.ObjBean(new(Server)).DependsOn("*core_test.Service")
				ctx.CtorBean((*Server).Consumer, parent.BeanId()).DependsOn("*core_test.Server")
				ctx.ObjBean(new(Service))
				ctx.AutoWireBeans()
			}()
		}
		fmt.Printf("ok:%d err:%d\n", okCount, errCount)
	})

	t.Run("method bean autowire", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.ObjBean(new(Server))
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, s.Version, "1.0.0")
	})

	t.Run("method bean selector type", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.ObjBean(new(Server))
		ctx.CtorBean(func(s *Server) *Consumer { return s.Consumer() }, (*Server)(nil))
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean selector type error", func(t *testing.T) {

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(Server))
			ctx.CtorBean(func(s *Server) *Consumer { return s.Consumer() }, (fmt.Stringer)(nil))
			ctx.AutoWireBeans()
		}, "selector can't be nil or empty")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(Server))
			ctx.CtorBean(func(s *Server) *Consumer { return s.Consumer() }, (*int)(nil))
			ctx.AutoWireBeans()
		}, "can't find bean, bean: \"int:\" field:  type: \\*core_test.Server")
	})

	t.Run("method bean selector beanId", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.ObjBean(new(Server))
		ctx.CtorBean(func(s *Server) *Consumer { return s.Consumer() }, "*core_test.Server")
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		ok = ctx.GetBean(&c)
		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean selector beanId error", func(t *testing.T) {
		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.ObjBean(new(Server))
			ctx.CtorBean(func(s *Server) *Consumer { return s.Consumer() }, "NULL")
			ctx.AutoWireBeans()
		}, "can't find bean, bean: \"NULL\" field:  type: \\*core_test.Server")
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

	ctx := core.NewApplicationContext()

	conf.Convert(func(v string) (level, error) {
		if v == "debug" {
			return 1, nil
		}
		return 0, errors.New("error level")
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
	// 直接创建的 RegisterBean 直接发生循环依赖是没有关系的。
	ctx := core.NewApplicationContext()
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

	t.Run("variable option param 1", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.SetProperty("var.obj", "description")
		ctx.ObjBean(&Var{"v1"}).Name("v1")
		ctx.ObjBean(&Var{"v2"}).Name("v2")
		ctx.CtorBean(NewVarObj, "${var.obj}", arg.Option(withVar, "v1"))
		ctx.AutoWireBeans()

		var obj *VarObj
		ctx.GetBean(&obj)

		util.AssertEqual(t, len(obj.v), 1)
		util.AssertEqual(t, obj.v[0].name, "v1")
		util.AssertEqual(t, obj.s, "description")
	})

	t.Run("variable option param 2", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.SetProperty("var.obj", "description")
		ctx.ObjBean(&Var{"v1"}).Name("v1")
		ctx.ObjBean(&Var{"v2"}).Name("v2")
		ctx.CtorBean(NewVarObj, "${var.obj}", arg.Option(withVar, "v1", "v2"))
		ctx.AutoWireBeans()

		var obj *VarObj
		ctx.GetBean(&obj)

		util.AssertEqual(t, len(obj.v), 2)
		util.AssertEqual(t, obj.v[0].name, "v1")
		util.AssertEqual(t, obj.v[1].name, "v2")
		util.AssertEqual(t, obj.s, "description")
	})

	t.Run("variable option interface param 1", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.ObjBean(&Var{"v1"}).Name("v1").Export((*interface{})(nil))
		ctx.ObjBean(&Var{"v2"}).Name("v2").Export((*interface{})(nil))
		ctx.CtorBean(NewVarInterfaceObj, arg.Option(withVarInterface, "v1"))
		ctx.AutoWireBeans()

		var obj *VarInterfaceObj
		ctx.GetBean(&obj)

		util.AssertEqual(t, len(obj.v), 1)
	})

	t.Run("variable option interface param 1", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.ObjBean(&Var{"v1"}).Name("v1").Export((*interface{})(nil))
		ctx.ObjBean(&Var{"v2"}).Name("v2").Export((*interface{})(nil))
		ctx.CtorBean(NewVarInterfaceObj, arg.Option(withVarInterface, "v1", "v2"))
		ctx.AutoWireBeans()

		var obj *VarInterfaceObj
		ctx.GetBean(&obj)

		util.AssertEqual(t, len(obj.v), 2)
	})
}

func TestApplicationContext_Close(t *testing.T) {

	t.Run("destroy type", func(t *testing.T) {

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func() {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func() int { return 0 })
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func(int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Destroy(func(int, int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")
	})

	t.Run("call destroy fn", func(t *testing.T) {
		called := false

		ctx := core.NewApplicationContext()
		ctx.ObjBean(new(int)).Destroy(func(i *int) { called = true })
		ctx.AutoWireBeans()
		ctx.Close()

		util.AssertEqual(t, called, true)
	})

	t.Run("call destroy", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).Destroy)
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.destroyed, true)
	})

	t.Run("call destroy with arg", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).DestroyWithArg, "${version}")
		ctx.AutoWireBeans()

		var d *callDestroy
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.destroyed, true)
	})

	t.Run("call destroy with error", func(t *testing.T) {

		// error
		{
			ctx := core.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d *callDestroy
			ok := ctx.GetBean(&d)

			ctx.Close()

			util.AssertEqual(t, ok, true)
			util.AssertEqual(t, d.destroyed, false)
		}

		// nil
		{
			ctx := core.NewApplicationContext()
			ctx.SetProperty("int", 0)
			ctx.ObjBean(new(callDestroy)).Destroy((*callDestroy).DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d *callDestroy
			ok := ctx.GetBean(&d)

			ctx.Close()

			util.AssertEqual(t, ok, true)
			util.AssertEqual(t, d.destroyed, true)
		}
	})

	t.Run("call interface destroy", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.Destroy)
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.(*callDestroy).destroyed, true)
	})

	t.Run("call interface destroy with arg", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("version", "v0.0.1")
		ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithArg, "${version}")
		ctx.AutoWireBeans()

		var d destroyable
		ok := ctx.GetBean(&d)

		ctx.Close()

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, d.(*callDestroy).destroyed, true)
	})

	t.Run("call interface destroy with error", func(t *testing.T) {

		// error
		{
			ctx := core.NewApplicationContext()
			ctx.SetProperty("int", 1)
			ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d destroyable
			ok := ctx.GetBean(&d)

			ctx.Close()

			util.AssertEqual(t, ok, true)
			util.AssertEqual(t, d.(*callDestroy).destroyed, false)
		}

		// nil
		{
			ctx := core.NewApplicationContext()
			ctx.SetProperty("int", 0)
			ctx.CtorBean(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithError, "${int}")
			ctx.AutoWireBeans()

			var d destroyable
			ok := ctx.GetBean(&d)

			ctx.Close()

			util.AssertEqual(t, ok, true)
			util.AssertEqual(t, d.(*callDestroy).destroyed, true)
		}
	})

	t.Run("context done", func(t *testing.T) {
		var wg sync.WaitGroup
		ctx := core.NewApplicationContext()
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
	util.AssertPanic(t, func() {
		ctx := core.NewApplicationContext()
		ctx.CtorBean(func(i *int) bool { return false }, "")
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

	ctx := core.NewApplicationContext()
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

	util.AssertEqual(t, ok, true)
	util.AssertEqual(t, *b.Int, 3)

	var b0 *PtrNestedAutowireBean
	ok = ctx.GetBean(&b0)

	util.AssertEqual(t, ok, true)
	util.AssertEqual(t, b0.Int, (*int)(nil))

	var b1 *FieldNestedAutowireBean
	ok = ctx.GetBean(&b1)

	util.AssertEqual(t, ok, true)
	util.AssertEqual(t, *b1.B.Int, 3)

	var b2 *PtrFieldNestedAutowireBean
	ok = ctx.GetBean(&b2)

	util.AssertEqual(t, ok, true)
	util.AssertEqual(t, b2.B.Int, (*int)(nil))
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

		ctx := core.NewApplicationContext()
		ctx.SetProperty("sdk.wx.auto-create", true)
		ctx.SetProperty("sdk.wx.enable", true)

		bd := ctx.CtorBean(func() int { return 3 })
		util.AssertEqual(t, bd.BeanName(), "*int")

		ctx.ObjBean(new(wxChannel))
		ctx.AutoWireBeans()

		var c *wxChannel
		ok := ctx.GetBean(&c)

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, *c.baseChannel.Int, 3)
		util.AssertEqual(t, *c.int, 3)
		util.AssertEqual(t, c.baseChannel.Int, c.int)
		util.AssertEqual(t, c.enable, true)
		util.AssertEqual(t, c.AutoCreate, true)
	})

	t.Run("public", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("sdk.wx.auto-create", true)
		ctx.SetProperty("sdk.wx.enable", true)
		ctx.CtorBean(func() int { return 3 })
		ctx.ObjBean(new(WXChannel))
		ctx.AutoWireBeans()

		var c *WXChannel
		ok := ctx.GetBean(&c)

		util.AssertEqual(t, ok, true)
		util.AssertEqual(t, *c.BaseChannel.Int, 3)
		util.AssertEqual(t, *c.Int, 3)
		util.AssertEqual(t, c.BaseChannel.Int, c.Int)
		util.AssertEqual(t, c.Enable, true)
		util.AssertEqual(t, c.AutoCreate, true)
	})
}

func TestApplicationContext_FnArgCollectBean(t *testing.T) {

	t.Run("base type", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.CtorBean(func() int { return 3 }).Name("i1")
		ctx.CtorBean(func() int { return 4 }).Name("i2")
		ctx.CtorBean(func(i []*int) bool {
			nums := make([]int, 0)
			for _, e := range i {
				nums = append(nums, *e)
			}
			sort.Ints(nums)
			util.AssertEqual(t, nums, []int{3, 4})
			return false
		}, "[]")
		ctx.AutoWireBeans()
	})

	t.Run("interface type", func(t *testing.T) {
		ctx := core.NewApplicationContext()
		ctx.ObjBean(newHistoryTeacher("t1")).Name("t1").Export((*Teacher)(nil))
		ctx.ObjBean(newHistoryTeacher("t2")).Name("t2").Export((*Teacher)(nil))
		ctx.CtorBean(func(teachers []Teacher) bool {
			names := make([]string, 0)
			for _, teacher := range teachers {
				names = append(names, teacher.(*historyTeacher).name)
			}
			sort.Strings(names)
			util.AssertEqual(t, names, []string{"t1", "t2"})
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
		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(new(int)).Export((*filter)(nil))
			ctx.AutoWireBeans()
		}, "not implement core_test.filter interface")
	})

	t.Run("implement interface", func(t *testing.T) {

		var server struct {
			F1 filter `autowire:"f1"`
			F2 filter `autowire:"f2"`
		}

		ctx := core.NewApplicationContext()
		ctx.CtorBean(func() filter { return new(filterImpl) }).Name("f1")
		ctx.ObjBean(new(filterImpl)).Export((*filter)(nil)).Name("f2")
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
	ctx := core.NewApplicationContext()
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

		ctx := core.NewApplicationContext()
		ctx.ObjBean(b)
		ctx.AutoWireBeans()

		var x context.Context
		ok := ctx.GetBean(&x)
		util.AssertEqual(t, true, ok)
		util.AssertEqual(t, b, x)

		var s fmt.Stringer
		ok = ctx.GetBean(&s)
		util.AssertEqual(t, true, ok)
		util.AssertEqual(t, b, s)

		var pbi ptrBaseInterface
		ok = ctx.GetBean(&pbi)
		util.AssertEqual(t, true, ok)
		util.AssertEqual(t, b, pbi)

		var bi baseInterface
		ok = ctx.GetBean(&bi)
		util.AssertEqual(t, true, ok)
		util.AssertEqual(t, b, bi)
	})

	t.Run("auto export private", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.CtorBean(pkg2.NewAppContext)
		ctx.AutoWireBeans()

		var x context.Context
		ok := ctx.GetBean(&x)
		util.AssertEqual(t, true, ok)

		var s fmt.Stringer
		ok = ctx.GetBean(&s)
		util.AssertEqual(t, true, ok)
	})

	t.Run("auto export & export", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}

		ctx := core.NewApplicationContext()
		ctx.ObjBean(b).Export((*fmt.Stringer)(nil))
		ctx.AutoWireBeans()

		var x context.Context
		ok := ctx.GetBean(&x)
		util.AssertEqual(t, true, ok)
		util.AssertEqual(t, b, x)

		var s fmt.Stringer
		ok = ctx.GetBean(&s)
		util.AssertEqual(t, true, ok)
		util.AssertEqual(t, b, s)
	})

	t.Run("unexported but auto match", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}
		ctx := core.NewApplicationContext()
		ctx.ObjBean(&struct {
			Error error `autowire:"e"`
		}{})
		ctx.ObjBean(b).Name("e")
		ctx.AutoWireBeans()
	})

	t.Run("export and match corerectly", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}
		ctx := core.NewApplicationContext()
		ctx.ObjBean(&struct {
			Error error `autowire:"e"`
		}{})
		ctx.ObjBean(b).Name("e").Export((*error)(nil))
		ctx.AutoWireBeans()
	})

	t.Run("panics", func(t *testing.T) {

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
			ctx.ObjBean(&struct {
				_ *int `export:""`
			}{})
			ctx.AutoWireBeans()
		}, "export can only use on interface")

		util.AssertPanic(t, func() {
			ctx := core.NewApplicationContext()
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
		b := new(ArrayProperties)
		ctx := core.NewApplicationContext()
		ctx.ObjBean(b)
		ctx.AutoWireBeans()
		util.AssertEqual(t, b.Duration, []time.Duration{time.Second, 5 * time.Second})
	})

	t.Run("map default value ", func(t *testing.T) {

		obj := struct {
			Int  int               `value:"${int:=5}"`
			IntA int               `value:"${int.a:=5}"`
			Map  map[string]string `value:"${map:={}}"`
			MapA map[string]string `value:"${map_a:={}}"`
		}{}

		ctx := core.NewApplicationContext()
		ctx.SetProperty("map_a.nba", "nba")
		ctx.SetProperty("map_a.cba", "cba")
		ctx.SetProperty("int.a", "3")
		ctx.SetProperty("int.b", "4")
		ctx.ObjBean(&obj)
		ctx.AutoWireBeans()

		util.AssertEqual(t, obj.Int, 5)
		util.AssertEqual(t, obj.IntA, 3)
		util.AssertEqual(t, obj.Map, map[string]string{})
		util.AssertEqual(t, obj.MapA, map[string]string{
			"cba": "cba", "nba": "nba",
		})
	})
}

func TestFnStringBincorengArg(t *testing.T) {
	ctx := core.NewApplicationContext()
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

	ctx := core.NewApplicationContext()
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

	util.AssertEqual(t, destroyArray, []int{1, 2, 2, 4})
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

	util.AssertPanic(t, func() {
		ctx := core.NewApplicationContext()
		ctx.ObjBean(DefaultRegistry)
		ctx.CtorBean(NewRegistry)
		ctx.AutoWireBeans()
	}, `duplicate registration, bean: `)

	util.AssertPanic(t, func() {
		ctx := core.NewApplicationContext()
		bd := ctx.ObjBean(&registryFactory{})
		ctx.CtorBean(func(f *registryFactory) Registry { return f.Create() }, bd)
		ctx.CtorBean(NewRegistryInterface)
		ctx.AutoWireBeans()
	}, `duplicate registration, bean: `)
}

type Obj struct {
	i int
}

type ObjFactory struct{}

func (factory *ObjFactory) NewObj(i int) *Obj { return &Obj{i: i} }

func TestApplicationContext_CreateBean(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.ObjBean(&ObjFactory{})
	ctx.AutoWireBeans()

	b, err := ctx.CreateBean((*ObjFactory).NewObj, "", "${i:=5}")
	fmt.Println(b, err)
}
