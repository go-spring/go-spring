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
	"testing"

	"github.com/go-spring/spring-cond"
	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-utils"
)

func TestFunctionCondition(t *testing.T) {
	ctx := SpringCore.NewApplicationContext()

	fn := func(ctx SpringCore.ApplicationContext) bool { return true }
	cond := SpringCond.FunctionCondition(fn)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	fn = func(ctx SpringCore.ApplicationContext) bool { return false }
	cond = SpringCond.FunctionCondition(fn)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}

func TestPropertyCondition(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	cond := SpringCond.PropertyCondition("int")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.PropertyCondition("bool")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.PropertyCondition("parent")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.PropertyCondition("parent123")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}

func TestMissingPropertyCondition(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	cond := SpringCond.MissingPropertyCondition("int")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.MissingPropertyCondition("bool")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.MissingPropertyCondition("parent")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.MissingPropertyCondition("parent123")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
}

func TestPropertyValueCondition(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("str", "this is a str")
	ctx.SetProperty("int", 3)

	cond := SpringCond.PropertyValueCondition("int", 3)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	//cond = SpringCond.PropertyValueCondition("int", "3")
	//SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.PropertyValueCondition("int", "$>2&&$<4")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.PropertyValueCondition("bool", true)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.PropertyValueCondition("str", "\"$\"==\"this is a str\"")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
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

	ctx := SpringCore.NewApplicationContext()
	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))
	ctx.AutoWireBeans()

	cond := SpringCond.BeanCondition("*SpringCond_test.BeanOne")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.BeanCondition("Null")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}

func TestMissingBeanCondition(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))
	ctx.AutoWireBeans()

	cond := SpringCond.MissingBeanCondition("*SpringCond_test.BeanOne")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.MissingBeanCondition("Null")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
}

func TestExpressionCondition(t *testing.T) {

}

func TestConditional(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProperty("bool", false)
	ctx.SetProperty("int", 3)
	ctx.AutoWireBeans()

	cond := SpringCond.OnProperty("int")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.OnProperty("int").OnBean("null")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	SpringUtils.AssertPanic(t, func() {
		cond = SpringCond.OnProperty("int").And()
		SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
	}, "last op need a cond triggered")

	cond = SpringCond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", true)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", true)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	SpringUtils.AssertPanic(t, func() {
		cond = SpringCond.OnPropertyValue("int", 2).
			Or().
			OnPropertyValue("bool", false).
			Or()
		SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
	}, "last op need a cond triggered")

	cond = SpringCond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false).
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
}

func TestNotCondition(t *testing.T) {

	ctx := SpringCore.NewApplicationContext()
	ctx.SetProfile("test")
	ctx.AutoWireBeans()

	profileCond := SpringCond.ProfileCondition("test")
	SpringUtils.AssertEqual(t, profileCond.Matches(ctx), true)

	notCond := SpringCond.NotCondition(profileCond)
	SpringUtils.AssertEqual(t, notCond.Matches(ctx), false)

	cond := SpringCond.OnPropertyValue("int", 2).
		OnConditionNot(profileCond)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCond.OnProfile("test").
		OnConditionNot(profileCond)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}
