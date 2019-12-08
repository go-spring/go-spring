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

	"github.com/go-spring/go-spring/spring-core"
	"github.com/magiconair/properties/assert"
)

func TestFunctionCondition(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	cond := SpringCore.NewFunctionCondition(func(ctx SpringCore.SpringContext) bool {
		return true
	})
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewFunctionCondition(func(ctx SpringCore.SpringContext) bool {
		return false
	})
	assert.Equal(t, cond.Matches(ctx), false)
}

func TestPropertyCondition(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("int", 3)

	cond := SpringCore.NewPropertyCondition("int", "3")
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewPropertyCondition("bool", "true")
	assert.Equal(t, cond.Matches(ctx), false)
}

func TestBeanCondition(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))

	ctx.AutoWireBeans()

	cond := SpringCore.NewBeanCondition("*SpringCore_test.BeanOne")
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewBeanCondition("Null")
	assert.Equal(t, cond.Matches(ctx), false)
}

func TestMissingBeanCondition(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))

	ctx.AutoWireBeans()

	cond := SpringCore.NewMissingBeanCondition("*SpringCore_test.BeanOne")
	assert.Equal(t, cond.Matches(ctx), false)

	cond = SpringCore.NewMissingBeanCondition("Null")
	assert.Equal(t, cond.Matches(ctx), true)
}

func TestExpressionCondition(t *testing.T) {

}

func TestConditional(t *testing.T) {

	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("bool", false)
	ctx.SetProperty("int", 3)
	ctx.AutoWireBeans()

	cond := SpringCore.NewConditional()
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewConditional().OnProperty("int", "3")
	assert.Equal(t, cond.Matches(ctx), true)

	assert.Panic(t, func() {
		cond = SpringCore.NewConditional().OnProperty("int", "3").OnBean("null")
		assert.Equal(t, cond.Matches(ctx), true)
	}, "condition already set")

	assert.Panic(t, func() {
		cond = SpringCore.NewConditional().OnProperty("int", "3").And()
		assert.Equal(t, cond.Matches(ctx), true)
	}, "last op need a cond triggered")

	cond = SpringCore.NewConditional().
		OnProperty("int", "3").
		And().
		OnProperty("bool", "false")
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewConditional().
		OnProperty("int", "3").
		And().
		OnProperty("bool", "true")
	assert.Equal(t, cond.Matches(ctx), false)

	cond = SpringCore.NewConditional().
		OnProperty("int", "2").
		Or().
		OnProperty("bool", "true")
	assert.Equal(t, cond.Matches(ctx), false)

	cond = SpringCore.NewConditional().
		OnProperty("int", "2").
		Or().
		OnProperty("bool", "false")
	assert.Equal(t, cond.Matches(ctx), true)

	assert.Panic(t, func() {
		cond = SpringCore.NewConditional().
			OnProperty("int", "2").
			Or().
			OnProperty("bool", "false").
			Or()
		assert.Equal(t, cond.Matches(ctx), true)
	}, "last op need a cond triggered")

	assert.Panic(t, func() {
		cond = SpringCore.NewConditional().
			OnProperty("int", "2").
			Or().
			OnProperty("bool", "false").
			OnProperty("bool", "false")
		assert.Equal(t, cond.Matches(ctx), true)
	}, "condition already set")
}
