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

package SpringCond_test

import (
	"errors"
	"testing"

	"github.com/go-spring/spring-cond"
	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-utils"
)

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

func TestDefaultSpringContext(t *testing.T) {

	t.Run("bean:test_ctx:", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(&BeanZero{5}).WithCondition(SpringCond.
			OnProfile("test").
			And().
			OnMissingBean("null"),
		)

		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		SpringUtils.AssertEqual(t, ok, false)
	})

	t.Run("bean:test_ctx:test", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProfile("test")
		ctx.RegisterBean(&BeanZero{5}).WithCondition(SpringCond.OnProfile("test"))
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		SpringUtils.AssertEqual(t, ok, true)
	})

	t.Run("bean:test_ctx:stable", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProfile("stable")
		ctx.RegisterBean(&BeanZero{5}).WithCondition(SpringCond.OnProfile("test"))
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		SpringUtils.AssertEqual(t, ok, false)
	})

	t.Run("option withClassName Condition", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.SetProperty("class_floor", 2)
		ctx.RegisterBeanFn(NewClassRoom).Options(
			SpringCore.NewOptionArg(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}",
			).WithCondition(SpringCond.OnProperty("class_name_enable")),
		)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		SpringUtils.AssertEqual(t, cls.floor, 0)
		SpringUtils.AssertEqual(t, len(cls.students), 0)
		SpringUtils.AssertEqual(t, cls.className, "default")
		SpringUtils.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName Apply", func(t *testing.T) {
		c := SpringCond.OnProperty("class_name_enable")

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.RegisterBeanFn(NewClassRoom).Options(
			SpringCore.NewOptionArg(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}",
			).WithCondition(c),
		)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		SpringUtils.AssertEqual(t, cls.floor, 0)
		SpringUtils.AssertEqual(t, len(cls.students), 0)
		SpringUtils.AssertEqual(t, cls.className, "default")
		SpringUtils.AssertEqual(t, cls.President, "CaiYuanPei")
	})

	t.Run("method bean condition", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.RegisterBean(new(Server))
		ctx.RegisterMethodBean(parent, "Consumer").WithCondition(SpringCond.OnProperty("consumer.enable"))
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		SpringUtils.AssertEqual(t, ok, true)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, false)
	})

	t.Run("fn method bean condition", func(t *testing.T) {

		ctx := SpringCore.NewDefaultSpringContext()
		ctx.SetProperty("server.version", "1.0.0")
		ctx.RegisterBeanFn(NewServerInterface)
		ctx.RegisterMethodBeanFn(ServerInterface.ConsumerT).WithCondition(SpringCond.OnProperty("consumer.enable"))
		ctx.AutoWireBeans()

		var si ServerInterface
		ok := ctx.GetBean(&si)
		SpringUtils.AssertEqual(t, ok, true)

		s := si.(*Server)
		SpringUtils.AssertEqual(t, s.Version, "1.0.0")

		var c *Consumer
		ok = ctx.GetBean(&c)
		SpringUtils.AssertEqual(t, ok, false)
	})
}

func TestDefaultSpringContext_ParentNotRegister(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	parent := ctx.RegisterBeanFn(NewServerInterface).
		WithCondition(SpringCond.OnProperty("server.is.nil"))
	ctx.RegisterMethodBean(parent, "Consumer")

	ctx.AutoWireBeans()

	var s *Server
	ok := ctx.GetBean(&s)
	SpringUtils.AssertEqual(t, ok, false)

	var c *Consumer
	ok = ctx.GetBean(&c)
	SpringUtils.AssertEqual(t, ok, false)
}

func TestDefaultSpringContext_ChainConditionOnBean(t *testing.T) {
	for i := 0; i < 20; i++ { // 不要排序
		ctx := SpringCore.NewDefaultSpringContext()
		ctx.RegisterBean(new(string)).WithCondition(SpringCond.OnBean("*bool"))
		ctx.RegisterBean(new(bool)).WithCondition(SpringCond.OnBean("*int"))
		ctx.RegisterBean(new(int)).WithCondition(SpringCond.OnBean("*float"))
		ctx.AutoWireBeans()
		SpringUtils.AssertEqual(t, len(ctx.GetBeanDefinitions()), 0)
	}
}

func TestDefaultSpringContext_ConditionOnBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	c := SpringCond.
		OnMissingProperty("Null").
		Or().
		OnProfile("test")

	ctx.RegisterBean(&BeanZero{5}).WithCondition(SpringCond.
		On(c).
		And().
		OnMissingBean("null"),
	)

	ctx.RegisterBean(new(BeanOne)).WithCondition(SpringCond.
		On(c).
		And().
		OnMissingBean("null"),
	)

	ctx.RegisterBean(new(BeanTwo)).WithCondition(SpringCond.OnBean("*SpringCond_test.BeanOne"))
	ctx.RegisterNameBean("another_two", new(BeanTwo)).WithCondition(SpringCond.OnBean("Null"))

	ctx.AutoWireBeans()

	var two *BeanTwo
	ok := ctx.GetBean(&two, "")
	SpringUtils.AssertEqual(t, ok, true)

	ok = ctx.GetBean(&two, "another_two")
	SpringUtils.AssertEqual(t, ok, false)
}

func TestDefaultSpringContext_ConditionOnMissingBean(t *testing.T) {

	for i := 0; i < 20; i++ { // 测试 FindBean 无需绑定，不要排序
		ctx := SpringCore.NewDefaultSpringContext()

		ctx.RegisterBean(&BeanZero{5})
		ctx.RegisterBean(new(BeanOne))

		ctx.RegisterBean(new(BeanTwo)).WithCondition(SpringCond.OnMissingBean("*SpringCond_test.BeanOne"))
		ctx.RegisterNameBean("another_two", new(BeanTwo)).WithCondition(SpringCond.OnMissingBean("Null"))

		ctx.AutoWireBeans()

		var two *BeanTwo
		ok := ctx.GetBean(&two, "")
		SpringUtils.AssertEqual(t, ok, true)

		ok = ctx.GetBean(&two, "another_two")
		SpringUtils.AssertEqual(t, ok, true)
	}
}
