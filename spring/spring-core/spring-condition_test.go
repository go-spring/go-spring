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
	"testing"

	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-utils"
)

func TestFunctionCondition(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	fn := func(ctx SpringCore.SpringContext) bool { return true }
	cond := SpringCore.NewFunctionCondition(fn)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	fn = func(ctx SpringCore.SpringContext) bool { return false }
	cond = SpringCore.NewFunctionCondition(fn)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}

func TestPropertyCondition(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	cond := SpringCore.NewPropertyCondition("int")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewPropertyCondition("bool")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewPropertyCondition("parent")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewPropertyCondition("parent123")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}

func TestMissingPropertyCondition(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	cond := SpringCore.NewMissingPropertyCondition("int")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewMissingPropertyCondition("bool")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewMissingPropertyCondition("parent")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewMissingPropertyCondition("parent123")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
}

func TestPropertyValueCondition(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("str", "this is a str")
	ctx.SetProperty("int", 3)

	cond := SpringCore.NewPropertyValueCondition("int", 3)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	//cond = SpringCore.NewPropertyValueCondition("int", "3")
	//SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewPropertyValueCondition("int", "$>2&&$<4")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewPropertyValueCondition("bool", true)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewPropertyValueCondition("str", "\"$\"==\"this is a str\"")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
}

func TestBeanCondition(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))
	ctx.AutoWireBeans()

	cond := SpringCore.NewBeanCondition("*SpringCore_test.BeanOne")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewBeanCondition("Null")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}

func TestMissingBeanCondition(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))
	ctx.AutoWireBeans()

	cond := SpringCore.NewMissingBeanCondition("*SpringCore_test.BeanOne")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewMissingBeanCondition("Null")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
}

func TestExpressionCondition(t *testing.T) {

}

func TestConditional(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("bool", false)
	ctx.SetProperty("int", 3)
	ctx.AutoWireBeans()

	cond := SpringCore.NewConditional()
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewConditional().OnProperty("int")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewConditional().
		OnProperty("int").
		OnBean("null")
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	SpringUtils.AssertPanic(t, func() {
		cond = SpringCore.NewConditional().OnProperty("int").And()
		SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
	}, "last op need a cond triggered")

	cond = SpringCore.NewConditional().
		OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	cond = SpringCore.NewConditional().
		OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", true)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewConditional().
		OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", true)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewConditional().
		OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)

	SpringUtils.AssertPanic(t, func() {
		cond = SpringCore.NewConditional().
			OnPropertyValue("int", 2).
			Or().
			OnPropertyValue("bool", false).
			Or()
		SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
	}, "last op need a cond triggered")

	cond = SpringCore.NewConditional().
		OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false).
		OnPropertyValue("bool", false)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), true)
}

func TestNotCondition(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProfile("test")
	ctx.AutoWireBeans()

	profileCond := SpringCore.NewProfileCondition("test")
	SpringUtils.AssertEqual(t, profileCond.Matches(ctx), true)

	notCond := SpringCore.NewNotCondition(profileCond)
	SpringUtils.AssertEqual(t, notCond.Matches(ctx), false)

	cond := SpringCore.NewConditional().
		OnPropertyValue("int", 2).
		OnConditionNot(profileCond)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)

	cond = SpringCore.NewConditional().
		OnProfile("test").
		OnConditionNot(profileCond)
	SpringUtils.AssertEqual(t, cond.Matches(ctx), false)
}
