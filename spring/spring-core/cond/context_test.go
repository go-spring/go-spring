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

package cond_test

import (
	"errors"
	"testing"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/core"
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

		ctx := core.NewApplicationContext()
		ctx.ObjBean(&BeanZero{5}).Cond(cond.
			OnProfile("test").
			And().
			OnMissingBean("null"),
		)

		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, false)
	})

	t.Run("bean:test_ctx:test", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProfile("test")
		ctx.ObjBean(&BeanZero{5}).Cond(cond.OnProfile("test"))
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, true)
	})

	t.Run("bean:test_ctx:stable", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProfile("stable")
		ctx.ObjBean(&BeanZero{5}).Cond(cond.OnProfile("test"))
		ctx.AutoWireBeans()

		var b *BeanZero
		ok := ctx.GetBean(&b)
		assert.Equal(t, ok, false)
	})

	t.Run("option withClassName Condition", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.SetProperty("class_floor", 2)
		ctx.CtorBean(NewClassRoom, arg.Option(withClassName,
			"${class_name:=二年级03班}",
			"${class_floor:=3}",
		).Cond(cond.OnProperty("class_name_enable")))
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("option withClassName Apply", func(t *testing.T) {
		c := cond.OnProperty("class_name_enable")

		ctx := core.NewApplicationContext()
		ctx.SetProperty("president", "CaiYuanPei")
		ctx.CtorBean(NewClassRoom,
			arg.Option(withClassName,
				"${class_name:=二年级03班}",
				"${class_floor:=3}",
			).Cond(c),
		)
		ctx.AutoWireBeans()

		var cls *ClassRoom
		ctx.GetBean(&cls)

		assert.Equal(t, cls.floor, 0)
		assert.Equal(t, len(cls.students), 0)
		assert.Equal(t, cls.className, "default")
		assert.Equal(t, cls.President, "CaiYuanPei")
	})

	t.Run("method bean cond", func(t *testing.T) {

		ctx := core.NewApplicationContext()
		ctx.SetProperty("server.version", "1.0.0")
		parent := ctx.ObjBean(new(Server))
		ctx.CtorBean((*Server).Consumer, parent.BeanId()).Cond(cond.OnProperty("consumer.enable"))
		ctx.AutoWireBeans()

		var s *Server
		ok := ctx.GetBean(&s)
		assert.Equal(t, ok, true)
		assert.Equal(t, s.Version, "1.0.0")

		var c *Consumer
		ok = ctx.GetBean(&c)
		assert.Equal(t, ok, false)
	})
}

// TODO 现在的方式父 Bean 不存在子 Bean 创建的时候会报错
//func TestDefaultSpringContext_ParentNotRegister(t *testing.T) {
//
//	ctx := core.NewApplicationContext()
//	parent := ctx.CtorBean(NewServerInterface).Cond(cond.OnProperty("server.is.nil"))
//	ctx.CtorBean(ServerInterface.Consumer, parent.BeanId())
//
//	ctx.AutoWireBeans()
//
//	var s *Server
//	ok := ctx.GetBean(&s)
//	util.Equal(t, ok, false)
//
//	var c *Consumer
//	ok = ctx.GetBean(&c)
//	util.Equal(t, ok, false)
//}

func TestDefaultSpringContext_ChainConditionOnBean(t *testing.T) {
	for i := 0; i < 20; i++ { // 不要排序
		ctx := core.NewApplicationContext()
		ctx.ObjBean(new(string)).Cond(cond.OnBean("*bool"))
		ctx.ObjBean(new(bool)).Cond(cond.OnBean("*int"))
		ctx.ObjBean(new(int)).Cond(cond.OnBean("*float"))
		ctx.AutoWireBeans()
		assert.Equal(t, len(ctx.Beans()), 0)
	}
}

func TestDefaultSpringContext_ConditionOnBean(t *testing.T) {
	ctx := core.NewApplicationContext()

	c := cond.
		OnMissingProperty("Null").
		Or().
		OnProfile("test")

	ctx.ObjBean(&BeanZero{5}).Cond(cond.
		On(c).
		And().
		OnMissingBean("null"),
	)

	ctx.ObjBean(new(BeanOne)).Cond(cond.
		On(c).
		And().
		OnMissingBean("null"),
	)

	ctx.ObjBean(new(BeanTwo)).Cond(cond.OnBean("*cond_test.BeanOne"))
	ctx.ObjBean(new(BeanTwo)).Name("another_two").Cond(cond.OnBean("Null"))

	ctx.AutoWireBeans()

	var two *BeanTwo
	ok := ctx.GetBean(&two, "")
	assert.Equal(t, ok, true)

	ok = ctx.GetBean(&two, "another_two")
	assert.Equal(t, ok, false)
}

func TestDefaultSpringContext_ConditionOnMissingBean(t *testing.T) {

	for i := 0; i < 20; i++ { // 测试 FindBean 无需绑定，不要排序
		ctx := core.NewApplicationContext()

		ctx.ObjBean(&BeanZero{5})
		ctx.ObjBean(new(BeanOne))

		ctx.ObjBean(new(BeanTwo)).Cond(cond.OnMissingBean("*cond_test.BeanOne"))
		ctx.ObjBean(new(BeanTwo)).Name("another_two").Cond(cond.OnMissingBean("Null"))

		ctx.AutoWireBeans()

		var two *BeanTwo
		ok := ctx.GetBean(&two, "")
		assert.Equal(t, ok, true)

		ok = ctx.GetBean(&two, "another_two")
		assert.Equal(t, ok, true)
	}
}
