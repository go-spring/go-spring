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

package gs_test

import (
	"context"
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
	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs"
	pkg1 "github.com/go-spring/spring-core/gs/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/gs/testdata/pkg/foo"
	"github.com/go-spring/spring-core/json"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

func TestConditionContext(t *testing.T) {
	var _ = cond.Context((gs.Context)(nil))
}

func TestApplicationContext_RegisterBeanFrozen(t *testing.T) {
	assert.Panic(t, func() {
		ctx := gs.New()
		ctx.Object(new(int)).Init(func(i *int) {
			// 不能在这里注册新的 RegisterBean
			ctx.Object(new(bool))
		})
		ctx.Refresh()
	}, "bean registration have been frozen")
}

func TestApplicationContext(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		ctx := gs.New()

		e := int(3)
		a := []int{3}

		// 普通类型用属性注入
		assert.Panic(t, func() {
			ctx.Object(e)
		}, "bean must be ref type")

		ctx.Object(&e)

		// 这种错误延迟到 Refresh 阶段
		// // 相同类型的匿名 bean 不能重复注册
		// util.Panic(t, func() {
		//	 ctx.RegisterBean(bean.Bean(&e))
		// }, "duplicate registration, bean: \"int:\\*int\"")

		// 相同类型不同名称的 bean 都可注册
		ctx.Object(&e).WithName("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.Object(&e).WithName("i4")

		ctx.Object(a)
		ctx.Object(&a)
		ctx.Refresh()

		assert.Panic(t, func() {
			var i int
			err := ctx.GetObject(&i)
			util.Panic(err).When(err != nil)
		}, "receiver must be ref type, bean:\"\" field:\"\"")

		// 找到多个符合条件的值
		assert.Panic(t, func() {
			var i *int
			err := ctx.GetObject(&i)
			util.Panic(err).When(err != nil)
		}, "found 3 beans, bean:\"\" field:\"\" type:\"\\*int\"")

		// 入参不是可赋值的对象
		assert.Panic(t, func() {
			var i int
			err := ctx.GetObject(&i, "i3")
			util.Panic(err).When(err != nil)
		}, "receiver must be ref type, bean:\"i3\" field:\"\"")

		{
			var i *int
			err := ctx.GetObject(&i, "i3")
			assert.Nil(t, err)
		}

		{
			var i []int
			err := ctx.GetObject(&i)
			assert.Nil(t, err)
		}

		{
			var i *[]int
			err := ctx.GetObject(&i)
			assert.Nil(t, err)
		}
	})

	/////////////////////////////////////////
	// 自定义数据类型

	t.Run("pkg1.SamePkg", func(t *testing.T) {
		ctx := gs.New()

		e := pkg1.SamePkg{}
		a := []pkg1.SamePkg{{}}
		p := []*pkg1.SamePkg{{}}

		// 栈上的对象不能注册
		assert.Panic(t, func() {
			ctx.Object(e)
		}, "bean must be ref type")

		ctx.Object(&e)

		// 相同类型不同名称的 bean 都可注册
		ctx.Object(&e).WithName("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.Object(&e).WithName("i4")

		ctx.Object(a)
		ctx.Object(&a)
		ctx.Object(p)
		ctx.Object(&p)

		ctx.Refresh()
	})

	t.Run("pkg2.SamePkg", func(t *testing.T) {
		ctx := gs.New()

		e := pkg2.SamePkg{}
		a := []pkg2.SamePkg{{}}
		p := []*pkg2.SamePkg{{}}

		// 栈上的对象不能注册
		assert.Panic(t, func() {
			ctx.Object(e)
		}, "bean must be ref type")

		ctx.Object(&e)

		// 相同类型不同名称的 bean 都可注册
		// 不同类型相同名称的 bean 也可注册
		ctx.Object(&e).WithName("i3")

		// 相同类型不同名称的 bean 都可注册
		ctx.Object(&e).WithName("i4")

		ctx.Object(a)
		ctx.Object(&a)
		ctx.Object(p)
		ctx.Object(&p)

		ctx.Object(&e).WithName("i5")

		ctx.Refresh()
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
		ctx := gs.New()

		obj := &TestObject{}
		ctx.Object(obj)

		i := int(3)
		ctx.Object(&i).WithName("int_ptr")

		i2 := int(3)
		ctx.Object(&i2).WithName("int_ptr_2")

		assert.Panic(t, func() {
			ctx.Refresh()
		}, "found 2 beans, bean:\"\" field:\"TestObject.IntPtrByType\" type:\"\\*int\"")
	})

	ctx := gs.New()

	obj := &TestObject{}
	ctx.Object(obj)

	i := int(3)
	ctx.Object(&i).WithName("int_ptr")

	is := []int{1, 2, 3}
	ctx.Object(is).WithName("int_slice_1")

	is2 := []int{2, 3, 4}
	ctx.Object(is2).WithName("int_slice_2")

	i2 := 4
	ips := []*int{&i2}
	ctx.Object(ips).WithName("int_ptr_slice")

	b := TestBincoreng{1}
	ctx.Object(&b).WithName("struct_ptr").Export((*fmt.Stringer)(nil))

	bs := []TestBincoreng{{10}}
	ctx.Object(bs).WithName("struct_slice")

	b2 := TestBincoreng{2}
	bps := []*TestBincoreng{&b2}
	ctx.Object(bps).WithName("struct_ptr_slice")

	s := []fmt.Stringer{&TestBincoreng{3}}
	ctx.Object(s)

	m := map[string]interface{}{
		"5": 5,
	}

	ctx.Object(m).WithName("map")

	f1 := float32(11.0)
	ctx.Object(&f1).WithName("float_ptr_1")

	f2 := float32(12.0)
	ctx.Object(&f2).WithName("float_ptr_2")

	ctx.Refresh()

	var ff []*float32
	err := ctx.CollectBeans(&ff, "float_ptr_2", "float_ptr_1")
	assert.Nil(t, err)
	assert.Equal(t, ff, []*float32{&f2, &f1})

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
	ctx := gs.New()

	ctx.SetProperty("int", int(3))
	ctx.SetProperty("uint", uint(3))
	ctx.SetProperty("float", float32(3))
	ctx.SetProperty("complex", complex(3, 0))
	ctx.SetProperty("string", "3")
	ctx.SetProperty("bool", true)

	setting := &Setting{}
	ctx.Object(setting)

	ctx.SetProperty("sub.int", int(4))
	ctx.SetProperty("sub.sub.int", int(5))
	ctx.SetProperty("sub_sub.int", int(6))

	ctx.SetProperty("int_slice", []int{1, 2})
	ctx.SetProperty("string_slice", []string{"1", "2"})
	// ctx.SetProperty("float_slice", []float64{1, 2})

	ctx.Refresh()

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
	Ctx gs.Context `autowire:""`
}

func (f *PrototypeBeanFactory) New(name string) *PrototypeBean {
	b := &PrototypeBean{
		name: name,
		t:    time.Now(),
	}

	// PrototypeBean 依赖的服务可以通过 Context 注入
	_, err := f.Ctx.WireBean(b)
	util.Panic(err).When(err != nil)
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
	ctx := gs.New()
	ctx.Object(ctx).Export((*gs.Context)(nil))

	gs := &GreetingService{}
	ctx.Object(gs)

	s := &PrototypeBeanService{}
	ctx.Object(s)

	f := &PrototypeBeanFactory{}
	ctx.Object(f)

	ctx.Refresh()

	s.Service("Li Lei")
	time.Sleep(50 * time.Millisecond)

	s.Service("Jim Green")
	time.Sleep(50 * time.Millisecond)

	s.Service("Han MeiMei")
}

type EnvEnum string

const ENV_TEST EnvEnum = "test"

type EnvEnumBean struct {
	EnvType EnvEnum `value:"${env.type}"`
}

type PointBean struct {
	Point        image.Point   `value:"${point}"`
	DefaultPoint image.Point   `value:"${default_point:=(3,4)}"`
	PointList    []image.Point `value:"${point_list}"`
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
	ctx := gs.New("testdata/config/application.yaml")

	b := &EnvEnumBean{}
	ctx.Object(b)

	ctx.SetProperty("env.type", "test")

	p := &PointBean{}
	ctx.Object(p)

	conf.Convert(PointConverter)
	ctx.SetProperty("point", "(7,5)")

	dbConfig := &DbConfig{}
	ctx.Object(dbConfig)

	ctx.Refresh()

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

func TestApplicationContext_NestedBean(t *testing.T) {
	ctx := gs.New()
	ctx.Object(new(MyGrouper)).Export((*Grouper)(nil))
	ctx.Object(new(ProxyGrouper))
	ctx.Refresh()
}

type Pkg interface {
	Package()
}

type SamePkgHolder struct {
	// Pkg `autowire:""` // 这种方式会找到多个符合条件的 RegisterBean
	Pkg `autowire:"github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg:*pkg.SamePkg"`
}

func TestApplicationContext_SameNameBean(t *testing.T) {
	ctx := gs.New()
	ctx.Object(new(SamePkgHolder))
	ctx.Object(&pkg1.SamePkg{}).Export((*Pkg)(nil))
	ctx.Object(&pkg2.SamePkg{}).Export((*Pkg)(nil))
	ctx.Refresh()
}

type DiffPkgOne struct {
}

func (d *DiffPkgOne) Package() {
	fmt.Println("github.com/go-spring/spring-core/gs_test.DiffPkgOne")
}

type DiffPkgTwo struct {
}

func (d *DiffPkgTwo) Package() {
	fmt.Println("github.com/go-spring/spring-core/gs_test.DiffPkgTwo")
}

type DiffPkgHolder struct {
	// Pkg `autowire:"same"` // 如果两个 RegisterBean 不小心重名了，也会找到多个符合条件的 RegisterBean
	Pkg `autowire:"github.com/go-spring/spring-core/gs_test/gs_test.DiffPkgTwo:same"`
}

func TestApplicationContext_DiffNameBean(t *testing.T) {
	ctx := gs.New()
	ctx.Object(&DiffPkgOne{}).WithName("same").Export((*Pkg)(nil))
	ctx.Object(&DiffPkgTwo{}).WithName("same").Export((*Pkg)(nil))
	ctx.Object(new(DiffPkgHolder))
	ctx.Refresh()
}

func TestApplicationContext_LoadProperties(t *testing.T) {

	ctx := gs.New(
		"testdata/config/application.yaml",
		"testdata/config/application.properties",
	)

	val0 := ctx.GetProperty("spring.application.name")
	assert.Equal(t, val0, "test")

	val1 := ctx.GetProperty("yaml.list")
	assert.Equal(t, val1, []interface{}{1, 2})
}

func TestApplicationContext_GetBean(t *testing.T) {

	t.Run("panic", func(t *testing.T) {

		ctx := gs.New()
		ctx.Refresh()

		assert.Panic(t, func() {
			var i int
			err := ctx.GetObject(i)
			util.Panic(err).When(err != nil)
		}, "i must be pointer")

		assert.Panic(t, func() {
			var i *int
			err := ctx.GetObject(i)
			util.Panic(err).When(err != nil)
		}, "receiver must be ref type")

		assert.Panic(t, func() {
			i := new(int)
			err := ctx.GetObject(i)
			util.Panic(err).When(err != nil)
		}, "receiver must be ref type")

		assert.Panic(t, func() {
			var i *int
			err := ctx.GetObject(&i)
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"\"")

		assert.Panic(t, func() {
			var is []int
			err := ctx.GetObject(is)
			util.Panic(err).When(err != nil)
		}, "i must be pointer")

		assert.Panic(t, func() {
			var a []int
			err := ctx.GetObject(&a)
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"\"")

		assert.Panic(t, func() {
			var s fmt.Stringer
			err := ctx.GetObject(s)
			util.Panic(err).When(err != nil)
		}, "i can't be nil")

		assert.Panic(t, func() {
			var s fmt.Stringer
			err := ctx.GetObject(&s)
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"\"")
	})

	t.Run("success", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(&BeanZero{5})
		ctx.Object(new(BeanOne))
		ctx.Object(new(BeanTwo)).Export((*Grouper)(nil))
		ctx.Refresh()

		var two *BeanTwo
		err := ctx.GetObject(&two)
		assert.Nil(t, err)

		var grouper Grouper
		err = ctx.GetObject(&grouper)
		assert.Nil(t, err)

		err = ctx.GetObject(&two, (*BeanTwo)(nil))
		assert.Nil(t, err)

		err = ctx.GetObject(&grouper, (*BeanTwo)(nil))
		assert.Nil(t, err)

		err = ctx.GetObject(&two, "")
		assert.Nil(t, err)

		err = ctx.GetObject(&grouper, "")
		assert.Nil(t, err)

		err = ctx.GetObject(&two, "*gs_test.BeanTwo")
		assert.Nil(t, err)

		err = ctx.GetObject(&grouper, "*gs_test.BeanTwo")
		assert.Nil(t, err)

		assert.Panic(t, func() {
			err = ctx.GetObject(&two, "BeanTwo")
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"BeanTwo\"")

		assert.Panic(t, func() {
			err = ctx.GetObject(&grouper, "BeanTwo")
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"BeanTwo\"")

		err = ctx.GetObject(&two, ":*gs_test.BeanTwo")
		assert.Nil(t, err)

		err = ctx.GetObject(&grouper, ":*gs_test.BeanTwo")
		assert.Nil(t, err)

		err = ctx.GetObject(&two, "github.com/go-spring/spring-core/gs_test/gs_test.BeanTwo:*gs_test.BeanTwo")
		assert.Nil(t, err)

		err = ctx.GetObject(&grouper, "github.com/go-spring/spring-core/gs_test/gs_test.BeanTwo:*gs_test.BeanTwo")
		assert.Nil(t, err)

		assert.Panic(t, func() {
			err = ctx.GetObject(&two, "xxx:*gs_test.BeanTwo")
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"xxx:\\*gs_test.BeanTwo\"")

		assert.Panic(t, func() {
			err = ctx.GetObject(&grouper, "xxx:*gs_test.BeanTwo")
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"xxx:\\*gs_test.BeanTwo\"")

		assert.Panic(t, func() {
			var three *BeanThree
			err = ctx.GetObject(&three, "")
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"\"")
	})
}

func TestApplicationContext_FindBeanByName(t *testing.T) {

	ctx := gs.New()
	ctx.Object(&BeanZero{5})
	ctx.Object(new(BeanOne))
	ctx.Object(new(BeanTwo))
	ctx.Refresh()

	b, _ := ctx.FindBean("")
	assert.Equal(t, len(b), 3)

	b, _ = ctx.FindBean("BeanTwo")
	fmt.Println(json.ToString(b))
	assert.Equal(t, len(b), 0)

	b, _ = ctx.FindBean("*gs_test.BeanTwo")
	fmt.Println(json.ToString(b))
	assert.Equal(t, len(b), 1)

	b, _ = ctx.FindBean(":*gs_test.BeanTwo")
	fmt.Println(json.ToString(b))
	assert.Equal(t, len(b), 1)

	b, _ = ctx.FindBean("github.com/go-spring/spring-core/gs_test/gs_test.BeanTwo:*gs_test.BeanTwo")
	fmt.Println(json.ToString(b))
	assert.Equal(t, len(b), 1)

	b, _ = ctx.FindBean("xxx:*gs_test.BeanTwo")
	fmt.Println(json.ToString(b))
	assert.Equal(t, len(b), 0)

	b, _ = ctx.FindBean((*BeanTwo)(nil))
	fmt.Println(json.ToString(b))
	assert.Equal(t, len(b), 1)

	b, _ = ctx.FindBean((*fmt.Stringer)(nil))
	assert.Equal(t, len(b), 0)

	b, _ = ctx.FindBean((*Grouper)(nil))
	assert.Equal(t, len(b), 0)
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
	ctx := gs.New()
	ctx.SetProperty("room", "Class 3 Grade 1")

	// 用接口注册时实际使用的是原始类型
	ctx.Object(Teacher(newHistoryTeacher(""))).Export((*Teacher)(nil))

	ctx.Factory(NewStudent, "", "${room}").WithName("st1")
	ctx.Factory(NewPtrStudent, "", "${room}").WithName("st2")
	ctx.Factory(NewStudent, "?", "${room:=http://}").WithName("st3")
	ctx.Factory(NewPtrStudent, "?", "${room:=4567}").WithName("st4")

	mapFn := func() map[int]string {
		return map[int]string{
			1: "ok",
		}
	}

	ctx.Factory(mapFn)

	sliceFn := func() []int {
		return []int{1, 2}
	}

	ctx.Factory(sliceFn)

	ctx.Refresh()

	var st1 *Student
	err := ctx.GetObject(&st1, "st1")

	assert.Nil(t, err)
	fmt.Println(json.ToString(st1))
	assert.Equal(t, st1.Room, ctx.GetProperty("room"))

	var st2 *Student
	err = ctx.GetObject(&st2, "st2")

	assert.Nil(t, err)
	fmt.Println(json.ToString(st2))
	assert.Equal(t, st2.Room, ctx.GetProperty("room"))

	fmt.Printf("%x\n", reflect.ValueOf(st1).Pointer())
	fmt.Printf("%x\n", reflect.ValueOf(st2).Pointer())

	var st3 *Student
	err = ctx.GetObject(&st3, "st3")

	assert.Nil(t, err)
	fmt.Println(json.ToString(st3))
	assert.Equal(t, st3.Room, ctx.GetProperty("room"))

	var st4 *Student
	err = ctx.GetObject(&st4, "st4")

	assert.Nil(t, err)
	fmt.Println(json.ToString(st4))
	assert.Equal(t, st4.Room, ctx.GetProperty("room"))

	var m map[int]string
	err = ctx.GetObject(&m)

	assert.Nil(t, err)
	fmt.Println(json.ToString(m))
	assert.Equal(t, m[1], "ok")

	var s []int
	err = ctx.GetObject(&s)

	assert.Nil(t, err)
	fmt.Println(json.ToString(s))
	assert.Equal(t, s[1], 2)
}

func TestApplicationContext_Profile(t *testing.T) {

	t.Run("bean:_ctx:", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(&BeanZero{5})
		ctx.Refresh()

		var b *BeanZero
		err := ctx.GetObject(&b)
		assert.Nil(t, err)
	})

	t.Run("bean:_ctx:test", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProfile("test")
		ctx.Object(&BeanZero{5})
		ctx.Refresh()

		var b *BeanZero
		err := ctx.GetObject(&b)
		assert.Nil(t, err)
	})
}

type BeanFour struct{}

func TestApplicationContext_DependsOn(t *testing.T) {

	t.Run("random", func(t *testing.T) {
		ctx := gs.New()
		ctx.Object(&BeanZero{5})
		ctx.Object(new(BeanOne))
		ctx.Object(new(BeanFour))
		ctx.Refresh()
	})

	t.Run("dependsOn", func(t *testing.T) {

		dependsOn := []bean.Selector{
			(*BeanOne)(nil), // 通过类型定义查找
			"github.com/go-spring/spring-core/gs_test/gs_test.BeanZero:*gs_test.BeanZero",
		}

		ctx := gs.New()
		ctx.Object(&BeanZero{5})
		ctx.Object(new(BeanOne))
		ctx.Object(new(BeanFour)).DependsOn(dependsOn...)
		ctx.Refresh()
	})
}

func TestApplicationContext_Primary(t *testing.T) {

	t.Run("duplicate", func(t *testing.T) {

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(&BeanZero{5})
			ctx.Object(&BeanZero{6})
			ctx.Object(new(BeanOne))
			ctx.Object(new(BeanTwo))
			ctx.Refresh()
		}, "duplicate registration, bean:")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(&BeanZero{5})
			// primary 是在多个候选 bean 里面选择，而不是允许同名同类型的两个 bean
			ctx.Object(&BeanZero{6}).Primary(true)
			ctx.Object(new(BeanOne))
			ctx.Object(new(BeanTwo))
			ctx.Refresh()
		}, "duplicate registration, bean:")
	})

	t.Run("not primary", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(&BeanZero{5})
		ctx.Object(new(BeanOne))
		ctx.Object(new(BeanTwo))
		ctx.Refresh()

		var b *BeanTwo
		err := ctx.GetObject(&b)
		assert.Nil(t, err)
		assert.Equal(t, b.One.Zero.Int, 5)
	})

	t.Run("primary", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(&BeanZero{5})
		ctx.Object(&BeanZero{6}).WithName("zero_6").Primary(true)
		ctx.Object(new(BeanOne))
		ctx.Object(new(BeanTwo))
		ctx.Refresh()

		var b *BeanTwo
		err := ctx.GetObject(&b)
		assert.Nil(t, err)
		assert.Equal(t, b.One.Zero.Int, 6)
	})
}

type FuncObj struct {
	Fn func(int) int `autowire:""`
}

func TestDefaultProperties_WireFunc(t *testing.T) {
	ctx := gs.New()
	ctx.Object(func(int) int { return 6 })
	obj := new(FuncObj)
	ctx.Object(obj)
	ctx.Refresh()
	i := obj.Fn(3)
	assert.Equal(t, i, 6)
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

		ctx := gs.New()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.Factory(NewPtrManager)
		ctx.Factory(NewInt)
		ctx.Refresh()

		var m Manager
		err := ctx.GetObject(&m)
		assert.Nil(t, err)

		assert.Panic(t, func() {
			// 因为用户是按照接口注册的，所以理论上在依赖
			// 系统中用户并不关心接口对应的真实类型是什么。
			var lm *localManager
			err = ctx.GetObject(&lm)
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"\"")
	})

	t.Run("manager", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("manager.version", "1.0.0")

		bd := ctx.Factory(NewManager)
		assert.Equal(t, bd.BeanName(), "gs_test.Manager")

		bd = ctx.Factory(NewInt)
		assert.Equal(t, bd.BeanName(), "*int")

		ctx.Refresh()

		var m Manager
		err := ctx.GetObject(&m)
		assert.Nil(t, err)

		assert.Panic(t, func() {
			var lm *localManager
			err = ctx.GetObject(&lm)
			util.Panic(err).When(err != nil)
		}, "can't find bean, bean:\"\"")
	})

	t.Run("manager return error", func(t *testing.T) {
		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.SetProperty("manager.version", "1.0.0")
			ctx.Factory(NewManagerRetError)
			ctx.Refresh()
		}, "return error")
	})

	t.Run("manager return error nil", func(t *testing.T) {
		ctx := gs.New()
		ctx.SetProperty("manager.version", "1.0.0")
		ctx.Factory(NewManagerRetErrorNil)
		ctx.Refresh()
	})

	t.Run("manager return nil", func(t *testing.T) {
		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.SetProperty("manager.version", "1.0.0")
			ctx.Factory(NewNullPtrManager)
			ctx.Refresh()
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

func (d *callDestroy) InitWithArg(_ string) {
	d.inited = true
}

func (d *callDestroy) DestroyWithArg(_ string) {
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

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Init(func() {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Init(func() int { return 0 })
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Init(func(int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Init(func(int, int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		ctx := gs.New()
		ctx.Object(new(int)).Init(func(i *int) { *i = 3 })
		ctx.Refresh()

		var i *int
		err := ctx.GetObject(&i)
		assert.Nil(t, err)
		assert.Equal(t, *i, 3)
	})

	t.Run("call init", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(new(callDestroy)).Init((*callDestroy).Init)
		ctx.Refresh()

		var d *callDestroy
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.inited)
	})

	t.Run("call init with arg", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("version", "v0.0.1")
		ctx.Object(new(callDestroy)).Init((*callDestroy).InitWithArg, "${version}")
		ctx.Refresh()

		var d *callDestroy
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.inited)
	})

	t.Run("call init with error", func(t *testing.T) {

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.SetProperty("int", 1)
			ctx.Object(new(callDestroy)).Init((*callDestroy).InitWithError, "${int}")
			ctx.Refresh()
		}, "error")

		ctx := gs.New()
		ctx.SetProperty("int", 0)
		ctx.Object(new(callDestroy)).Init((*callDestroy).InitWithError, "${int}")
		ctx.Refresh()

		var d *callDestroy
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.inited)
	})

	t.Run("call interface init", func(t *testing.T) {

		ctx := gs.New()
		ctx.Factory(func() destroyable { return new(callDestroy) }).Init(destroyable.Init)
		ctx.Refresh()

		var d destroyable
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.(*callDestroy).inited)
	})

	t.Run("call interface init with arg", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("version", "v0.0.1")
		ctx.Factory(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithArg, "${version}")
		ctx.Refresh()

		var d destroyable
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.(*callDestroy).inited)
	})

	t.Run("call interface init with error", func(t *testing.T) {

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.SetProperty("int", 1)
			ctx.Factory(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithError, "${int}")
			ctx.Refresh()
		}, "error")

		ctx := gs.New()
		ctx.SetProperty("int", 0)
		ctx.Factory(func() destroyable { return new(callDestroy) }).Init(destroyable.InitWithError, "${int}")
		ctx.Refresh()

		var d destroyable
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.(*callDestroy).inited)
	})

	t.Run("call nested init", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(new(nestedCallDestroy)).Init((*nestedCallDestroy).Init)
		ctx.Refresh()

		var d *nestedCallDestroy
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.inited)
	})

	t.Run("call nested interface init", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(&nestedDestroyable{
			destroyable: new(callDestroy),
		}).Init((*nestedDestroyable).Init)
		ctx.Refresh()

		var d *nestedDestroyable
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.destroyable.(*callDestroy).inited)
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

	ctx := gs.New()
	ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
	ctx.Object(new(RecoresCluster))
	ctx.Refresh()

	var cluster *RecoresCluster
	err := ctx.GetObject(&cluster)
	fmt.Println(cluster)

	assert.Nil(t, err)
	assert.Equal(t, cluster.Endpoints, cluster.RecoresConfig.Endpoints)
	assert.Equal(t, cluster.Endpoints, cluster.Nested.RecoresConfig.Endpoints)
}

func TestApplicationContext_CollectBeans(t *testing.T) {

	t.Run("more than one *", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.Object(new(RecoresCluster)).WithName("one")
		ctx.Object(new(RecoresCluster))
		ctx.Refresh()

		var rcs []*RecoresCluster
		err := ctx.CollectBeans(&rcs, "*", "*")
		assert.Error(t, err, "more than one \\* in collection \"\\[\\*,\\*]\"")
	})

	t.Run("before *", func(t *testing.T) {

		d1 := new(RecoresCluster)
		d2 := new(RecoresCluster)

		ctx := gs.New()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.Object(d1).WithName("one")
		ctx.Object(d2)
		ctx.Refresh()

		var rcs []*RecoresCluster
		err := ctx.CollectBeans(&rcs, "one", "*")
		assert.Nil(t, err)

		assert.Equal(t, len(rcs), 2)
		assert.Equal(t, rcs[0], d1)
		assert.Equal(t, rcs[1], d2)
	})

	t.Run("after *", func(t *testing.T) {

		d1 := new(RecoresCluster)
		d2 := new(RecoresCluster)

		ctx := gs.New()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.Object(d1).WithName("one")
		ctx.Object(d2)
		ctx.Refresh()

		var rcs []*RecoresCluster
		err := ctx.CollectBeans(&rcs, "one", "*")
		assert.Nil(t, err)

		assert.Equal(t, len(rcs), 2)
		assert.Equal(t, rcs[1], d1)
		assert.Equal(t, rcs[0], d2)
	})

	t.Run("only *", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
		ctx.Object(new(RecoresCluster)).WithName("one")
		ctx.Object(new(RecoresCluster))
		ctx.Refresh()

		var rcs []*RecoresCluster
		err := ctx.CollectBeans(&rcs, "*")

		assert.Nil(t, err)
		assert.Equal(t, len(rcs), 2)
	})

	ctx := gs.New()
	ctx.SetProperty("recores.endpoints", "recores://localhost:6379")

	ctx.Object([]*RecoresCluster{new(RecoresCluster)})
	ctx.Object([]RecoresCluster{{}})
	ctx.Object(new(RecoresCluster))

	intBean := ctx.Object(new(int)).Init(func(*int) {

		var rcs []*RecoresCluster
		err := ctx.CollectBeans(&rcs)
		fmt.Println(json.ToString(rcs))

		assert.Nil(t, err)
		assert.Equal(t, len(rcs), 2)
		assert.Equal(t, rcs[0].Endpoints, "recores://localhost:6379")
	})
	assert.Equal(t, intBean.BeanName(), "*int")

	ctx.Refresh()

	var rcs []RecoresCluster
	err := ctx.GetObject(&rcs)
	fmt.Println(json.ToString(rcs))

	assert.Nil(t, err)
	assert.Equal(t, len(rcs), 1)
	assert.Equal(t, rcs[0].Endpoints, "recores://localhost:6379")
}

func TestApplicationContext_WireSliceBean(t *testing.T) {

	ctx := gs.New()
	ctx.SetProperty("recores.endpoints", "recores://localhost:6379")
	ctx.Object([]*RecoresCluster{new(RecoresCluster)})
	ctx.Object([]RecoresCluster{{}})
	ctx.Refresh()

	{
		var rcs []*RecoresCluster
		err := ctx.GetObject(&rcs)
		fmt.Println(json.ToString(rcs))

		assert.Nil(t, err)
		assert.Equal(t, rcs[0].Endpoints, "recores://localhost:6379")
	}

	{
		var rcs []RecoresCluster
		err := ctx.GetObject(&rcs)
		fmt.Println(json.ToString(rcs))

		assert.Nil(t, err)
		assert.Equal(t, rcs[0].Endpoints, "recores://localhost:6379")
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

		ctx := gs.New()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.Factory(NewClassRoom)
		ctx.Refresh()

		var cls *ClassRoom
		err := ctx.GetObject(&cls)

		assert.Nil(t, err)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.Factory(NewClassRoom, arg.Option(withClassName, "${class_name:=二年级03班}", "${class_floor:=3}"))
		ctx.Refresh()

		var cls *ClassRoom
		err := ctx.GetObject(&cls)

		assert.Nil(t, err)
		assert.Equal(t, cls.floor, 3)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "二年级03班")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("class_name", "二年级03班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.Factory(NewClassRoom, arg.Option(withStudents, ""))
		ctx.Object([]*Student{new(Student), new(Student)})
		ctx.Refresh()

		var cls *ClassRoom
		err := ctx.GetObject(&cls)

		assert.Nil(t, err)
		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 2)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withStudents withClassName", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("class_name", "二年级06班")
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.Factory(NewClassRoom,
			arg.Option(withStudents, ""),
			arg.Option(withClassName, "${class_name:=二年级03班}", "${class_floor:=3}"),
		)
		ctx.Object([]*Student{
			new(Student), new(Student),
		})
		ctx.Refresh()

		var cls *ClassRoom
		err := ctx.GetObject(&cls)

		assert.Nil(t, err)
		assert.Equal(t, cls.floor, 3)
		assert.Equal(t, len(cls.students), 2)
		assert.Equal(t, cls.className, "二年级06班")
		assert.Equal(t, cls.President, "CaiYuanPei")
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

func (s *Server) ConsumerArg(_ int) *Consumer {
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

		ctx := gs.New()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.Object(new(Server))
		bd := ctx.Factory((*Server).Consumer, parent.BeanId())
		ctx.Refresh()
		assert.Equal(t, bd.BeanName(), "*gs_test.Consumer")

		var s *Server
		err := ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		err = ctx.GetObject(&c)
		assert.Nil(t, err)
		assert.Equal(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean arg", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.Object(new(Server))
		// ctx.Bean((*Server).ConsumerArg, "", "${i:=9}")
		ctx.Factory((*Server).ConsumerArg, parent.BeanId(), "${i:=9}")
		ctx.Refresh()

		var s *Server
		err := ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		err = ctx.GetObject(&c)
		assert.Nil(t, err)
		assert.Equal(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean wire to other bean", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.Factory(NewServerInterface)
		// ctx.Factory(ServerInterface.Consumer, "").DependsOn("gs_test.ServerInterface")
		ctx.Factory(ServerInterface.Consumer, parent.BeanId()).DependsOn("gs_test.ServerInterface")
		ctx.Object(new(Service))
		ctx.Refresh()

		var si ServerInterface
		err := ctx.GetObject(&si)
		assert.Nil(t, err)

		s := si.(*Server)
		assert.Equal(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		err = ctx.GetObject(&c)
		assert.Nil(t, err)
		assert.Equal(t, c.s.Version, "2.0.0")
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

				ctx := gs.New()
				ctx.SetProperty("server.version", "1.0.0")
				parent := ctx.Object(new(Server)).DependsOn("*gs_test.Service")
				ctx.Factory((*Server).Consumer, parent.BeanId()).DependsOn("*gs_test.Server")
				ctx.Object(new(Service))
				ctx.Refresh()
			}()
		}
		fmt.Printf("ok:%d err:%d\n", okCount, errCount)
	})

	t.Run("method bean autowire", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.Object(new(Server))
		ctx.Refresh()

		var s *Server
		err := ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.Version, "1.0.0")
	})

	t.Run("method bean selector type", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.Object(new(Server))
		ctx.Factory(func(s *Server) *Consumer { return s.Consumer() }, (*Server)(nil))
		ctx.Refresh()

		var s *Server
		err := ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		err = ctx.GetObject(&c)
		assert.Nil(t, err)
		assert.Equal(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean selector type error", func(t *testing.T) {
		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.Object(new(Server))
			ctx.Factory(func(s *Server) *Consumer { return s.Consumer() }, (*int)(nil))
			ctx.Refresh()
		}, "can't find bean, bean:\"int:\" field:\"\" type:\"\\*gs_test.Server\"")
	})

	t.Run("method bean selector beanId", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.Object(new(Server))
		ctx.Factory(func(s *Server) *Consumer { return s.Consumer() }, "*gs_test.Server")
		ctx.Refresh()

		var s *Server
		err := ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.Version, "1.0.0")

		s.Version = "2.0.0"

		var c *Consumer
		err = ctx.GetObject(&c)
		assert.Nil(t, err)
		assert.Equal(t, c.s.Version, "2.0.0")
	})

	t.Run("method bean selector beanId error", func(t *testing.T) {
		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.SetProperty("server.version", "1.0.0")
			ctx.Object(new(Server))
			ctx.Factory(func(s *Server) *Consumer { return s.Consumer() }, "NULL")
			ctx.Refresh()
		}, "can't find bean, bean:\"NULL\" field:\"\" type:\"\\*gs_test.Server\"")
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

	ctx := gs.New()

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
	ctx.Object(&config)
	ctx.Refresh()

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
	ctx := gs.New()
	ctx.Object(new(CircleA))
	ctx.Object(new(CircleB))
	ctx.Object(new(CircleC))
	ctx.Refresh()
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
		ctx := gs.New()
		ctx.SetProperty("var.obj", "description")
		ctx.Object(&Var{"v1"}).WithName("v1")
		ctx.Object(&Var{"v2"}).WithName("v2")
		ctx.Factory(NewVarObj, "${var.obj}", arg.Option(withVar, "v1"))
		ctx.Refresh()

		var obj *VarObj
		err := ctx.GetObject(&obj)

		assert.Nil(t, err)
		assert.Equal(t, len(obj.v), 1)
		assert.Equal(t, obj.v[0].name, "v1")
		assert.Equal(t, obj.s, "description")
	})

	t.Run("variable option param 2", func(t *testing.T) {
		ctx := gs.New()
		ctx.SetProperty("var.obj", "description")
		ctx.Object(&Var{"v1"}).WithName("v1")
		ctx.Object(&Var{"v2"}).WithName("v2")
		ctx.Factory(NewVarObj, arg.Value("description"), arg.Option(withVar, "v1", "v2"))
		ctx.Refresh()

		var obj *VarObj
		err := ctx.GetObject(&obj)

		assert.Nil(t, err)
		assert.Equal(t, len(obj.v), 2)
		assert.Equal(t, obj.v[0].name, "v1")
		assert.Equal(t, obj.v[1].name, "v2")
		assert.Equal(t, obj.s, "description")
	})

	t.Run("variable option interface param 1", func(t *testing.T) {
		ctx := gs.New()
		ctx.Object(&Var{"v1"}).WithName("v1").Export((*interface{})(nil))
		ctx.Object(&Var{"v2"}).WithName("v2").Export((*interface{})(nil))
		ctx.Factory(NewVarInterfaceObj, arg.Option(withVarInterface, "v1"))
		ctx.Refresh()

		var obj *VarInterfaceObj
		err := ctx.GetObject(&obj)

		assert.Nil(t, err)
		assert.Equal(t, len(obj.v), 1)
	})

	t.Run("variable option interface param 1", func(t *testing.T) {
		ctx := gs.New()
		ctx.Object(&Var{"v1"}).WithName("v1").Export((*interface{})(nil))
		ctx.Object(&Var{"v2"}).WithName("v2").Export((*interface{})(nil))
		ctx.Factory(NewVarInterfaceObj, arg.Option(withVarInterface, "v1", "v2"))
		ctx.Refresh()

		var obj *VarInterfaceObj
		err := ctx.GetObject(&obj)

		assert.Nil(t, err)
		assert.Equal(t, len(obj.v), 2)
	})
}

func TestApplicationContext_Close(t *testing.T) {

	t.Run("destroy type", func(t *testing.T) {

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Destroy(func() {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Destroy(func() int { return 0 })
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Destroy(func(int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Destroy(func(int, int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")
	})

	t.Run("call destroy fn", func(t *testing.T) {
		called := false

		ctx := gs.New()
		ctx.Object(new(int)).Destroy(func(i *int) { called = true })
		ctx.Refresh()
		ctx.Close()

		assert.True(t, called)
	})

	t.Run("call destroy", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(new(callDestroy)).Destroy((*callDestroy).Destroy)
		ctx.Refresh()

		var d *callDestroy
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.destroyed)
	})

	t.Run("call destroy with arg", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("version", "v0.0.1")
		ctx.Object(new(callDestroy)).Destroy((*callDestroy).DestroyWithArg, "${version}")
		ctx.Refresh()

		var d *callDestroy
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.destroyed)
	})

	t.Run("call destroy with error", func(t *testing.T) {

		// error
		{
			ctx := gs.New()
			ctx.SetProperty("int", 1)
			ctx.Object(new(callDestroy)).Destroy((*callDestroy).DestroyWithError, "${int}")
			ctx.Refresh()

			var d *callDestroy
			err := ctx.GetObject(&d)

			ctx.Close()

			assert.Nil(t, err)
			assert.False(t, d.destroyed)
		}

		// nil
		{
			ctx := gs.New()
			ctx.SetProperty("int", 0)
			ctx.Object(new(callDestroy)).Destroy((*callDestroy).DestroyWithError, "${int}")
			ctx.Refresh()

			var d *callDestroy
			err := ctx.GetObject(&d)

			ctx.Close()

			assert.Nil(t, err)
			assert.True(t, d.destroyed)
		}
	})

	t.Run("call interface destroy", func(t *testing.T) {

		ctx := gs.New()
		ctx.Factory(func() destroyable { return new(callDestroy) }).Destroy(destroyable.Destroy)
		ctx.Refresh()

		var d destroyable
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.(*callDestroy).destroyed)
	})

	t.Run("call interface destroy with arg", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("version", "v0.0.1")
		ctx.Factory(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithArg, "${version}")
		ctx.Refresh()

		var d destroyable
		err := ctx.GetObject(&d)

		ctx.Close()

		assert.Nil(t, err)
		assert.True(t, d.(*callDestroy).destroyed)
	})

	t.Run("call interface destroy with error", func(t *testing.T) {

		// error
		{
			ctx := gs.New()
			ctx.SetProperty("int", 1)
			ctx.Factory(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithError, "${int}")
			ctx.Refresh()

			var d destroyable
			err := ctx.GetObject(&d)

			ctx.Close()

			assert.Nil(t, err)
			assert.False(t, d.(*callDestroy).destroyed)
		}

		// nil
		{
			ctx := gs.New()
			ctx.SetProperty("int", 0)
			ctx.Factory(func() destroyable { return new(callDestroy) }).Destroy(destroyable.DestroyWithError, "${int}")
			ctx.Refresh()

			var d destroyable
			err := ctx.GetObject(&d)

			ctx.Close()

			assert.Nil(t, err)
			assert.True(t, d.(*callDestroy).destroyed)
		}
	})

	t.Run("context done", func(t *testing.T) {
		var wg sync.WaitGroup
		ctx := gs.New()
		ctx.Object(new(int)).Init(func(i *int) {
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
		ctx.Refresh()
		ctx.Close()
		wg.Wait()
	})
}

func TestApplicationContext_BeanNotFound(t *testing.T) {
	assert.Panic(t, func() {
		ctx := gs.New()
		ctx.Factory(func(i *int) bool { return false }, "")
		ctx.Refresh()
	}, "can't find bean, bean:\"\" field:\"\" type:\"\\*int\"")
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

	ctx := gs.New()
	ctx.Factory(func() int { return 3 })
	ctx.Object(new(NestedAutowireBean))
	ctx.Object(&PtrNestedAutowireBean{
		SubNestedAutowireBean: new(SubNestedAutowireBean),
	})
	ctx.Object(new(FieldNestedAutowireBean))
	ctx.Object(&PtrFieldNestedAutowireBean{
		B: new(SubNestedAutowireBean),
	})
	ctx.Refresh()

	var b *NestedAutowireBean
	err := ctx.GetObject(&b)

	assert.Nil(t, err)
	assert.Equal(t, *b.Int, 3)

	var b0 *PtrNestedAutowireBean
	err = ctx.GetObject(&b0)

	assert.Nil(t, err)
	assert.Equal(t, b0.Int, (*int)(nil))

	var b1 *FieldNestedAutowireBean
	err = ctx.GetObject(&b1)

	assert.Nil(t, err)
	assert.Equal(t, *b1.B.Int, 3)

	var b2 *PtrFieldNestedAutowireBean
	err = ctx.GetObject(&b2)

	assert.Nil(t, err)
	assert.Equal(t, b2.B.Int, (*int)(nil))
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

		ctx := gs.New()
		ctx.SetProperty("sdk.wx.auto-create", true)
		ctx.SetProperty("sdk.wx.enable", true)

		bd := ctx.Factory(func() int { return 3 })
		assert.Equal(t, bd.BeanName(), "*int")

		ctx.Object(new(wxChannel))
		ctx.Refresh()

		var c *wxChannel
		err := ctx.GetObject(&c)

		assert.Nil(t, err)
		assert.Equal(t, *c.baseChannel.Int, 3)
		assert.Equal(t, *c.int, 3)
		assert.Equal(t, c.baseChannel.Int, c.int)
		assert.Equal(t, c.enable, true)
		assert.Equal(t, c.AutoCreate, true)
	})

	t.Run("public", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("sdk.wx.auto-create", true)
		ctx.SetProperty("sdk.wx.enable", true)
		ctx.Factory(func() int { return 3 })
		ctx.Object(new(WXChannel))
		ctx.Refresh()

		var c *WXChannel
		err := ctx.GetObject(&c)

		assert.Nil(t, err)
		assert.Equal(t, *c.BaseChannel.Int, 3)
		assert.Equal(t, *c.Int, 3)
		assert.Equal(t, c.BaseChannel.Int, c.Int)
		assert.True(t, c.Enable)
		assert.True(t, c.AutoCreate)
	})
}

func TestApplicationContext_FnArgCollectBean(t *testing.T) {

	t.Run("base type", func(t *testing.T) {
		ctx := gs.New()
		ctx.Factory(func() int { return 3 }).WithName("i1")
		ctx.Factory(func() int { return 4 }).WithName("i2")
		ctx.Factory(func(i []*int) bool {
			nums := make([]int, 0)
			for _, e := range i {
				nums = append(nums, *e)
			}
			sort.Ints(nums)
			assert.Equal(t, nums, []int{3, 4})
			return false
		}, "[]")
		ctx.Refresh()
	})

	t.Run("interface type", func(t *testing.T) {
		ctx := gs.New()
		ctx.Factory(newHistoryTeacher("t1")).WithName("t1").Export((*Teacher)(nil))
		ctx.Factory(newHistoryTeacher("t2")).WithName("t2").Export((*Teacher)(nil))
		ctx.Factory(func(teachers []Teacher) bool {
			names := make([]string, 0)
			for _, teacher := range teachers {
				names = append(names, teacher.(*historyTeacher).name)
			}
			sort.Strings(names)
			assert.Equal(t, names, []string{"t1", "t2"})
			return false
		}, "[]")
		ctx.Refresh()
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
		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(new(int)).Export((*filter)(nil))
			ctx.Refresh()
		}, "not implement gs_test.filter interface")
	})

	t.Run("implement interface", func(t *testing.T) {

		var server struct {
			F1 filter `autowire:"f1"`
			F2 filter `autowire:"f2"`
		}

		ctx := gs.New()
		ctx.Factory(func() filter { return new(filterImpl) }).WithName("f1")
		ctx.Object(new(filterImpl)).Export((*filter)(nil)).WithName("f2")
		ctx.Object(&server)

		ctx.Refresh()
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
	ctx := gs.New()
	ctx.Factory(func() IntInterface { return Integer(5) })
	ctx.Refresh()
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

		ctx := gs.New()
		ctx.Object(b)
		ctx.Refresh()

		var x context.Context
		err := ctx.GetObject(&x)
		assert.Nil(t, err)
		assert.Equal(t, x, b)

		var s fmt.Stringer
		err = ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s, b)

		var pbi ptrBaseInterface
		err = ctx.GetObject(&pbi)
		assert.Nil(t, err)
		assert.Equal(t, pbi, b)

		var bi baseInterface
		err = ctx.GetObject(&bi)
		assert.Nil(t, err)
		assert.Equal(t, bi, b)
	})

	t.Run("auto export private", func(t *testing.T) {

		ctx := gs.New()
		ctx.Factory(pkg2.NewAppContext)
		ctx.Refresh()

		var x context.Context
		err := ctx.GetObject(&x)
		assert.Nil(t, err)

		var s fmt.Stringer
		err = ctx.GetObject(&s)
		assert.Nil(t, err)
	})

	t.Run("auto export & export", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}

		ctx := gs.New()
		ctx.Object(b).Export((*fmt.Stringer)(nil))
		ctx.Refresh()

		var x context.Context
		err := ctx.GetObject(&x)
		assert.Nil(t, err)
		assert.Equal(t, x, b)

		var s fmt.Stringer
		err = ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s, b)
	})

	t.Run("unexported but auto match", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}
		ctx := gs.New()
		ctx.Object(&struct {
			Error error `autowire:"e"`
		}{})
		ctx.Object(b).WithName("e")
		ctx.Refresh()
	})

	t.Run("export and match corerectly", func(t *testing.T) {
		b := &AppContext{Context: context.TODO()}
		ctx := gs.New()
		ctx.Object(&struct {
			Error error `autowire:"e"`
		}{})
		ctx.Object(b).WithName("e").Export((*error)(nil))
		ctx.Refresh()
	})

	t.Run("panics", func(t *testing.T) {

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(&struct {
				_ *int `export:""`
			}{})
			ctx.Refresh()
		}, "export can only use on interface")

		assert.Panic(t, func() {
			ctx := gs.New()
			ctx.Object(&struct {
				_ Runner `export:"" autowire:""`
			}{})
			ctx.Refresh()
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
		ctx := gs.New()
		ctx.Object(b)
		ctx.Refresh()
		assert.Equal(t, b.Duration, []time.Duration{time.Second, 5 * time.Second})
	})

	t.Run("map default value ", func(t *testing.T) {

		obj := struct {
			Int  int               `value:"${int:=5}"`
			IntA int               `value:"${int_a:=5}"`
			Map  map[string]string `value:"${map:={}}"`
			MapA map[string]string `value:"${map_a:={}}"`
		}{}

		ctx := gs.New()
		ctx.SetProperty("map_a.nba", "nba")
		ctx.SetProperty("map_a.cba", "cba")
		ctx.SetProperty("int_a", "3")
		ctx.SetProperty("int_b", "4")
		ctx.Object(&obj)
		ctx.Refresh()

		assert.Equal(t, obj.Int, 5)
		assert.Equal(t, obj.IntA, 3)
		assert.Equal(t, obj.Map, map[string]string{})
		assert.Equal(t, obj.MapA, map[string]string{
			"cba": "cba",
			"nba": "nba",
		})
	})
}

func TestFnStringBincorengArg(t *testing.T) {
	ctx := gs.New()
	ctx.Factory(func(i *int) bool {
		fmt.Printf("i=%d\n", *i)
		return false
	}, "${key.name:=*int}")
	i := 5
	ctx.Object(&i)
	ctx.Refresh()
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

	ctx := gs.New()
	ctx.Object(new(FirstDestroy)).Destroy(
		func(_ *FirstDestroy) {
			fmt.Println("::FirstDestroy")
			destroyArray[destroyIndex] = 1
			destroyIndex++
		})
	ctx.Object(new(Second2Destroy)).Destroy(
		func(_ *Second2Destroy) {
			fmt.Println("::Second2Destroy")
			destroyArray[destroyIndex] = 2
			destroyIndex++
		})
	ctx.Object(new(Second1Destroy)).Destroy(
		func(_ *Second1Destroy) {
			fmt.Println("::Second1Destroy")
			destroyArray[destroyIndex] = 2
			destroyIndex++
		})
	ctx.Object(new(ThirdDestroy)).Destroy(
		func(_ *ThirdDestroy) {
			fmt.Println("::ThirdDestroy")
			destroyArray[destroyIndex] = 4
			destroyIndex++
		})
	ctx.Refresh()
	ctx.Close()

	assert.Equal(t, destroyArray, []int{1, 2, 2, 4})
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

	assert.Panic(t, func() {
		ctx := gs.New()
		ctx.Object(DefaultRegistry)
		ctx.Factory(NewRegistry)
		ctx.Refresh()
	}, `duplicate registration, bean:`)

	assert.Panic(t, func() {
		ctx := gs.New()
		bd := ctx.Object(&registryFactory{})
		ctx.Factory(func(f *registryFactory) Registry { return f.Create() }, bd)
		ctx.Factory(NewRegistryInterface)
		ctx.Refresh()
	}, `duplicate registration, bean:`)
}

type Obj struct {
	i int
}

type ObjFactory struct{}

func (factory *ObjFactory) NewObj(i int) *Obj { return &Obj{i: i} }

func TestApplicationContext_CreateBean(t *testing.T) {

	ctx := gs.New()
	ctx.Object(&ObjFactory{})
	ctx.Refresh()

	b, err := ctx.WireBean((*ObjFactory).NewObj, arg.R2("${i:=5}"))
	fmt.Println(b, err)
}

func TestDefaultSpringContext(t *testing.T) {

	t.Run("bean:test_ctx:", func(t *testing.T) {

		ctx := gs.New()
		ctx.Object(&BeanZero{5}).WithCond(cond.
			OnProfile("test").
			And().
			OnMissingBean("null"),
		)

		ctx.Refresh()

		var b *BeanZero
		err := ctx.GetObject(&b)
		assert.Error(t, err, "can't find bean, bean:\"\"")
	})

	t.Run("bean:test_ctx:test", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProfile("test")
		ctx.Object(&BeanZero{5}).WithCond(cond.OnProfile("test"))
		ctx.Refresh()

		var b *BeanZero
		err := ctx.GetObject(&b)
		assert.Nil(t, err)
	})

	t.Run("bean:test_ctx:stable", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProfile("stable")
		ctx.Object(&BeanZero{5}).WithCond(cond.OnProfile("test"))
		ctx.Refresh()

		var b *BeanZero
		err := ctx.GetObject(&b)
		assert.Error(t, err, "can't find bean, bean:\"\"")
	})

	t.Run("option withClassName Condition", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.SetProperty("class_floor", 2)
		ctx.Factory(NewClassRoom, arg.Option(withClassName,
			"${class_name:=二年级03班}",
			"${class_floor:=3}",
		).WithCond(cond.OnProperty("class_name_enable")))
		ctx.Refresh()

		var cls *ClassRoom
		err := ctx.GetObject(&cls)

		assert.Nil(t, err)
		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName Apply", func(t *testing.T) {
		c := cond.OnProperty("class_name_enable")

		ctx := gs.New()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.Factory(NewClassRoom,
			arg.Option(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}",
			).WithCond(c),
		)
		ctx.Refresh()

		var cls *ClassRoom
		err := ctx.GetObject(&cls)

		assert.Nil(t, err)
		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("method bean cond", func(t *testing.T) {

		ctx := gs.New()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.Object(new(Server))
		ctx.Factory((*Server).Consumer, parent.BeanId()).WithCond(cond.OnProperty("consumer.enable"))
		ctx.Refresh()

		var s *Server
		err := ctx.GetObject(&s)
		assert.Nil(t, err)
		assert.Equal(t, s.Version, "1.0.0")

		var c *Consumer
		err = ctx.GetObject(&c)
		assert.Error(t, err, "can't find bean, bean:\"\"")
	})
}

// TODO 现在的方式父 Bean 不存在子 Bean 创建的时候会报错
//func TestDefaultSpringContext_ParentNotRegister(t *testing.T) {
//
//	ctx := core.New()
//	parent := ctx.Factory(NewServerInterface).WithCond(cond.OnProperty("server.is.nil"))
//	ctx.Factory(ServerInterface.Consumer, parent.BeanId())
//
//	ctx.Refresh()
//
//	var s *Server
//	ok := ctx.GetObject(&s)
//	util.Equal(t, ok, false)
//
//	var c *Consumer
//	ok = ctx.GetObject(&c)
//	util.Equal(t, ok, false)
//}

func TestDefaultSpringContext_ConditionOnBean(t *testing.T) {
	ctx := gs.New()

	c := cond.
		OnMissingProperty("Null").
		Or().
		OnProfile("test")

	ctx.Object(&BeanZero{5}).WithCond(cond.
		On(c).
		And().
		OnMissingBean("null"),
	)

	ctx.Object(new(BeanOne)).WithCond(cond.
		On(c).
		And().
		OnMissingBean("null"),
	)

	ctx.Object(new(BeanTwo)).WithCond(cond.OnBean("*gs_test.BeanOne"))
	ctx.Object(new(BeanTwo)).WithName("another_two").WithCond(cond.OnBean("Null"))

	ctx.Refresh()

	var two *BeanTwo
	err := ctx.GetObject(&two, "")
	assert.Nil(t, err)

	err = ctx.GetObject(&two, "another_two")
	assert.Error(t, err, "can't find bean, bean:\"another_two\"")
}

func TestDefaultSpringContext_ConditionOnMissingBean(t *testing.T) {

	for i := 0; i < 20; i++ { // 测试 FindBean 无需绑定，不要排序
		ctx := gs.New()

		ctx.Object(&BeanZero{5})
		ctx.Object(new(BeanOne))

		ctx.Object(new(BeanTwo)).WithCond(cond.OnMissingBean("*gs_test.BeanOne"))
		ctx.Object(new(BeanTwo)).WithName("another_two").WithCond(cond.OnMissingBean("Null"))

		ctx.Refresh()

		var two *BeanTwo
		err := ctx.GetObject(&two, "")
		assert.Nil(t, err)

		err = ctx.GetObject(&two, "another_two")
		assert.Nil(t, err)
	}
}

func TestFunctionCondition(t *testing.T) {
	ctx := gs.New()

	fn := func(ctx cond.Context) bool { return true }
	c := cond.OnMatches(fn)
	assert.True(t, c.Matches(ctx))

	fn = func(ctx cond.Context) bool { return false }
	c = cond.OnMatches(fn)
	assert.False(t, c.Matches(ctx))
}

func TestPropertyCondition(t *testing.T) {

	ctx := gs.New()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	c := cond.OnProperty("int")
	assert.True(t, c.Matches(ctx))

	c = cond.OnProperty("bool")
	assert.False(t, c.Matches(ctx))

	c = cond.OnProperty("parent")
	assert.True(t, c.Matches(ctx))

	c = cond.OnProperty("parent123")
	assert.False(t, c.Matches(ctx))
}

func TestMissingPropertyCondition(t *testing.T) {

	ctx := gs.New()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	c := cond.OnMissingProperty("int")
	assert.False(t, c.Matches(ctx))

	c = cond.OnMissingProperty("bool")
	assert.True(t, c.Matches(ctx))

	c = cond.OnMissingProperty("parent")
	assert.False(t, c.Matches(ctx))

	c = cond.OnMissingProperty("parent123")
	assert.True(t, c.Matches(ctx))
}

func TestPropertyValueCondition(t *testing.T) {

	ctx := gs.New()
	ctx.SetProperty("str", "this is a str")
	ctx.SetProperty("int", 3)

	c := cond.OnPropertyValue("int", 3)
	assert.True(t, c.Matches(ctx))

	c = cond.OnPropertyValue("int", "3")
	assert.False(t, c.Matches(ctx))

	c = cond.OnPropertyValue("int", "$>2&&$<4")
	assert.True(t, c.Matches(ctx))

	c = cond.OnPropertyValue("bool", true)
	assert.False(t, c.Matches(ctx))

	c = cond.OnPropertyValue("str", "\"$\"==\"this is a str\"")
	assert.True(t, c.Matches(ctx))
}

func TestBeanCondition(t *testing.T) {

	ctx := gs.New()
	ctx.Object(&BeanZero{5})
	ctx.Object(new(BeanOne))
	ctx.Refresh()

	c := cond.OnBean("*gs_test.BeanOne")
	assert.True(t, c.Matches(ctx))

	c = cond.OnBean("Null")
	assert.False(t, c.Matches(ctx))
}

func TestMissingBeanCondition(t *testing.T) {

	ctx := gs.New()
	ctx.Object(&BeanZero{5})
	ctx.Object(new(BeanOne))
	ctx.Refresh()

	c := cond.OnMissingBean("*gs_test.BeanOne")
	assert.False(t, c.Matches(ctx))

	c = cond.OnMissingBean("Null")
	assert.True(t, c.Matches(ctx))
}

func TestExpressionCondition(t *testing.T) {

}

func TestConditional(t *testing.T) {

	ctx := gs.New()
	ctx.SetProperty("bool", false)
	ctx.SetProperty("int", 3)
	ctx.Refresh()

	c := cond.OnProperty("int")
	assert.True(t, c.Matches(ctx))

	c = cond.OnProperty("int").OnBean("null")
	assert.False(t, c.Matches(ctx))

	assert.Panic(t, func() {
		c = cond.OnProperty("int").And()
		assert.Equal(t, c.Matches(ctx), true)
	}, "no condition in last node")

	c = cond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", false)
	assert.True(t, c.Matches(ctx))

	c = cond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", true)
	assert.False(t, c.Matches(ctx))

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", true)
	assert.False(t, c.Matches(ctx))

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false)
	assert.True(t, c.Matches(ctx))

	assert.Panic(t, func() {
		c = cond.OnPropertyValue("int", 2).
			Or().
			OnPropertyValue("bool", false).
			Or()
		assert.Equal(t, c.Matches(ctx), true)
	}, "no condition in last node")

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false).
		OnPropertyValue("bool", false)
	assert.True(t, c.Matches(ctx))
}

func TestNotCondition(t *testing.T) {

	ctx := gs.New()
	ctx.SetProfile("test")
	ctx.Refresh()

	profileCond := cond.OnProfile("test")
	assert.True(t, profileCond.Matches(ctx))

	notCond := cond.Not(profileCond)
	assert.False(t, notCond.Matches(ctx))

	c := cond.OnPropertyValue("int", 2).
		And().
		On(cond.Not(profileCond))
	assert.False(t, c.Matches(ctx))

	c = cond.OnProfile("test").
		And().
		On(cond.Not(profileCond))
	assert.False(t, c.Matches(ctx))
}

func TestApplicationContext_Invoke(t *testing.T) {

	t.Run("before Refresh", func(t *testing.T) {

		ctx := gs.New()
		ctx.Factory(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")

		assert.Panic(t, func() {
			_ = ctx.Invoke(func(i *int, version string) {
				fmt.Println("version:", version)
				fmt.Println("int:", *i)
			}, "", "${version}")
		}, "should call after Refresh")

		ctx.Refresh()
	})

	t.Run("not run", func(t *testing.T) {

		ctx := gs.New()
		ctx.Factory(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.Refresh()

		_ = ctx.Invoke(func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
		}, "", "${version}")
	})

	t.Run("run", func(t *testing.T) {

		ctx := gs.New()
		ctx.Factory(func() int { return 3 })
		ctx.SetProperty("version", "v0.0.1")
		ctx.SetProfile("dev")
		ctx.Refresh()

		fn := func(i *int, version string) {
			fmt.Println("version:", version)
			fmt.Println("int:", *i)
		}

		_ = ctx.Invoke(fn, "", "${version}")
	})
}

func init() {
	log.SetLevel(log.TraceLevel)
}
