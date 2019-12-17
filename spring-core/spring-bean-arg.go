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
	Fn        interface{}
	Tags      []string
	cond      Condition // 判断条件
	profile   string    // 运行环境
	dependsOn []string  // 非直接依赖
}

//
// 构造函数
//
func NewOptionArg(fn interface{}, tags ...string) *OptionArg {
	return &OptionArg{
		Fn:   fn,
		Tags: tags,
	}
}

func (arg *OptionArg) checkCondition() {
	if arg.cond != nil {
		panic("condition already set")
	}
}

//
// 设置一个 Condition
//
func (arg *OptionArg) ConditionOn(cond Condition) *OptionArg {
	arg.checkCondition()
	arg.cond = cond
	return arg
}

//
// 设置一个 PropertyCondition
//
func (arg *OptionArg) ConditionOnProperty(name string) *OptionArg {
	arg.checkCondition()
	arg.cond = NewPropertyCondition(name)
	return arg
}

//
// 设置一个 MissingPropertyCondition
//
func (arg *OptionArg) ConditionOnMissingProperty(name string) *OptionArg {
	arg.checkCondition()
	arg.cond = NewMissingPropertyCondition(name)
	return arg
}

//
// 设置一个 PropertyValueCondition
//
func (arg *OptionArg) ConditionOnPropertyValue(name string, havingValue interface{}) *OptionArg {
	arg.checkCondition()
	arg.cond = NewPropertyValueCondition(name, havingValue)
	return arg
}

//
// 设置一个 BeanCondition
//
func (arg *OptionArg) ConditionOnBean(beanId string) *OptionArg {
	arg.checkCondition()
	arg.cond = NewBeanCondition(beanId)
	return arg
}

//
// 设置一个 MissingBeanCondition
//
func (arg *OptionArg) ConditionOnMissingBean(beanId string) *OptionArg {
	arg.checkCondition()
	arg.cond = NewMissingBeanCondition(beanId)
	return arg
}

//
// 设置一个 ExpressionCondition
//
func (arg *OptionArg) ConditionOnExpression(expression string) *OptionArg {
	arg.checkCondition()
	arg.cond = NewExpressionCondition(expression)
	return arg
}

//
// 设置一个 FunctionCondition
//
func (arg *OptionArg) ConditionOnMatches(fn ConditionFunc) *OptionArg {
	arg.checkCondition()
	arg.cond = NewFunctionCondition(fn)
	return arg
}

//
// 设置 bean 的运行环境
//
func (arg *OptionArg) Profile(profile string) *OptionArg {
	arg.profile = profile
	return arg
}

//
// 设置 bean 的非直接依赖
//
func (arg *OptionArg) DependsOn(beanId ...string) *OptionArg {
	arg.dependsOn = beanId
	return arg
}

//
// 设置 Bean 应用自定义限制
//
func (arg *OptionArg) Apply(c *Constriction) *OptionArg {

	// 设置条件
	if c.cond != nil {
		arg.checkCondition()
		arg.cond = c.cond
	}

	// 设置运行环境
	if c.profile != "" {
		if arg.profile != "" {
			panic("profile already set")
		}
		arg.profile = c.profile
	}

	// 设置非直接依赖
	if len(c.dependsOn) > 0 {
		if len(arg.dependsOn) > 0 {
			panic("dependsOn already set")
		}
		arg.dependsOn = c.dependsOn
	}

	return arg
}
