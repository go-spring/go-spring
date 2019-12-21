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
// Option 模式绑定参数
//
type OptionArg struct {
	Constriction

	fn   interface{}
	tags []string
}

//
// 构造函数
//
func NewOptionArg(fn interface{}, tags ...string) *OptionArg {
	return &OptionArg{
		fn:   fn,
		tags: tags,
	}
}

//
// 设置一个 Condition
//
func (arg *OptionArg) ConditionOn(cond Condition) *OptionArg {
	arg.Constriction.ConditionOn(cond)
	return arg
}

//
// 设置一个 PropertyCondition
//
func (arg *OptionArg) ConditionOnProperty(name string) *OptionArg {
	arg.Constriction.ConditionOnProperty(name)
	return arg
}

//
// 设置一个 MissingPropertyCondition
//
func (arg *OptionArg) ConditionOnMissingProperty(name string) *OptionArg {
	arg.Constriction.ConditionOnMissingProperty(name)
	return arg
}

//
// 设置一个 PropertyValueCondition
//
func (arg *OptionArg) ConditionOnPropertyValue(name string, havingValue interface{}) *OptionArg {
	arg.Constriction.ConditionOnPropertyValue(name, havingValue)
	return arg
}

//
// 设置一个 BeanCondition
//
func (arg *OptionArg) ConditionOnBean(beanId string) *OptionArg {
	arg.Constriction.ConditionOnBean(beanId)
	return arg
}

//
// 设置一个 MissingBeanCondition
//
func (arg *OptionArg) ConditionOnMissingBean(beanId string) *OptionArg {
	arg.Constriction.ConditionOnMissingBean(beanId)
	return arg
}

//
// 设置一个 ExpressionCondition
//
func (arg *OptionArg) ConditionOnExpression(expression string) *OptionArg {
	arg.Constriction.ConditionOnExpression(expression)
	return arg
}

//
// 设置一个 FunctionCondition
//
func (arg *OptionArg) ConditionOnMatches(fn ConditionFunc) *OptionArg {
	arg.Constriction.ConditionOnMatches(fn)
	return arg
}

//
// 设置 bean 的运行环境
//
func (arg *OptionArg) Profile(profile string) *OptionArg {
	arg.Constriction.Profile(profile)
	return arg
}

//
// 设置 bean 的非直接依赖
//
func (arg *OptionArg) DependsOn(beanId ...string) *OptionArg {
	arg.Constriction.DependsOn(beanId...)
	return arg
}

//
// 设置 Bean 应用自定义限制
//
func (arg *OptionArg) Apply(c *Constriction) *OptionArg {
	arg.Constriction.Apply(c)
	return arg
}
