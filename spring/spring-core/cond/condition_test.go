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

	"github.com/go-spring/spring-core/assert"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/core"
)

func TestFunctionCondition(t *testing.T) {
	ctx := core.NewApplicationContext()

	fn := func(ctx cond.Context) bool { return true }
	c := cond.FunctionCondition(fn)
	assert.Equal(t, c.Matches(ctx), true)

	fn = func(ctx cond.Context) bool { return false }
	c = cond.FunctionCondition(fn)
	assert.Equal(t, c.Matches(ctx), false)
}

func TestPropertyCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	c := cond.PropertyCondition("int")
	assert.Equal(t, c.Matches(ctx), true)

	c = cond.PropertyCondition("bool")
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.PropertyCondition("parent")
	assert.Equal(t, c.Matches(ctx), true)

	c = cond.PropertyCondition("parent123")
	assert.Equal(t, c.Matches(ctx), false)
}

func TestMissingPropertyCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.SetProperty("int", 3)
	ctx.SetProperty("parent.child", 0)

	c := cond.MissingPropertyCondition("int")
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.MissingPropertyCondition("bool")
	assert.Equal(t, c.Matches(ctx), true)

	c = cond.MissingPropertyCondition("parent")
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.MissingPropertyCondition("parent123")
	assert.Equal(t, c.Matches(ctx), true)
}

func TestPropertyValueCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.SetProperty("str", "this is a str")
	ctx.SetProperty("int", 3)

	c := cond.PropertyValueCondition("int", 3)
	assert.Equal(t, c.Matches(ctx), true)

	//c = cond.PropertyValueCondition("int", "3")
	//util.Equal(t, c.Matches(ctx), true)

	c = cond.PropertyValueCondition("int", "$>2&&$<4")
	assert.Equal(t, c.Matches(ctx), true)

	c = cond.PropertyValueCondition("bool", true)
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.PropertyValueCondition("str", "\"$\"==\"this is a str\"")
	assert.Equal(t, c.Matches(ctx), true)
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
	ctx.ObjBean(&BeanZero{5})
	ctx.ObjBean(new(BeanOne))
	ctx.AutoWireBeans()

	c := cond.BeanCondition("*cond_test.BeanOne")
	assert.Equal(t, c.Matches(ctx), true)

	c = cond.BeanCondition("Null")
	assert.Equal(t, c.Matches(ctx), false)
}

func TestMissingBeanCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.ObjBean(&BeanZero{5})
	ctx.ObjBean(new(BeanOne))
	ctx.AutoWireBeans()

	c := cond.MissingBeanCondition("*cond_test.BeanOne")
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.MissingBeanCondition("Null")
	assert.Equal(t, c.Matches(ctx), true)
}

func TestExpressionCondition(t *testing.T) {

}

func TestConditional(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.SetProperty("bool", false)
	ctx.SetProperty("int", 3)
	ctx.AutoWireBeans()

	c := cond.OnProperty("int")
	assert.Equal(t, c.Matches(ctx), true)

	c = cond.OnProperty("int").OnBean("null")
	assert.Equal(t, c.Matches(ctx), false)

	assert.Panic(t, func() {
		c = cond.OnProperty("int").And()
		assert.Equal(t, c.Matches(ctx), true)
	}, "last op need a cond triggered")

	c = cond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", false)
	assert.Equal(t, c.Matches(ctx), true)

	c = cond.OnPropertyValue("int", 3).
		And().
		OnPropertyValue("bool", true)
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", true)
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false)
	assert.Equal(t, c.Matches(ctx), true)

	assert.Panic(t, func() {
		c = cond.OnPropertyValue("int", 2).
			Or().
			OnPropertyValue("bool", false).
			Or()
		assert.Equal(t, c.Matches(ctx), true)
	}, "last op need a cond triggered")

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false).
		OnPropertyValue("bool", false)
	assert.Equal(t, c.Matches(ctx), true)
}

func TestNotCondition(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.SetProfile("test")
	ctx.AutoWireBeans()

	profileCond := cond.ProfileCondition("test")
	assert.Equal(t, profileCond.Matches(ctx), true)

	notCond := cond.NotCondition(profileCond)
	assert.Equal(t, notCond.Matches(ctx), false)

	c := cond.OnPropertyValue("int", 2).
		OnConditionNot(profileCond)
	assert.Equal(t, c.Matches(ctx), false)

	c = cond.OnProfile("test").
		OnConditionNot(profileCond)
	assert.Equal(t, c.Matches(ctx), false)
}
