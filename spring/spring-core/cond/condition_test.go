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
	"testing"

	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/core"
	"github.com/go-spring/spring-utils"
)

func TestFunctionCondition(t *testing.T) {
	ctx := core.NewApplicationContext()

	fn := func(ctx bean.ConditionContext) bool { return true }
	c := cond.FunctionCondition(fn)
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	fn = func(ctx bean.ConditionContext) bool { return false }
	c = cond.FunctionCondition(fn)
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)
}

func TestPropertyCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Property("int", 3)
	ctx.Property("parent.child", 0)

	c := cond.PropertyCondition("int")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.PropertyCondition("bool")
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.PropertyCondition("parent")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.PropertyCondition("parent123")
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)
}

func TestMissingPropertyCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Property("int", 3)
	ctx.Property("parent.child", 0)

	c := cond.MissingPropertyCondition("int")
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.MissingPropertyCondition("bool")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.MissingPropertyCondition("parent")
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.MissingPropertyCondition("parent123")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)
}

func TestPropertyValueCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Property("str", "this is a str")
	ctx.Property("int", 3)

	c := cond.PropertyValueCondition("int", 3)
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	//c = cond.PropertyValueCondition("int", "3")
	//SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.PropertyValueCondition("int", "$>2&&$<4")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.PropertyValueCondition("bool", true)
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.PropertyValueCondition("str", "\"$\"==\"this is a str\"")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)
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

func TestBeanCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Register(bean.Ref(&BeanZero{5}))
	ctx.Register(bean.Ref(new(BeanOne)))
	ctx.AutoWireBeans()

	c := cond.BeanCondition("*cond_test.BeanOne")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.BeanCondition("Null")
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)
}

func TestMissingBeanCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Register(bean.Ref(&BeanZero{5}))
	ctx.Register(bean.Ref(new(BeanOne)))
	ctx.AutoWireBeans()

	c := cond.MissingBeanCondition("*cond_test.BeanOne")
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.MissingBeanCondition("Null")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)
}

func TestExpressionCondition(t *testing.T) {

}

func TestConditional(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Property("bool", false)
	ctx.Property("int", 3)
	ctx.AutoWireBeans()

	c := cond.OnProperty("int")
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.OnProperty("int").OnBean("null")
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	SpringUtils.AssertPanic(t, func() {
		c = cond.OnProperty("int").And()
		SpringUtils.AssertEqual(t, c.Matches(ctx), true)
	}, "last op need a cond triggered")

	c = cond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	c = cond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", true)
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", true)
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)

	SpringUtils.AssertPanic(t, func() {
		c = cond.OnPropertyValue("int", 2).
			Or().
			OnPropertyValue("bool", false).
			Or()
		SpringUtils.AssertEqual(t, c.Matches(ctx), true)
	}, "last op need a cond triggered")

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false).
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, c.Matches(ctx), true)
}

func TestNotCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Profile("test")
	ctx.AutoWireBeans()

	profileCond := cond.ProfileCondition("test")
	SpringUtils.AssertEqual(t, profileCond.Matches(ctx), true)

	notCond := cond.NotCondition(profileCond)
	SpringUtils.AssertEqual(t, notCond.Matches(ctx), false)

	c := cond.OnPropertyValue("int", 2).
		OnConditionNot(profileCond)
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)

	c = cond.OnProfile("test").
		OnConditionNot(profileCond)
	SpringUtils.AssertEqual(t, c.Matches(ctx), false)
}
