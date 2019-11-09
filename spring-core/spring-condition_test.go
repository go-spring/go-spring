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

	cond := SpringCore.NewPropertyCondition("int")
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewPropertyCondition("bool")
	assert.Equal(t, cond.Matches(ctx), false)
}

func TestConditional(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()
	ctx.SetProperty("int", 3)

	cond := SpringCore.NewConditional()
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewConditional().And()
	assert.Panic(t, func() {
		cond.Matches(ctx)
	}, "last op need a cond triggered")

	cond = SpringCore.NewConditional().ConditionOnProperty("int")
	assert.Equal(t, cond.Matches(ctx), true)

	cond = SpringCore.NewConditional().ConditionOnProperty("bool")
	assert.Equal(t, cond.Matches(ctx), false)
}

func TestConditional_ConditionalOnBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))

	ctx.RegisterBean(new(BeanTwo)).ConditionalOnBean("*SpringCore_test.BeanOne")
	ctx.RegisterNameBean("another_two", new(BeanTwo)).ConditionalOnBean("BeanOne")

	ctx.AutoWireBeans()

	var two *BeanTwo
	ok := ctx.GetBeanByName("", &two)
	assert.Equal(t, ok, true)

	ok = ctx.GetBeanByName("another_two", &two)
	assert.Equal(t, ok, false)
}

func TestConditional_ConditionalOnMissingBean(t *testing.T) {
	ctx := SpringCore.NewDefaultSpringContext()

	ctx.RegisterBean(&BeanZero{5})
	ctx.RegisterBean(new(BeanOne))

	ctx.RegisterBean(new(BeanTwo)).ConditionalOnBean("*SpringCore_test.BeanOne")
	ctx.RegisterNameBean("another_two", new(BeanTwo)).ConditionalOnMissingBean("BeanOne")

	ctx.AutoWireBeans()

	var two *BeanTwo

	assert.Panic(t, func() {
		ctx.GetBeanByName("", &two)
	}, "找到多个符合条件的值")

	ok := ctx.GetBeanByName("another_two", &two)
	assert.Equal(t, ok, true)
}
