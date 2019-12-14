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

type OptionArg struct {
	Fn   interface{}
	Tag  string
	Cond Condition // 条件
}

func NewOptionArg(fn interface{}, tag string) OptionArg {
	return OptionArg{fn, tag, nil}
}

func (arg *OptionArg) checkCondition() {
	if arg.Cond != nil {
		panic("condition already set")
	}
}

//
// 设置一个 Condition
//
func (arg OptionArg) ConditionOn(cond Condition) OptionArg {
	arg.checkCondition()
	arg.Cond = cond
	return arg
}

//
// 设置一个 PropertyCondition
//
func (arg OptionArg) ConditionOnProperty(name string) OptionArg {
	arg.checkCondition()
	arg.Cond = NewPropertyCondition(name)
	return arg
}

//
// 设置一个 MissingPropertyCondition
//
func (arg OptionArg) ConditionOnMissingProperty(name string) OptionArg {
	arg.checkCondition()
	arg.Cond = NewMissingPropertyCondition(name)
	return arg
}

//
// 设置一个 PropertyValueCondition
//
func (arg OptionArg) ConditionOnPropertyValue(name string, havingValue interface{}) OptionArg {
	arg.checkCondition()
	arg.Cond = NewPropertyValueCondition(name, havingValue)
	return arg
}

//
// 设置一个 BeanCondition
//
func (arg OptionArg) ConditionOnBean(beanId string) OptionArg {
	arg.checkCondition()
	arg.Cond = NewBeanCondition(beanId)
	return arg
}

//
// 设置一个 MissingBeanCondition
//
func (arg OptionArg) ConditionOnMissingBean(beanId string) OptionArg {
	arg.checkCondition()
	arg.Cond = NewMissingBeanCondition(beanId)
	return arg
}

//
// 设置一个 ExpressionCondition
//
func (arg OptionArg) ConditionOnExpression(expression string) OptionArg {
	arg.checkCondition()
	arg.Cond = NewExpressionCondition(expression)
	return arg
}

//
// 设置一个 FunctionCondition
//
func (arg OptionArg) ConditionOnMatches(fn ConditionFunc) OptionArg {
	arg.checkCondition()
	arg.Cond = NewFunctionCondition(fn)
	return arg
}

//
// 另外一种形式: key 是 tag，value 是 fn
//
type MapOptionArg map[string]interface{}

func (arg MapOptionArg) checkCondition() {
	if cond, ok := arg["cond"]; ok && cond != nil {
		panic("condition already set")
	}
}

//
// 设置一个 Condition
//
func (arg MapOptionArg) ConditionOn(cond Condition) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = cond
	return arg
}

//
// 设置一个 PropertyCondition
//
func (arg MapOptionArg) ConditionOnProperty(name string) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = NewPropertyCondition(name)
	return arg
}

//
// 设置一个 MissingPropertyCondition
//
func (arg MapOptionArg) ConditionOnMissingProperty(name string) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = NewMissingPropertyCondition(name)
	return arg
}

//
// 设置一个 PropertyValueCondition
//
func (arg MapOptionArg) ConditionOnPropertyValue(name string, havingValue interface{}) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = NewPropertyValueCondition(name, havingValue)
	return arg
}

//
// 设置一个 BeanCondition
//
func (arg MapOptionArg) ConditionOnBean(beanId string) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = NewBeanCondition(beanId)
	return arg
}

//
// 设置一个 MissingBeanCondition
//
func (arg MapOptionArg) ConditionOnMissingBean(beanId string) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = NewMissingBeanCondition(beanId)
	return arg
}

//
// 设置一个 ExpressionCondition
//
func (arg MapOptionArg) ConditionOnExpression(expression string) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = NewExpressionCondition(expression)
	return arg
}

//
// 设置一个 FunctionCondition
//
func (arg MapOptionArg) ConditionOnMatches(fn ConditionFunc) MapOptionArg {
	arg.checkCondition()
	arg["cond"] = NewFunctionCondition(fn)
	return arg
}
