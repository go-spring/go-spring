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

package gs_test

import (
	"errors"
	"fmt"
	"image"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-spring/spring-boost/assert"
	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-boost/conf"
	"github.com/go-spring/spring-boost/json"
	"github.com/go-spring/spring-boost/log"
	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/gs"
	"github.com/go-spring/spring-core/gs/arg"
	"github.com/go-spring/spring-core/gs/cond"
	"github.com/go-spring/spring-core/gs/internal"
	pkg1 "github.com/go-spring/spring-core/gs/testdata/pkg/bar"
	pkg2 "github.com/go-spring/spring-core/gs/testdata/pkg/foo"
)

func init() {
	log.SetLevel(log.TraceLevel)
}

func container(fn func(gs.Environment)) gs.Container {
	c := gs.New()
	type PandoraAware struct{}
	c.Provide(func(p gs.Environment) PandoraAware {
		fn(p)
		return PandoraAware{}
	})
	return c
}

func TestApplicationContext_RegisterBeanFrozen(t *testing.T) {
	assert.Panic(t, func() {
		c := gs.New()
		c.Object(new(int)).Init(func(i *int) {
			// 不能在这里注册新的 Object
			c.Object(new(bool))
		})
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	}, "should call before Refresh")
}

func TestApplicationContext(t *testing.T) {

	t.Run("int", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			{
				var i int
				err := p.BeanRegistry().Get(&i)
				assert.Error(t, err, "int is not valid receiver type")
			}

			// 找到多个符合条件的值
			{
				var i *int
				err := p.BeanRegistry().Get(&i)
				assert.Error(t, err, "found 3 beans, bean:\"\" type:\"\\*int\" ")
			}

			// 入参不是可赋值的对象
			{
				var i int
				err := p.BeanRegistry().Get(&i, "i3")
				assert.Error(t, err, "int is not valid receiver type")
			}

			{
				var i *int
				err := p.BeanRegistry().Get(&i, "i3")
				assert.Nil(t, err)
			}

			{
				var i []int
				err := p.BeanRegistry().Get(&i)
				assert.NotNil(t, err)
			}

			{
				var i *[]int
				err := p.BeanRegistry().Get(&i)
				assert.NotNil(t, err)
			}
		})

		e := int(3)

		// 普通类型用属性注入
		assert.Panic(t, func() {
			c.Object(e)
		}, "bean must be ref type")

		c.Object(&e)

		// 相同类型不同名称的 bean 都可注册
		c.Object(&e).Name("i3")

		// 相同类型不同名称的 bean 都可注册
		c.Object(&e).Name("i4")

		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	/////////////////////////////////////////
	// 自定义数据类型

	t.Run("pkg1.SamePkg", func(t *testing.T) {
		c := gs.New()
		e := pkg1.SamePkg{}

		assert.Panic(t, func() {
			c.Object(e)
		}, "bean must be ref type")

		c.Object(&e)
		c.Object(&e).Name("i3")
		c.Object(&e).Name("i4")

		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("pkg2.SamePkg", func(t *testing.T) {
		c := gs.New()
		e := pkg2.SamePkg{}

		assert.Panic(t, func() {
			c.Object(e)
		}, "bean must be ref type")

		c.Object(&e)
		c.Object(&e).Name("i3")
		c.Object(&e).Name("i4")
		c.Object(&e).Name("i5")

		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
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
	IntPtrByType *int `inject:"?"`
	IntPtrByName *int `autowire:"${key_1:=int_ptr}?"`

	// 基础类型指针数组
	IntPtrSliceByType []*int `inject:"?"`
	IntPtrCollection  []*int `autowire:"${key_2:=[int_ptr]}?"`
	IntPtrSliceByName []*int `autowire:"int_ptr_slice?"`

	// 自定义类型指针
	StructByType *TestBincoreng `inject:"?"`
	StructByName *TestBincoreng `autowire:"struct_ptr?"`

	// 自定义类型指针数组
	StructPtrSliceByType []*TestBincoreng `inject:"?"`
	StructPtrCollection  []*TestBincoreng `autowire:"?"`
	StructPtrSliceByName []*TestBincoreng `autowire:"struct_ptr_slice?"`

	// 接口
	InterfaceByType fmt.Stringer `inject:"?"`
	InterfaceByName fmt.Stringer `autowire:"struct_ptr?"`

	// 接口数组
	InterfaceSliceByType []fmt.Stringer `autowire:"?"`

	InterfaceCollection  []fmt.Stringer `inject:"?"`
	InterfaceCollection2 []fmt.Stringer `autowire:"?"`

	// 指定名称时使用精确匹配模式，不对数组元素进行转换，即便能做到似乎也无意义
	InterfaceSliceByName []fmt.Stringer `autowire:"struct_ptr_slice?"`

	MapTyType map[string]interface{} `inject:"?"`
	MapByName map[string]interface{} `autowire:"map?"`
}

func TestApplicationContext_AutoWireBeans(t *testing.T) {

	t.Run("wired error", func(t *testing.T) {
		c := gs.New()

		obj := &TestObject{}
		c.Object(obj)

		i := int(3)
		c.Object(&i).Name("int_ptr")

		i2 := int(3)
		c.Object(&i2).Name("int_ptr_2")

		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "\"TestObject.IntPtrByType\" wired error: found 2 beans, bean:\"\\?\" type:\"\\*int\"")
	})

	f1 := float32(11.0)
	f2 := float32(12.0)

	c := container(func(p gs.Environment) {

		var ff []*float32
		err := p.BeanRegistry().Get(&ff, "float_ptr_2", "float_ptr_1")
		assert.Nil(t, err)
		assert.Equal(t, ff, []*float32{&f2, &f1})
	})

	obj := &TestObject{}
	c.Object(obj)

	i := int(3)
	c.Object(&i).Name("int_ptr")

	b := TestBincoreng{1}
	c.Object(&b).Name("struct_ptr").Export((*fmt.Stringer)(nil))

	c.Object(&f1).Name("float_ptr_1")
	c.Object(&f2).Name("float_ptr_2")

	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)

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
	c := gs.New()

	c.Property("int", int(3))
	c.Property("uint", uint(3))
	c.Property("float", float32(3))
	c.Property("complex", complex(3, 0))
	c.Property("string", "3")
	c.Property("bool", true)

	setting := &Setting{}
	c.Object(setting)

	c.Property("sub.int", int(4))
	c.Property("sub.sub.int", int(5))
	c.Property("sub_sub.int", int(6))

	c.Property("int_slice", []int{1, 2})
	c.Property("string_slice", []string{"1", "2"})
	// c.Property("float_slice", []float64{1, 2})

	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)

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
	container gs.Environment `autowire:""`
}

func (f *PrototypeBeanFactory) New(name string) *PrototypeBean {
	b := &PrototypeBean{
		name: name,
		t:    time.Now(),
	}

	// PrototypeBean 依赖的服务可以通过 Context 注入
	_, err := f.container.BeanRegistry().Wire(b)
	util.Panic(err).When(err != nil)
	return b
}

type PrototypeBeanService struct {
	Provide *PrototypeBeanFactory `autowire:""`
}

func (s *PrototypeBeanService) Service(name string) {
	// 通过 PrototypeBean 的工厂获取新的实例，并且每个实例都有自己的时间戳
	fmt.Println(s.Provide.New(name).Greeting())
}

func TestApplicationContext_PrototypeBean(t *testing.T) {
	c := container(func(p gs.Environment) {})

	greetingService := &GreetingService{}
	c.Object(greetingService)

	s := &PrototypeBeanService{}
	c.Object(s)

	f := &PrototypeBeanFactory{}
	c.Object(f)

	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)

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
	c := gs.New()
	{
		p, _ := conf.Load("testdata/config/application.yaml")
		for _, key := range p.Keys() {
			c.Property(key, p.Get(key))
		}
	}

	b := &EnvEnumBean{}
	c.Object(b)

	c.Property("env.type", "test")

	p := &PointBean{}
	c.Object(p)

	conf.Convert(PointConverter)
	c.Property("point", "(7,5)")

	dbConfig := &DbConfig{}
	c.Object(dbConfig)

	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)

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
	c := gs.New()
	c.Object(new(MyGrouper)).Export((*Grouper)(nil))
	c.Object(new(ProxyGrouper))
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

type Pkg interface {
	Package()
}

type SamePkgHolder struct {
	// Pkg `autowire:""` // 这种方式会找到多个符合条件的 Object
	Pkg `autowire:"github.com/go-spring/spring-core/gs/testdata/pkg/bar/pkg.SamePkg:SamePkg"`
}

func TestApplicationContext_SameNameBean(t *testing.T) {
	c := gs.New()
	c.Object(new(SamePkgHolder))
	c.Object(&pkg1.SamePkg{}).Export((*Pkg)(nil))
	c.Object(&pkg2.SamePkg{}).Export((*Pkg)(nil))
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
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
	// Pkg `autowire:"same"` // 如果两个 Object 不小心重名了，也会找到多个符合条件的 Object
	Pkg `autowire:"github.com/go-spring/spring-core/gs_test/gs_test.DiffPkgTwo:same"`
}

func TestApplicationContext_DiffNameBean(t *testing.T) {
	c := gs.New()
	c.Object(&DiffPkgOne{}).Name("same").Export((*Pkg)(nil))
	c.Object(&DiffPkgTwo{}).Name("same").Export((*Pkg)(nil))
	c.Object(new(DiffPkgHolder))
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

func TestApplicationContext_LoadProperties(t *testing.T) {

	c := container(func(e gs.Environment) {
		p := e.Properties()
		assert.Equal(t, p.Get("yaml.list[0]"), "1")
		assert.Equal(t, p.Get("yaml.list[1]"), "2")
		assert.Equal(t, p.Get("spring.application.name"), "test")
	})

	p, _ := conf.Load("testdata/config/application.yaml")
	for _, key := range p.Keys() {
		c.Property(key, p.Get(key))
	}

	p, _ = conf.Load("testdata/config/application.properties")
	for _, key := range p.Keys() {
		c.Property(key, p.Get(key))
	}

	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

func TestApplicationContext_Get(t *testing.T) {

	t.Run("panic", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			{
				var i int
				err := p.BeanRegistry().Get(i)
				assert.Error(t, err, "i must be pointer")
			}

			{
				var i *int
				err := p.BeanRegistry().Get(i)
				assert.Error(t, err, "receiver must be ref type")
			}

			{
				i := new(int)
				err := p.BeanRegistry().Get(i)
				assert.Error(t, err, "int is not valid receiver type")
			}

			{
				var i *int
				err := p.BeanRegistry().Get(&i)
				assert.Error(t, err, "can't find bean, bean:\"\"")
			}

			{
				var s fmt.Stringer
				err := p.BeanRegistry().Get(s)
				assert.Error(t, err, "i can't be nil")
			}

			{
				var s fmt.Stringer
				err := p.BeanRegistry().Get(&s)
				assert.Error(t, err, "can't find bean, bean:\"\"")
			}
		})
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("success", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var two *BeanTwo
			err := p.BeanRegistry().Get(&two)
			assert.Nil(t, err)

			var grouper Grouper
			err = p.BeanRegistry().Get(&grouper)
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&two, (*BeanTwo)(nil))
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&grouper, (*BeanTwo)(nil))
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&two)
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&grouper)
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&two, "BeanTwo")
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&grouper, "BeanTwo")
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&two, ":BeanTwo")
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&grouper, ":BeanTwo")
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&two, "github.com/go-spring/spring-core/gs_test/gs_test.BeanTwo:BeanTwo")
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&grouper, "github.com/go-spring/spring-core/gs_test/gs_test.BeanTwo:BeanTwo")
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&two, "xxx:BeanTwo")
			assert.Error(t, err, "can't find bean, bean:\"xxx:BeanTwo\"")

			err = p.BeanRegistry().Get(&grouper, "xxx:BeanTwo")
			assert.Error(t, err, "can't find bean, bean:\"xxx:BeanTwo\"")

			var three *BeanThree
			err = p.BeanRegistry().Get(&three)
			assert.Error(t, err, "can't find bean, bean:\"\"")
		})
		c.Object(&BeanZero{5})
		c.Object(new(BeanOne))
		c.Object(new(BeanTwo)).Export((*Grouper)(nil))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

//func TestApplicationContext_FindByName(t *testing.T) {
//
//	c := container(func(p gs.Environment) {})
//	c.Object(&BeanZero{5})
//	c.Object(new(BeanOne))
//	c.Object(new(BeanTwo))
//	c.Refresh(internal.AutoClear(false))
//
//	p := <-ch
//
//	b, _ := p.Find("")
//	assert.Equal(t, len(b), 4)
//
//	b, _ = p.Find("BeanTwo")
//	fmt.Println(json.ToString(b))
//	assert.Equal(t, len(b), 0)
//
//	b, _ = p.Find("BeanTwo")
//	fmt.Println(json.ToString(b))
//	assert.Equal(t, len(b), 1)
//
//	b, _ = p.Find(":BeanTwo")
//	fmt.Println(json.ToString(b))
//	assert.Equal(t, len(b), 1)
//
//	b, _ = p.Find("github.com/go-spring/spring-core/gs_test/gs_test.BeanTwo:BeanTwo")
//	fmt.Println(json.ToString(b))
//	assert.Equal(t, len(b), 1)
//
//	b, _ = p.Find("xxx:BeanTwo")
//	fmt.Println(json.ToString(b))
//	assert.Equal(t, len(b), 0)
//
//	b, _ = p.Find((*BeanTwo)(nil))
//	fmt.Println(json.ToString(b))
//	assert.Equal(t, len(b), 1)
//
//	b, _ = p.Find((*fmt.Stringer)(nil))
//	assert.Equal(t, len(b), 0)
//
//	b, _ = p.Find((*Grouper)(nil))
//	assert.Equal(t, len(b), 0)
//}

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
	c := container(func(p gs.Environment) {

		var st1 *Student
		err := p.BeanRegistry().Get(&st1, "st1")

		assert.Nil(t, err)
		fmt.Println(json.ToString(st1))
		assert.Equal(t, st1.Room, p.Properties().Get("room"))

		var st2 *Student
		err = p.BeanRegistry().Get(&st2, "st2")

		assert.Nil(t, err)
		fmt.Println(json.ToString(st2))
		assert.Equal(t, st2.Room, p.Properties().Get("room"))

		fmt.Printf("%x\n", reflect.ValueOf(st1).Pointer())
		fmt.Printf("%x\n", reflect.ValueOf(st2).Pointer())

		var st3 *Student
		err = p.BeanRegistry().Get(&st3, "st3")

		assert.Nil(t, err)
		fmt.Println(json.ToString(st3))
		assert.Equal(t, st3.Room, p.Properties().Get("room"))

		var st4 *Student
		err = p.BeanRegistry().Get(&st4, "st4")

		assert.Nil(t, err)
		fmt.Println(json.ToString(st4))
		assert.Equal(t, st4.Room, p.Properties().Get("room"))
	})

	c.Property("room", "Class 3 Grade 1")

	// 用接口注册时实际使用的是原始类型
	c.Object(Teacher(newHistoryTeacher(""))).Export((*Teacher)(nil))

	c.Provide(NewStudent, "", "${room}").Name("st1")
	c.Provide(NewPtrStudent, "", "${room}").Name("st2")
	c.Provide(NewStudent, "?", "${room:=https://}").Name("st3")
	c.Provide(NewPtrStudent, "?", "${room:=4567}").Name("st4")

	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

func TestApplicationContext_Profile(t *testing.T) {

	t.Run("bean:_c:", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var b *BeanZero
			err := p.BeanRegistry().Get(&b)
			assert.Nil(t, err)
		})
		c.Object(&BeanZero{5})
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("bean:_c:test", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var b *BeanZero
			err := p.BeanRegistry().Get(&b)
			assert.Nil(t, err)
		})
		c.Property("spring.profiles.active", "test")
		c.Object(&BeanZero{5})
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

type BeanFour struct{}

func TestApplicationContext_DependsOn(t *testing.T) {

	t.Run("random", func(t *testing.T) {
		c := gs.New()
		c.Object(&BeanZero{5})
		c.Object(new(BeanOne))
		c.Object(new(BeanFour))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("dependsOn", func(t *testing.T) {

		dependsOn := []gs.BeanSelector{
			(*BeanOne)(nil), // 通过类型定义查找
			"github.com/go-spring/spring-core/gs_test/gs_test.BeanZero:BeanZero",
		}

		c := gs.New()
		c.Object(&BeanZero{5})
		c.Object(new(BeanOne))
		c.Object(new(BeanFour)).DependsOn(dependsOn...)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

func TestApplicationContext_Primary(t *testing.T) {

	t.Run("duplicate", func(t *testing.T) {
		c := gs.New()
		c.Object(&BeanZero{5})
		c.Object(&BeanZero{6})
		c.Object(new(BeanOne))
		c.Object(new(BeanTwo))
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "duplicate beans ")
	})

	t.Run("duplicate", func(t *testing.T) {
		c := gs.New()
		c.Object(&BeanZero{5})
		// primary 是在多个候选 bean 里面选择，而不是允许同名同类型的两个 bean
		c.Object(&BeanZero{6}).Primary()
		c.Object(new(BeanOne))
		c.Object(new(BeanTwo))
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "duplicate beans ")
	})

	t.Run("not primary", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var b *BeanTwo
			err := p.BeanRegistry().Get(&b)
			assert.Nil(t, err)
			assert.Equal(t, b.One.Zero.Int, 5)
		})
		c.Object(&BeanZero{5})
		c.Object(new(BeanOne))
		c.Object(new(BeanTwo))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("primary", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var b *BeanTwo
			err := p.BeanRegistry().Get(&b)
			assert.Nil(t, err)
			assert.Equal(t, b.One.Zero.Int, 6)
		})
		c.Object(&BeanZero{5})
		c.Object(&BeanZero{6}).Name("zero_6").Primary()
		c.Object(new(BeanOne))
		c.Object(new(BeanTwo))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

type FuncObj struct {
	Fn func(int) int `autowire:""`
}

func TestDefaultProperties_WireFunc(t *testing.T) {
	c := gs.New()
	c.Object(func(int) int { return 6 })
	obj := new(FuncObj)
	c.Object(obj)
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
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
		c := container(func(p gs.Environment) {

			var m Manager
			err := p.BeanRegistry().Get(&m)
			assert.Nil(t, err)

			// 因为用户是按照接口注册的，所以理论上在依赖
			// 系统中用户并不关心接口对应的真实类型是什么。
			var lm *localManager
			err = p.BeanRegistry().Get(&lm)
			assert.Error(t, err, "can't find bean, bean:\"\"")
		})
		c.Property("manager.version", "1.0.0")
		c.Provide(NewPtrManager)
		c.Provide(NewInt)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("manager", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var m Manager
			err := p.BeanRegistry().Get(&m)
			assert.Nil(t, err)

			var lm *localManager
			err = p.BeanRegistry().Get(&lm)
			assert.Error(t, err, "can't find bean, bean:\"\"")
		})
		c.Property("manager.version", "1.0.0")

		bd := c.Provide(NewManager)
		assert.Equal(t, bd.BeanName(), "Manager")

		bd = c.Provide(NewInt)
		assert.Equal(t, bd.BeanName(), "int")

		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("manager return error", func(t *testing.T) {
		c := gs.New()
		c.Property("manager.version", "1.0.0")
		c.Provide(NewManagerRetError)
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "return error")
	})

	t.Run("manager return error nil", func(t *testing.T) {
		c := gs.New()
		c.Property("manager.version", "1.0.0")
		c.Provide(NewManagerRetErrorNil)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("manager return nil", func(t *testing.T) {
		c := gs.New()
		c.Property("manager.version", "1.0.0")
		c.Provide(NewNullPtrManager)
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "return nil")
	})
}

type destroyable interface {
	Init()
	Destroy()
	InitWithError() error
	DestroyWithError() error
}

type callDestroy struct {
	i         int
	inited    bool
	destroyed bool
}

func (d *callDestroy) Init() {
	d.inited = true
}

func (d *callDestroy) Destroy() {
	d.destroyed = true
}

func (d *callDestroy) InitWithError() error {
	if d.i == 0 {
		d.inited = true
		return nil
	}
	return errors.New("error")
}

func (d *callDestroy) DestroyWithError() error {
	if d.i == 0 {
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
			c := gs.New()
			c.Object(new(int)).Init(func() {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			c := gs.New()
			c.Object(new(int)).Init(func() int { return 0 })
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			c := gs.New()
			c.Object(new(int)).Init(func(int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			c := gs.New()
			c.Object(new(int)).Init(func(int, int) {})
		}, "init should be func\\(bean\\) or func\\(bean\\)error")

		c := container(func(p gs.Environment) {
			var i *int
			err := p.BeanRegistry().Get(&i)
			assert.Nil(t, err)
			assert.Equal(t, *i, 3)
		})
		c.Object(new(int)).Init(func(i *int) { *i = 3 })
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("call init", func(t *testing.T) {

		c := container(func(p gs.Environment) {
			var d *callDestroy
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
			assert.True(t, d.inited)
		})
		c.Object(new(callDestroy)).Init((*callDestroy).Init)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("call init with error", func(t *testing.T) {

		{
			c := gs.New()
			c.Object(&callDestroy{i: 1}).Init((*callDestroy).InitWithError)
			err := c.Refresh(internal.AutoClear(false))
			assert.Error(t, err, "error")
		}

		c := container(func(p gs.Environment) {
			var d *callDestroy
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
			assert.True(t, d.inited)
		})
		c.Property("int", 0)
		c.Object(&callDestroy{}).Init((*callDestroy).InitWithError)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("call interface init", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var d destroyable
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
			assert.True(t, d.(*callDestroy).inited)
		})
		c.Provide(func() destroyable { return new(callDestroy) }).Init(destroyable.Init)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("call interface init with error", func(t *testing.T) {

		{
			c := gs.New()
			c.Provide(func() destroyable { return &callDestroy{i: 1} }).Init(destroyable.InitWithError)
			err := c.Refresh(internal.AutoClear(false))
			assert.Error(t, err, "error")
		}

		c := container(func(p gs.Environment) {
			var d destroyable
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
			assert.True(t, d.(*callDestroy).inited)
		})
		c.Property("int", 0)
		c.Provide(func() destroyable { return &callDestroy{} }).Init(destroyable.InitWithError)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("call nested init", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var d *nestedCallDestroy
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
			assert.True(t, d.inited)
		})
		c.Object(new(nestedCallDestroy)).Init((*nestedCallDestroy).Init)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("call nested interface init", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var d *nestedDestroyable
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
			assert.True(t, d.destroyable.(*callDestroy).inited)
		})
		c.Object(&nestedDestroyable{
			destroyable: new(callDestroy),
		}).Init((*nestedDestroyable).Init)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

type RecoresCluster struct {
	Endpoints string `value:"${redis.endpoints}"`
}

func TestApplicationContext_ValueBincoreng(t *testing.T) {
	c := container(func(p gs.Environment) {
		var cluster *RecoresCluster
		err := p.BeanRegistry().Get(&cluster)
		fmt.Println(cluster)
		assert.Nil(t, err)
	})
	c.Property("redis.endpoints", "redis://localhost:6379")
	c.Object(new(RecoresCluster))
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

func TestApplicationContext_Collect(t *testing.T) {

	t.Run("", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var rcs []*RecoresCluster
			err := p.BeanRegistry().Get(&rcs)
			assert.Nil(t, err)
			assert.Equal(t, len(rcs), 2)
		})
		c.Property("redis.endpoints", "redis://localhost:6379")
		c.Object(new(RecoresCluster)).Name("one")
		c.Object(new(RecoresCluster))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("", func(t *testing.T) {

		c := container(func(p gs.Environment) {})
		c.Property("redis.endpoints", "redis://localhost:6379")
		c.Object(new(RecoresCluster)).Name("a").Order(1)
		c.Object(new(RecoresCluster)).Name("b").Order(2)

		intBean := c.Provide(func(p gs.Environment) *int {

			var rcs []*RecoresCluster
			err := p.BeanRegistry().Get(&rcs)
			fmt.Println(json.ToString(rcs))

			assert.Nil(t, err)
			assert.Equal(t, len(rcs), 2)
			assert.Equal(t, rcs[0].Endpoints, "redis://localhost:6379")

			return new(int)
		})
		assert.Equal(t, intBean.BeanName(), "int")

		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
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
		c := container(func(p gs.Environment) {
			var cls *ClassRoom
			err := p.BeanRegistry().Get(&cls)
			assert.Nil(t, err)
			assert.Equal(t, len(cls.students), 0)
			assert.Equal(t, cls.className, "default")
			assert.Equal(t, cls.President, "CaiYuanPei")
		})
		c.Property("president", "CaiYuanPei")
		c.Provide(NewClassRoom)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("option withClassName", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var cls *ClassRoom
			err := p.BeanRegistry().Get(&cls)
			assert.Nil(t, err)
			assert.Equal(t, cls.floor, 3)
			assert.Equal(t, len(cls.students), 0)
			assert.Equal(t, cls.className, "二年级03班")
			assert.Equal(t, cls.President, "CaiYuanPei")
		})
		c.Property("president", "CaiYuanPei")
		c.Provide(NewClassRoom, arg.Option(withClassName, "${class_name:=二年级03班}", "${class_floor:=3}"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("option withStudents", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var cls *ClassRoom
			err := p.BeanRegistry().Get(&cls)
			assert.Nil(t, err)
			assert.Equal(t, cls.floor, 0)
			assert.Equal(t, len(cls.students), 2)
			assert.Equal(t, cls.className, "default")
			assert.Equal(t, cls.President, "CaiYuanPei")
		})
		c.Property("class_name", "二年级03班")
		c.Property("president", "CaiYuanPei")
		c.Provide(NewClassRoom, arg.Option(withStudents))
		c.Object(new(Student)).Name("Student1")
		c.Object(new(Student)).Name("Student2")
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("option withStudents withClassName", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var cls *ClassRoom
			err := p.BeanRegistry().Get(&cls)
			assert.Nil(t, err)
			assert.Equal(t, cls.floor, 3)
			assert.Equal(t, len(cls.students), 2)
			assert.Equal(t, cls.className, "二年级06班")
			assert.Equal(t, cls.President, "CaiYuanPei")
		})
		c.Property("class_name", "二年级06班")
		c.Property("president", "CaiYuanPei")
		c.Provide(NewClassRoom,
			arg.Option(withStudents),
			arg.Option(withClassName, "${class_name:=二年级03班}", "${class_floor:=3}"),
		)
		c.Object(&Student{}).Name("Student1")
		c.Object(&Student{}).Name("Student2")
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
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
		c := container(func(p gs.Environment) {

			var s *Server
			err := p.BeanRegistry().Get(&s)
			assert.Nil(t, err)
			assert.Equal(t, s.Version, "1.0.0")

			s.Version = "2.0.0"

			var consumer *Consumer
			err = p.BeanRegistry().Get(&consumer)
			assert.Nil(t, err)
			assert.Equal(t, consumer.s.Version, "2.0.0")
		})
		c.Property("server.version", "1.0.0")
		parent := c.Object(new(Server))
		bd := c.Provide((*Server).Consumer, parent.ID())
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
		assert.Equal(t, bd.BeanName(), "Consumer")
	})

	t.Run("method bean arg", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var s *Server
			err := p.BeanRegistry().Get(&s)
			assert.Nil(t, err)
			assert.Equal(t, s.Version, "1.0.0")

			s.Version = "2.0.0"

			var consumer *Consumer
			err = p.BeanRegistry().Get(&consumer)
			assert.Nil(t, err)
			assert.Equal(t, consumer.s.Version, "2.0.0")
		})
		c.Property("server.version", "1.0.0")
		parent := c.Object(new(Server))
		c.Provide((*Server).ConsumerArg, parent.ID(), "${i:=9}")
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("method bean wire to other bean", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var si ServerInterface
			err := p.BeanRegistry().Get(&si)
			assert.Nil(t, err)

			s := si.(*Server)
			assert.Equal(t, s.Version, "1.0.0")

			s.Version = "2.0.0"

			var consumer *Consumer
			err = p.BeanRegistry().Get(&consumer)
			assert.Nil(t, err)
			assert.Equal(t, consumer.s.Version, "2.0.0")
		})
		c.Property("server.version", "1.0.0")
		parent := c.Provide(NewServerInterface)
		c.Provide(ServerInterface.Consumer, parent.ID()).DependsOn("ServerInterface")
		c.Object(new(Service))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
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

				c := gs.New()
				c.Property("server.version", "1.0.0")
				parent := c.Object(new(Server)).DependsOn("Service")
				c.Provide((*Server).Consumer, parent.ID()).DependsOn("Server")
				c.Object(new(Service))
				err := c.Refresh(internal.AutoClear(false))
				util.Panic(err).When(err != nil)
			}()
		}
		fmt.Printf("ok:%d err:%d\n", okCount, errCount)
	})

	t.Run("method bean autowire", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var s *Server
			err := p.BeanRegistry().Get(&s)
			assert.Nil(t, err)
			assert.Equal(t, s.Version, "1.0.0")
		})
		c.Property("server.version", "1.0.0")
		c.Object(new(Server))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("method bean selector type", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var s *Server
			err := p.BeanRegistry().Get(&s)
			assert.Nil(t, err)
			assert.Equal(t, s.Version, "1.0.0")

			s.Version = "2.0.0"

			var consumer *Consumer
			err = p.BeanRegistry().Get(&consumer)
			assert.Nil(t, err)
			assert.Equal(t, consumer.s.Version, "2.0.0")
		})
		c.Property("server.version", "1.0.0")
		c.Object(new(Server))
		c.Provide(func(s *Server) *Consumer { return s.Consumer() }, (*Server)(nil))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("method bean selector type error", func(t *testing.T) {
		c := gs.New()
		c.Property("server.version", "1.0.0")
		c.Object(new(Server))
		c.Provide(func(s *Server) *Consumer { return s.Consumer() }, (*int)(nil))
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "can't find bean, bean:\"int:\" type:\"\\*gs_test.Server\"")
	})

	t.Run("method bean selector beanId", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var s *Server
			err := p.BeanRegistry().Get(&s)
			assert.Nil(t, err)
			assert.Equal(t, s.Version, "1.0.0")

			s.Version = "2.0.0"

			var consumer *Consumer
			err = p.BeanRegistry().Get(&consumer)
			assert.Nil(t, err)
			assert.Equal(t, consumer.s.Version, "2.0.0")
		})
		c.Property("server.version", "1.0.0")
		c.Object(new(Server))
		c.Provide(func(s *Server) *Consumer { return s.Consumer() }, "Server")
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("method bean selector beanId error", func(t *testing.T) {
		c := gs.New()
		c.Property("server.version", "1.0.0")
		c.Object(new(Server))
		c.Provide(func(s *Server) *Consumer { return s.Consumer() }, "NULL")
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "can't find bean, bean:\"NULL\" type:\"\\*gs_test.Server\"")
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

	c := gs.New()

	conf.Convert(func(v string) (level, error) {
		if v == "debug" {
			return 1, nil
		}
		return 0, errors.New("error level")
	})

	c.Property("time", "2018-12-20")
	c.Property("duration", "1h")
	c.Property("level", "debug")
	c.Property("complex", "1+i")
	c.Object(&config)
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)

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
	// 直接创建的 Object 直接发生循环依赖是没有关系的。
	c := gs.New()
	c.Object(new(CircleA))
	c.Object(new(CircleB))
	c.Object(new(CircleC))
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
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
		c := container(func(p gs.Environment) {
			var obj *VarObj
			err := p.BeanRegistry().Get(&obj)
			assert.Nil(t, err)
			assert.Equal(t, len(obj.v), 1)
			assert.Equal(t, obj.v[0].name, "v1")
			assert.Equal(t, obj.s, "description")
		})
		c.Property("var.obj", "description")
		c.Object(&Var{"v1"}).Name("v1")
		c.Object(&Var{"v2"}).Name("v2")
		c.Provide(NewVarObj, "${var.obj}", arg.Option(withVar, "v1"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("variable option param 2", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var obj *VarObj
			err := p.BeanRegistry().Get(&obj)
			assert.Nil(t, err)
			assert.Equal(t, len(obj.v), 2)
			assert.Equal(t, obj.v[0].name, "v1")
			assert.Equal(t, obj.v[1].name, "v2")
			assert.Equal(t, obj.s, "description")
		})
		c.Property("var.obj", "description")
		c.Object(&Var{"v1"}).Name("v1")
		c.Object(&Var{"v2"}).Name("v2")
		c.Provide(NewVarObj, arg.Value("description"), arg.Option(withVar, "v1", "v2"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("variable option interface param 1", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var obj *VarInterfaceObj
			err := p.BeanRegistry().Get(&obj)
			assert.Nil(t, err)
			assert.Equal(t, len(obj.v), 1)
		})
		c.Object(&Var{"v1"}).Name("v1").Export((*interface{})(nil))
		c.Object(&Var{"v2"}).Name("v2").Export((*interface{})(nil))
		c.Provide(NewVarInterfaceObj, arg.Option(withVarInterface, "v1"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("variable option interface param 1", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var obj *VarInterfaceObj
			err := p.BeanRegistry().Get(&obj)
			assert.Nil(t, err)
			assert.Equal(t, len(obj.v), 2)
		})
		c.Object(&Var{"v1"}).Name("v1").Export((*interface{})(nil))
		c.Object(&Var{"v2"}).Name("v2").Export((*interface{})(nil))
		c.Provide(NewVarInterfaceObj, arg.Option(withVarInterface, "v1", "v2"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

func TestApplicationContext_Close(t *testing.T) {

	t.Run("destroy type", func(t *testing.T) {

		assert.Panic(t, func() {
			c := gs.New()
			c.Object(new(int)).Destroy(func() {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			c := gs.New()
			c.Object(new(int)).Destroy(func() int { return 0 })
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			c := gs.New()
			c.Object(new(int)).Destroy(func(int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")

		assert.Panic(t, func() {
			c := gs.New()
			c.Object(new(int)).Destroy(func(int, int) {})
		}, "destroy should be func\\(bean\\) or func\\(bean\\)error")
	})

	t.Run("call destroy fn", func(t *testing.T) {
		called := false

		c := gs.New()
		c.Object(new(int)).Destroy(func(i *int) { called = true })
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
		c.Close()

		assert.True(t, called)
	})

	t.Run("call destroy", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var d *callDestroy
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
		})
		d := new(callDestroy)
		c.Object(d).Destroy((*callDestroy).Destroy)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
		c.Close()
		assert.True(t, d.destroyed)
	})

	t.Run("call destroy with error", func(t *testing.T) {

		// error
		{
			c := container(func(p gs.Environment) {
				var d *callDestroy
				err := p.BeanRegistry().Get(&d)
				assert.Nil(t, err)
			})
			d := &callDestroy{i: 1}
			c.Object(d).Destroy((*callDestroy).DestroyWithError)
			err := c.Refresh(internal.AutoClear(false))
			assert.Nil(t, err)
			c.Close()
			assert.False(t, d.destroyed)
		}

		// nil
		{
			c := container(func(p gs.Environment) {
				var d *callDestroy
				err := p.BeanRegistry().Get(&d)
				assert.Nil(t, err)
			})
			d := &callDestroy{}
			c.Object(d).Destroy((*callDestroy).DestroyWithError)
			err := c.Refresh(internal.AutoClear(false))
			assert.Nil(t, err)
			c.Close()
			assert.True(t, d.destroyed)
		}
	})

	t.Run("call interface destroy", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var d destroyable
			err := p.BeanRegistry().Get(&d)
			assert.Nil(t, err)
		})
		bd := c.Provide(func() destroyable { return new(callDestroy) }).Destroy(destroyable.Destroy)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
		c.Close()
		d := bd.Interface().(*callDestroy)
		assert.True(t, d.destroyed)
	})

	t.Run("call interface destroy with error", func(t *testing.T) {

		// error
		{
			c := container(func(p gs.Environment) {
				var d destroyable
				err := p.BeanRegistry().Get(&d)
				assert.Nil(t, err)
			})
			bd := c.Provide(func() destroyable { return &callDestroy{i: 1} }).Destroy(destroyable.DestroyWithError)
			err := c.Refresh(internal.AutoClear(false))
			assert.Nil(t, err)
			c.Close()
			d := bd.Interface()
			assert.False(t, d.(*callDestroy).destroyed)
		}

		// nil
		{
			c := container(func(p gs.Environment) {
				var d destroyable
				err := p.BeanRegistry().Get(&d)
				assert.Nil(t, err)
			})
			c.Property("int", 0)
			bd := c.Provide(func() destroyable { return &callDestroy{} }).Destroy(destroyable.DestroyWithError)
			err := c.Refresh(internal.AutoClear(false))
			assert.Nil(t, err)
			c.Close()
			d := bd.Interface()
			assert.True(t, d.(*callDestroy).destroyed)
		}
	})
}

func TestApplicationContext_BeanNotFound(t *testing.T) {
	c := gs.New()
	c.Provide(func(i *int) bool { return false }, "")
	err := c.Refresh(internal.AutoClear(false))
	assert.Error(t, err, "can't find bean, bean:\"\" type:\"\\*int\"")
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

func TestApplicationContext_NestedAutowireBean(t *testing.T) {
	c := container(func(p gs.Environment) {

		var b *NestedAutowireBean
		err := p.BeanRegistry().Get(&b)
		assert.Nil(t, err)
		assert.Equal(t, *b.Int, 3)

		var b0 *PtrNestedAutowireBean
		err = p.BeanRegistry().Get(&b0)
		assert.Nil(t, err)
		assert.Equal(t, b0.Int, (*int)(nil))
	})
	c.Provide(func() int { return 3 })
	c.Object(new(NestedAutowireBean))
	c.Object(&PtrNestedAutowireBean{
		SubNestedAutowireBean: new(SubNestedAutowireBean),
	})
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
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

	// nolint 支持对私有字段注入，但是不推荐！代码扫描请忽略这行。
	enable bool `value:"${enable:=false}"`
}

type wxChannel struct {
	baseChannel `value:"${sdk.wx}"`

	// 支持对私有字段注入，但是不推荐！代码扫描请忽略这行。
	int *int `autowire:""`
}

func TestApplicationContext_NestValueField(t *testing.T) {

	t.Run("private", func(t *testing.T) {

		c := container(func(p gs.Environment) {
			var channel *wxChannel
			err := p.BeanRegistry().Get(&channel)
			assert.Nil(t, err)
			assert.Equal(t, *channel.baseChannel.Int, 3)
			assert.Equal(t, *channel.int, 3)
			assert.Equal(t, channel.baseChannel.Int, channel.int)
			assert.Equal(t, channel.enable, true)
			assert.Equal(t, channel.AutoCreate, true)
		})

		c.Property("sdk.wx.auto-create", true)
		c.Property("sdk.wx.enable", true)

		bd := c.Provide(func() int { return 3 })
		assert.Equal(t, bd.BeanName(), "int")

		c.Object(new(wxChannel))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("public", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var channel *WXChannel
			err := p.BeanRegistry().Get(&channel)
			assert.Nil(t, err)
			assert.Equal(t, *channel.BaseChannel.Int, 3)
			assert.Equal(t, *channel.Int, 3)
			assert.Equal(t, channel.BaseChannel.Int, channel.Int)
			assert.True(t, channel.Enable)
			assert.True(t, channel.AutoCreate)
		})
		c.Property("sdk.wx.auto-create", true)
		c.Property("sdk.wx.enable", true)
		c.Provide(func() int { return 3 })
		c.Object(new(WXChannel))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

func TestApplicationContext_FnArgCollectBean(t *testing.T) {

	t.Run("base type", func(t *testing.T) {
		c := gs.New()
		c.Provide(func() int { return 3 }).Name("i1")
		c.Provide(func() int { return 4 }).Name("i2")
		c.Provide(func(i []*int) bool {
			nums := make([]int, 0)
			for _, e := range i {
				nums = append(nums, *e)
			}
			sort.Ints(nums)
			assert.Equal(t, nums, []int{3, 4})
			return false
		})
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("interface type", func(t *testing.T) {
		c := gs.New()
		c.Provide(newHistoryTeacher("t1")).Name("t1").Export((*Teacher)(nil))
		c.Provide(newHistoryTeacher("t2")).Name("t2").Export((*Teacher)(nil))
		c.Provide(func(teachers []Teacher) bool {
			names := make([]string, 0)
			for _, teacher := range teachers {
				names = append(names, teacher.(*historyTeacher).name)
			}
			sort.Strings(names)
			assert.Equal(t, names, []string{"t1", "t2"})
			return false
		})
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
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
		c := gs.New()
		c.Object(new(int)).Export((*filter)(nil))
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "doesn't implement interface gs_test.filter")
	})

	t.Run("implement interface", func(t *testing.T) {

		var server struct {
			F1 filter `autowire:"f1"`
			F2 filter `autowire:"f2"`
		}

		c := gs.New()
		c.Provide(func() filter { return new(filterImpl) }).Name("f1")
		c.Object(new(filterImpl)).Export((*filter)(nil)).Name("f2")
		c.Object(&server)

		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
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
	c := gs.New()
	c.Provide(func() IntInterface { return Integer(5) })
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

type ArrayProperties struct {
	Int      []int           `value:"${int.array:=}"`
	Int8     []int8          `value:"${int8.array:=}"`
	Int16    []int16         `value:"${int16.array:=}"`
	Int32    []int32         `value:"${int32.array:=}"`
	Int64    []int64         `value:"${int64.array:=}"`
	UInt     []uint          `value:"${uint.array:=}"`
	UInt8    []uint8         `value:"${uint8.array:=}"`
	UInt16   []uint16        `value:"${uint16.array:=}"`
	UInt32   []uint32        `value:"${uint32.array:=}"`
	UInt64   []uint64        `value:"${uint64.array:=}"`
	String   []string        `value:"${string.array:=}"`
	Bool     []bool          `value:"${bool.array:=}"`
	Duration []time.Duration `value:"${duration.array:=}"`
	Time     []time.Time     `value:"${time.array:=}"`
}

func TestApplicationContext_Properties(t *testing.T) {

	t.Run("array properties", func(t *testing.T) {
		b := new(ArrayProperties)
		c := gs.New()
		c.Object(b)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("map default value ", func(t *testing.T) {

		obj := struct {
			Int  int               `value:"${int:=5}"`
			IntA int               `value:"${int_a:=5}"`
			Map  map[string]string `value:"${map:=}"`
			MapA map[string]string `value:"${map_a:=}"`
		}{}

		c := gs.New()
		c.Property("map_a.nba", "nba")
		c.Property("map_a.cba", "cba")
		c.Property("int_a", "3")
		c.Property("int_b", "4")
		c.Object(&obj)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)

		assert.Equal(t, obj.Int, 5)
		assert.Equal(t, obj.IntA, 3)
	})
}

func TestFnStringBincorengArg(t *testing.T) {
	c := gs.New()
	c.Provide(func(i *int) bool {
		fmt.Printf("i=%d\n", *i)
		return false
	}, "${key.name:=int}")
	i := 5
	c.Object(&i)
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
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

	c := gs.New()
	c.Object(new(FirstDestroy)).Destroy(
		func(_ *FirstDestroy) {
			fmt.Println("::FirstDestroy")
			destroyArray[destroyIndex] = 1
			destroyIndex++
		})
	c.Object(new(Second2Destroy)).Destroy(
		func(_ *Second2Destroy) {
			fmt.Println("::Second2Destroy")
			destroyArray[destroyIndex] = 2
			destroyIndex++
		})
	c.Object(new(Second1Destroy)).Destroy(
		func(_ *Second1Destroy) {
			fmt.Println("::Second1Destroy")
			destroyArray[destroyIndex] = 2
			destroyIndex++
		})
	c.Object(new(ThirdDestroy)).Destroy(
		func(_ *ThirdDestroy) {
			fmt.Println("::ThirdDestroy")
			destroyArray[destroyIndex] = 4
			destroyIndex++
		})
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
	c.Close()

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

	t.Run("", func(t *testing.T) {
		c := gs.New()
		c.Object(DefaultRegistry)
		c.Provide(NewRegistry)
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "duplicate beans")
	})

	t.Run("", func(t *testing.T) {
		c := gs.New()
		bd := c.Object(&registryFactory{})
		c.Provide(func(f *registryFactory) Registry { return f.Create() }, bd)
		c.Provide(NewRegistryInterface)
		err := c.Refresh(internal.AutoClear(false))
		assert.Error(t, err, "duplicate beans")
	})
}

type Obj struct {
	i int
}

type ObjFactory struct{}

func (factory *ObjFactory) NewObj(i int) *Obj { return &Obj{i: i} }

func TestApplicationContext_CreateBean(t *testing.T) {
	c := container(func(p gs.Environment) {
		b, err := p.BeanRegistry().Wire((*ObjFactory).NewObj, arg.R1("${i:=5}"))
		fmt.Println(b, err)
	})
	c.Object(&ObjFactory{})
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

func TestDefaultSpringContext(t *testing.T) {

	t.Run("bean:test_ctx:", func(t *testing.T) {

		c := container(func(p gs.Environment) {
			var b *BeanZero
			err := p.BeanRegistry().Get(&b)
			assert.Error(t, err, "can't find bean, bean:\"\"")
		})

		c.Object(&BeanZero{5}).On(cond.
			OnProfile("test").
			And().
			OnMissingBean("null"),
		)

		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("bean:test_ctx:test", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var b *BeanZero
			err := p.BeanRegistry().Get(&b)
			assert.Nil(t, err)
		})
		c.Property("spring.profiles.active", "test")
		c.Object(&BeanZero{5}).On(cond.OnProfile("test"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("bean:test_ctx:stable", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			var b *BeanZero
			err := p.BeanRegistry().Get(&b)
			assert.Error(t, err, "can't find bean, bean:\"\"")
		})
		c.Property("spring.profiles.active", "stable")
		c.Object(&BeanZero{5}).On(cond.OnProfile("test"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("option withClassName Condition", func(t *testing.T) {

		c := container(func(p gs.Environment) {
			var cls *ClassRoom
			err := p.BeanRegistry().Get(&cls)
			assert.Nil(t, err)
			assert.Equal(t, cls.floor, 0)
			assert.Equal(t, len(cls.students), 0)
			assert.Equal(t, cls.className, "default")
			assert.Equal(t, cls.President, "CaiYuanPei")
		})
		c.Property("president", "CaiYuanPei")
		c.Property("class_floor", 2)
		c.Provide(NewClassRoom, arg.Option(withClassName,
			"${class_name:=二年级03班}",
			"${class_floor:=3}",
		).On(cond.OnProperty("class_name_enable")))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("option withClassName Apply", func(t *testing.T) {
		onProperty := cond.OnProperty("class_name_enable")
		c := container(func(p gs.Environment) {
			var cls *ClassRoom
			err := p.BeanRegistry().Get(&cls)
			assert.Nil(t, err)
			assert.Equal(t, cls.floor, 0)
			assert.Equal(t, len(cls.students), 0)
			assert.Equal(t, cls.className, "default")
			assert.Equal(t, cls.President, "CaiYuanPei")
		})
		c.Property("president", "CaiYuanPei")
		c.Provide(NewClassRoom,
			arg.Option(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}",
			).On(onProperty),
		)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("method bean cond", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var s *Server
			err := p.BeanRegistry().Get(&s)
			assert.Nil(t, err)
			assert.Equal(t, s.Version, "1.0.0")

			var consumer *Consumer
			err = p.BeanRegistry().Get(&consumer)
			assert.Error(t, err, "can't find bean, bean:\"\"")
		})
		c.Property("server.version", "1.0.0")
		parent := c.Object(new(Server))
		c.Provide((*Server).Consumer, parent.ID()).On(cond.OnProperty("consumer.enable"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

// TODO 现在的方式父 Bean 不存在子 Bean 创建的时候会报错
//func TestDefaultSpringContext_ParentNotRegister(t *testing.T) {
//
//	c := gs.New()
//	parent := c.Provide(NewServerInterface).On(cond.OnProperty("server.is.nil"))
//	c.Provide(ServerInterface.Consumer, parent.ID())
//
//	c.Refresh(internal.AutoClear(false))
//
//	var s *Server
//	ok := p.BeanRegistry().Get(&s)
//	util.Equal(t, ok, false)
//
//	var c *Consumer
//	ok = p.BeanRegistry().Get(&c)
//	util.Equal(t, ok, false)
//}

func TestDefaultSpringContext_ConditionOnBean(t *testing.T) {
	c := container(func(p gs.Environment) {

		var two *BeanTwo
		err := p.BeanRegistry().Get(&two)
		assert.Nil(t, err)

		err = p.BeanRegistry().Get(&two, "another_two")
		assert.Error(t, err, "can't find bean, bean:\"another_two\"")
	})

	c1 := cond.OnProperty("null", cond.MatchIfMissing()).Or().OnProfile("test")

	c.Object(&BeanZero{5}).On(cond.On(c1).And().OnMissingBean("null"))
	c.Object(new(BeanOne)).On(cond.On(c1).And().OnMissingBean("null"))

	c.Object(new(BeanTwo)).On(cond.OnBean("BeanOne"))
	c.Object(new(BeanTwo)).Name("another_two").On(cond.OnBean("Null"))

	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)
}

func TestDefaultSpringContext_ConditionOnMissingBean(t *testing.T) {
	for i := 0; i < 20; i++ { // 测试 Find 无需绑定，不要排序
		c := container(func(p gs.Environment) {

			var two *BeanTwo
			err := p.BeanRegistry().Get(&two)
			assert.Nil(t, err)

			err = p.BeanRegistry().Get(&two, "another_two")
			assert.Nil(t, err)
		})
		c.Object(&BeanZero{5})
		c.Object(new(BeanOne))
		c.Object(new(BeanTwo)).On(cond.OnMissingBean("BeanOne"))
		c.Object(new(BeanTwo)).Name("another_two").On(cond.OnMissingBean("Null"))
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	}
}

//func TestFunctionCondition(t *testing.T) {
//	c := gs.New()
//
//	fn := func(c cond.Context) bool { return true }
//	c1 := cond.OnMatches(fn)
//	assert.True(t, c1.Matches(c))
//
//	fn = func(c cond.Context) bool { return false }
//	c2 := cond.OnMatches(fn)
//	assert.False(t, c2.Matches(c))
//}
//
//func TestPropertyCondition(t *testing.T) {
//
//	c := gs.New()
//	c.Property("int", 3)
//	c.Property("parent.child", 0)
//
//	c1 := cond.OnProperty("int")
//	assert.True(t, c1.Matches(c))
//
//	c2 := cond.OnProperty("bool")
//	assert.False(t, c2.Matches(c))
//
//	c3 := cond.OnProperty("parent")
//	assert.True(t, c3.Matches(c))
//
//	c4 := cond.OnProperty("parent123")
//	assert.False(t, c4.Matches(c))
//}
//
//func TestMissingPropertyCondition(t *testing.T) {
//
//	c := gs.New()
//	c.Property("int", 3)
//	c.Property("parent.child", 0)
//
//	c1 := cond.OnMissingProperty("int")
//	assert.False(t, c1.Matches(c))
//
//	c2 := cond.OnMissingProperty("bool")
//	assert.True(t, c2.Matches(c))
//
//	c3 := cond.OnMissingProperty("parent")
//	assert.False(t, c3.Matches(c))
//
//	c4 := cond.OnMissingProperty("parent123")
//	assert.True(t, c4.Matches(c))
//}
//
//func TestPropertyValueCondition(t *testing.T) {
//
//	c := gs.New()
//	c.Property("str", "this is a str")
//	c.Property("int", 3)
//
//	c1 := cond.OnPropertyValue("int", 3)
//	assert.True(t, c1.Matches(c))
//
//	c2 := cond.OnPropertyValue("int", "3")
//	assert.False(t, c2.Matches(c))
//
//	c3 := cond.OnPropertyValue("int", "$>2&&$<4")
//	assert.True(t, c3.Matches(c))
//
//	c4 := cond.OnPropertyValue("bool", true)
//	assert.False(t, c4.Matches(c))
//
//	c5 := cond.OnPropertyValue("str", "\"$\"==\"this is a str\"")
//	assert.True(t, c5.Matches(c))
//}
//
//func TestBeanCondition(t *testing.T) {
//
//	c := gs.New()
//	c.Object(&BeanZero{5})
//	c.Object(new(BeanOne))
//	c.Refresh(internal.AutoClear(false))
//
//	c1 := cond.OnBean("BeanOne")
//	assert.True(t, c1.Matches(c))
//
//	c2 := cond.OnBean("Null")
//	assert.False(t, c2.Matches(c))
//}
//
//func TestMissingBeanCondition(t *testing.T) {
//
//	c := gs.New()
//	c.Object(&BeanZero{5})
//	c.Object(new(BeanOne))
//	c.Refresh(internal.AutoClear(false))
//
//	c1 := cond.OnMissingBean("BeanOne")
//	assert.False(t, c1.Matches(c))
//
//	c2 := cond.OnMissingBean("Null")
//	assert.True(t, c2.Matches(c))
//}
//
//func TestExpressionCondition(t *testing.T) {
//
//}
//
//func TestConditional(t *testing.T) {
//
//	c := gs.New()
//	c.Property("bool", false)
//	c.Property("int", 3)
//	c.Refresh(internal.AutoClear(false))
//
//	c1 := cond.OnProperty("int")
//	assert.True(t, c1.Matches(c))
//
//	c2 := cond.OnProperty("int").OnBean("null")
//	assert.False(t, c2.Matches(c))
//
//	assert.Panic(t, func() {
//		c3 := cond.OnProperty("int").And()
//		assert.Equal(t, c3.Matches(c), true)
//	}, "no condition in last node")
//
//	c4 := cond.OnPropertyValue("int", 3).
//		And().
//		OnPropertyValue("bool", false)
//	assert.True(t, c4.Matches(c))
//
//	c5 := cond.OnPropertyValue("int", 3).
//		And().
//		OnPropertyValue("bool", true)
//	assert.False(t, c5.Matches(c))
//
//	c6 := cond.OnPropertyValue("int", 2).
//		Or().
//		OnPropertyValue("bool", true)
//	assert.False(t, c6.Matches(c))
//
//	c7 := cond.OnPropertyValue("int", 2).
//		Or().
//		OnPropertyValue("bool", false)
//	assert.True(t, c7.Matches(c))
//
//	assert.Panic(t, func() {
//		c8 := cond.OnPropertyValue("int", 2).
//			Or().
//			OnPropertyValue("bool", false).
//			Or()
//		assert.Equal(t, c8.Matches(c), true)
//	}, "no condition in last node")
//
//	c9 := cond.OnPropertyValue("int", 2).
//		Or().
//		OnPropertyValue("bool", false).
//		OnPropertyValue("bool", false)
//	assert.True(t, c9.Matches(c))
//}
//
//func TestNotCondition(t *testing.T) {
//
//	c := gs.New()
//	c.Property(environ.SpringProfilesActive, "test")
//	c.Refresh(internal.AutoClear(false))
//
//	profileCond := cond.OnProfile("test")
//	assert.True(t, profileCond.Matches(c))
//
//	notCond := cond.Not(profileCond)
//	assert.False(t, notCond.Matches(c))
//
//	c1 := cond.OnPropertyValue("int", 2).
//		And().
//		On(cond.Not(profileCond))
//	assert.False(t, c1.Matches(c))
//
//	c2 := cond.OnProfile("test").
//		And().
//		On(cond.Not(profileCond))
//	assert.False(t, c2.Matches(c))
//}

func TestApplicationContext_Invoke(t *testing.T) {

	t.Run("not run", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			_, _ = p.BeanRegistry().Invoke(func(i *int, version string) {
				fmt.Println("version:", version)
				fmt.Println("int:", *i)
			}, "", "${version}")
		})
		c.Provide(func() int { return 3 })
		c.Property("version", "v0.0.1")
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})

	t.Run("run", func(t *testing.T) {
		c := container(func(p gs.Environment) {
			fn := func(i *int, version string) {
				fmt.Println("version:", version)
				fmt.Println("int:", *i)
			}
			_, _ = p.BeanRegistry().Invoke(fn, "", "${version}")
		})
		c.Provide(func() int { return 3 })
		c.Property("version", "v0.0.1")
		c.Property("spring.profiles.active", "dev")
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

type emptyStructA struct{}

type emptyStructB struct{}

func TestEmptyStruct(t *testing.T) {

	c := gs.New()
	objA := &emptyStructA{}
	c.Object(objA)
	objB := &emptyStructB{}
	c.Object(objB)
	err := c.Refresh(internal.AutoClear(false))
	assert.Nil(t, err)

	// objA 和 objB 的地址相同但是类型确实不一样。
	fmt.Printf("objA:%p objB:%p\n", objA, objB)
	fmt.Printf("objA:%#v objB:%#v\n", objA, objB)
}

func TestMapCollection(t *testing.T) {

	type mapValue struct {
		v string
	}

	t.Run("", func(t *testing.T) {
		c := container(func(p gs.Environment) {

			var vSlice []*mapValue
			err := p.BeanRegistry().Get(&vSlice)
			assert.Nil(t, err)
			fmt.Println(vSlice)

			var vMap map[string]*mapValue
			err = p.BeanRegistry().Get(&vMap)
			assert.Nil(t, err)
			fmt.Println(vMap)
		})
		c.Object(&mapValue{"a"}).Name("a").Order(1)
		c.Object(&mapValue{"b"}).Name("b").Order(2)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	})
}

type circularA struct {
	b *circularB
}

func newCircularA(b *circularB) *circularA {
	return &circularA{b: b}
}

type circularB struct {
	A *circularA `autowire:""`
}

func newCircularB() *circularB {
	return new(circularB)
}

func TestLazy(t *testing.T) {
	for i := 0; i < 20; i++ {
		c := container(func(p gs.Environment) {
			var a *circularA
			err := p.BeanRegistry().Get(&a)
			assert.Nil(t, err)
		})
		c.Provide(newCircularA)
		c.Provide(newCircularB)
		err := c.Refresh(internal.AutoClear(false))
		assert.Nil(t, err)
	}
}
