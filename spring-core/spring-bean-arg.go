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
// optionArg Option 模式绑定参数
//
type optionArg struct {
	Constriction

	fn   interface{}
	tags []string
}

//
// NewOptionArg 构造函数
//
func NewOptionArg(fn interface{}, tags ...string) *optionArg {
	return &optionArg{
		fn:   fn,
		tags: tags,
	}
}

//
// ConditionOn 设置一个 Condition
//
func (arg *optionArg) ConditionOn(cond Condition) *optionArg {
	arg.Constriction.ConditionOn(cond)
	return arg
}

//
// ConditionOnProperty 设置一个 PropertyCondition
//
func (arg *optionArg) ConditionOnProperty(name string) *optionArg {
	arg.Constriction.ConditionOnProperty(name)
	return arg
}

//
// ConditionOnMissingProperty 设置一个 MissingPropertyCondition
//
func (arg *optionArg) ConditionOnMissingProperty(name string) *optionArg {
	arg.Constriction.ConditionOnMissingProperty(name)
	return arg
}

//
// ConditionOnPropertyValue 设置一个 PropertyValueCondition
//
func (arg *optionArg) ConditionOnPropertyValue(name string, havingValue interface{}) *optionArg {
	arg.Constriction.ConditionOnPropertyValue(name, havingValue)
	return arg
}

//
// ConditionOnBean 设置一个 BeanCondition
//
func (arg *optionArg) ConditionOnBean(beanId string) *optionArg {
	arg.Constriction.ConditionOnBean(beanId)
	return arg
}

//
// ConditionOnMissingBean 设置一个 MissingBeanCondition
//
func (arg *optionArg) ConditionOnMissingBean(beanId string) *optionArg {
	arg.Constriction.ConditionOnMissingBean(beanId)
	return arg
}

//
// ConditionOnExpression 设置一个 ExpressionCondition
//
func (arg *optionArg) ConditionOnExpression(expression string) *optionArg {
	arg.Constriction.ConditionOnExpression(expression)
	return arg
}

//
// ConditionOnMatches 设置一个 FunctionCondition
//
func (arg *optionArg) ConditionOnMatches(fn ConditionFunc) *optionArg {
	arg.Constriction.ConditionOnMatches(fn)
	return arg
}

//
// Profile 设置 bean 的运行环境
//
func (arg *optionArg) Profile(profile string) *optionArg {
	arg.Constriction.Profile(profile)
	return arg
}

//
// DependsOn 设置 bean 的非直接依赖
//
func (arg *optionArg) DependsOn(beanId ...string) *optionArg {
	arg.Constriction.DependsOn(beanId...)
	return arg
}

//
// Apply 设置 Bean 应用自定义限制
//
func (arg *optionArg) Apply(c *Constriction) *optionArg {
	arg.Constriction.Apply(c)
	return arg
}
