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

//
// 对 Bean 或者 Option 限制
//
type Constriction struct {
	cond      Condition // 判断条件
	profile   string    // 运行环境
	dependsOn []string  // 非直接依赖
}

//
// 构造函数
//
func NewConstriction() *Constriction {
	return &Constriction{}
}

func (c *Constriction) checkCondition() {
	if c.cond != nil {
		panic("condition already set")
	}
}

//
// 设置一个 Condition
//
func (c *Constriction) ConditionOn(cond Condition) *Constriction {
	c.checkCondition()
	c.cond = cond
	return c
}

//
// 设置一个 PropertyCondition
//
func (c *Constriction) ConditionOnProperty(name string) *Constriction {
	c.checkCondition()
	c.cond = NewPropertyCondition(name)
	return c
}

//
// 设置一个 MissingPropertyCondition
//
func (c *Constriction) ConditionOnMissingProperty(name string) *Constriction {
	c.checkCondition()
	c.cond = NewMissingPropertyCondition(name)
	return c
}

//
// 设置一个 PropertyValueCondition
//
func (c *Constriction) ConditionOnPropertyValue(name string, havingValue interface{}) *Constriction {
	c.checkCondition()
	c.cond = NewPropertyValueCondition(name, havingValue)
	return c
}

//
// 设置一个 BeanCondition
//
func (c *Constriction) ConditionOnBean(beanId string) *Constriction {
	c.checkCondition()
	c.cond = NewBeanCondition(beanId)
	return c
}

//
// 设置一个 MissingBeanCondition
//
func (c *Constriction) ConditionOnMissingBean(beanId string) *Constriction {
	c.checkCondition()
	c.cond = NewMissingBeanCondition(beanId)
	return c
}

//
// 设置一个 ExpressionCondition
//
func (c *Constriction) ConditionOnExpression(expression string) *Constriction {
	c.checkCondition()
	c.cond = NewExpressionCondition(expression)
	return c
}

//
// 设置一个 FunctionCondition
//
func (c *Constriction) ConditionOnMatches(fn ConditionFunc) *Constriction {
	c.checkCondition()
	c.cond = NewFunctionCondition(fn)
	return c
}

//
// 设置 bean 的运行环境
//
func (c *Constriction) Profile(profile string) *Constriction {
	if c.profile != "" {
		panic("profile already set")
	}
	c.profile = profile
	return c
}

//
// 设置 bean 的非直接依赖
//
func (c *Constriction) DependsOn(beanId ...string) *Constriction {
	if len(c.dependsOn) > 0 {
		panic("dependsOn already set")
	}
	c.dependsOn = beanId
	return c
}

//
// 应用自定义限制
//
func (c *Constriction) Apply(c0 *Constriction) *Constriction {

	// 设置条件
	if c0.cond != nil {
		c.checkCondition()
		c.cond = c0.cond
	}

	// 设置运行环境
	if c0.profile != "" {
		c.Profile(c0.profile)
	}

	// 设置非直接依赖
	if len(c0.dependsOn) > 0 {
		c.DependsOn(c0.dependsOn...)
	}

	return c
}

//
// GetResult
//
func (c *Constriction) GetResult(ctx SpringContext) bool {

	// 检查是否符合运行环境
	if c.profile != "" && c.profile != ctx.GetProfile() {
		return false
	}

	// 检查是否符合注册条件
	if c.cond != nil && !c.cond.Matches(ctx) {
		return false
	}

	return true
}
