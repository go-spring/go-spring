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

package SpringCore

import (
	"errors"
)

// Constriction 约束条件
type Constriction struct {
	cond    Condition // 判断条件
	profile string    // 运行环境
}

// NewConstriction Constriction 的构造函数
func NewConstriction() *Constriction {
	return &Constriction{}
}

// ConditionOn 为 Constriction 设置一个 Condition
func (c *Constriction) ConditionOn(cond Condition) *Constriction {
	if c.cond != nil {
		panic(errors.New("condition already set"))
	}
	c.cond = cond
	return c
}

// ConditionOnProperty 为 Constriction 设置一个 PropertyCondition
func (c *Constriction) ConditionOnProperty(name string) *Constriction {
	return c.ConditionOn(NewPropertyCondition(name))
}

// ConditionOnMissingProperty 为 Constriction 设置一个 MissingPropertyCondition
func (c *Constriction) ConditionOnMissingProperty(name string) *Constriction {
	return c.ConditionOn(NewMissingPropertyCondition(name))
}

// ConditionOnPropertyValue 为 Constriction 设置一个 PropertyValueCondition
func (c *Constriction) ConditionOnPropertyValue(name string, havingValue interface{}) *Constriction {
	return c.ConditionOn(NewPropertyValueCondition(name, havingValue))
}

// ConditionOnBean 为 Constriction 设置一个 BeanCondition
func (c *Constriction) ConditionOnBean(beanId string) *Constriction {
	return c.ConditionOn(NewBeanCondition(beanId))
}

// ConditionOnMissingBean 为 Constriction 设置一个 MissingBeanCondition
func (c *Constriction) ConditionOnMissingBean(beanId string) *Constriction {
	return c.ConditionOn(NewMissingBeanCondition(beanId))
}

// ConditionOnExpression 为 Constriction 设置一个 ExpressionCondition
func (c *Constriction) ConditionOnExpression(expression string) *Constriction {
	return c.ConditionOn(NewExpressionCondition(expression))
}

// ConditionOnMatches 为 Constriction 设置一个 FunctionCondition
func (c *Constriction) ConditionOnMatches(fn ConditionFunc) *Constriction {
	return c.ConditionOn(NewFunctionCondition(fn))
}

// Profile 为 Constriction 设置运行环境
func (c *Constriction) Profile(profile string) *Constriction {
	if c.profile != "" {
		panic(errors.New("profile already set"))
	}
	c.profile = profile
	return c
}

// Apply 为 Constriction 应用自定义限制
func (c *Constriction) Apply(c0 *Constriction) *Constriction {

	// 设置判断条件
	if c0.cond != nil {
		c.ConditionOn(c0.cond)
	}

	// 设置运行环境
	if c0.profile != "" {
		c.Profile(c0.profile)
	}

	return c
}

// GetResult 获取约束条件的结果
func (c *Constriction) GetResult(ctx SpringContext) bool {

	// 检查是否符合运行环境
	if c.profile != "" && c.profile != ctx.GetProfile() {
		return false
	}

	// 检查是否符合判断条件
	if c.cond != nil && !c.cond.Matches(ctx) {
		return false
	}

	return true
}
