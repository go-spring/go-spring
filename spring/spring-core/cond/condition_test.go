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

//func TestFunctionCondition(t *testing.T) {
//	ctx := core.NewApplicationContext()
//
//	fn := func(ctx cond.Context) bool { return true }
//	c := cond.matches(fn)
//	assert.True(t, c.Matches(ctx))
//
//	fn = func(ctx cond.Context) bool { return false }
//	c = cond.matches(fn)
//	assert.False(t, c.Matches(ctx))
//}

//func TestPropertyCondition(t *testing.T) {
//
//	ctx := core.NewApplicationContext()
//	ctx.Property("int", 3)
//	ctx.Property("parent.child", 0)
//
//	c := cond.PropertyCondition("int")
//	assert.True(t, c.Matches(ctx))
//
//	c = cond.PropertyCondition("bool")
//	assert.False(t, c.Matches(ctx))
//
//	c = cond.PropertyCondition("parent")
//	assert.True(t, c.Matches(ctx))
//
//	c = cond.PropertyCondition("parent123")
//	assert.False(t, c.Matches(ctx))
//}

//func TestMissingPropertyCondition(t *testing.T) {
//
//	ctx := core.NewApplicationContext()
//	ctx.Property("int", 3)
//	ctx.Property("parent.child", 0)
//
//	c := cond.MissingPropertyCondition("int")
//	assert.False(t, c.Matches(ctx))
//
//	c = cond.MissingPropertyCondition("bool")
//	assert.True(t, c.Matches(ctx))
//
//	c = cond.MissingPropertyCondition("parent")
//	assert.False(t, c.Matches(ctx))
//
//	c = cond.MissingPropertyCondition("parent123")
//	assert.True(t, c.Matches(ctx))
//}

//func TestPropertyValueCondition(t *testing.T) {
//
//	ctx := core.NewApplicationContext()
//	ctx.Property("str", "this is a str")
//	ctx.Property("int", 3)
//
//	c := cond.PropertyValueCondition("int", 3)
//	assert.True(t, c.Matches(ctx))
//
//	//c = cond.PropertyValueCondition("int", "3")
//	//util.Equal(t, c.Matches(ctx), true)
//
//	c = cond.PropertyValueCondition("int", "$>2&&$<4")
//	assert.True(t, c.Matches(ctx))
//
//	c = cond.PropertyValueCondition("bool", true)
//	assert.False(t, c.Matches(ctx))
//
//	c = cond.PropertyValueCondition("str", "\"$\"==\"this is a str\"")
//	assert.True(t, c.Matches(ctx))
//}

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

//func TestBeanCondition(t *testing.T) {
//
//	ctx := core.NewApplicationContext()
//	ctx.Bean(&BeanZero{5})
//	ctx.Bean(new(BeanOne))
//	ctx.Refresh()
//
//	c := cond.BeanCondition("*cond_test.BeanOne")
//	assert.True(t, c.Matches(ctx))
//
//	c = cond.BeanCondition("Null")
//	assert.False(t, c.Matches(ctx))
//}

//func TestMissingBeanCondition(t *testing.T) {
//
//	ctx := core.NewApplicationContext()
//	ctx.Bean(&BeanZero{5})
//	ctx.Bean(new(BeanOne))
//	ctx.Refresh()
//
//	c := cond.MissingBeanCondition("*cond_test.BeanOne")
//	assert.False(t, c.Matches(ctx))
//
//	c = cond.MissingBeanCondition("Null")
//	assert.True(t, c.Matches(ctx))
//}

func TestExpressionCondition(t *testing.T) {

}

func TestConditional(t *testing.T) {

	ctx := core.NewApplicationContext()
	ctx.Property("bool", false)
	ctx.Property("int", 3)
	ctx.Refresh()

	c := cond.OnProperty("int")
	assert.True(t, c.Matches(ctx))

	c = cond.OnProperty("int").OnBean("null")
	assert.False(t, c.Matches(ctx))

	assert.Panic(t, func() {
		c = cond.OnProperty("int").And()
		assert.Equal(t, c.Matches(ctx), true)
	}, "last op need a cond triggered")

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
	}, "last op need a cond triggered")

	c = cond.OnPropertyValue("int", 2).
		Or().
		OnPropertyValue("bool", false).
		OnPropertyValue("bool", false)
	assert.True(t, c.Matches(ctx))
}

//func TestNotCondition(t *testing.T) {
//
//	ctx := core.NewApplicationContext()
//	ctx.Profile("test")
//	ctx.Refresh()
//
//	//profileCond := cond.ProfileCondition("test")
//	//assert.True(t, profileCond.Matches(ctx))
//
//	//notCond := cond.Not(profileCond)
//	//assert.False(t, notCond.Matches(ctx))
//
//	//c := cond.OnPropertyValue("int", 2).
//	//	OnConditionNot(profileCond)
//	//assert.False(t, c.Matches(ctx))
//	//
//	//c = cond.OnProfile("test").
//	//	OnConditionNot(profileCond)
//	//assert.False(t, c.Matches(ctx))
//}
