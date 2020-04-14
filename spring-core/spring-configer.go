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
	"fmt"
	"reflect"
	"runtime"
)

// Configer 封装配置函数
type Configer struct {
	fn        interface{}
	stringArg *fnStringBindingArg // 普通参数绑定
	optionArg *fnOptionBindingArg // Option 绑定
	cond      *Conditional        // 判断条件
}

// newConfiger Configer 的构造函数
func NewConfiger(fn interface{}, tags []string) *Configer {

	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic(errors.New("fn must be a func"))
	}

	return &Configer{
		fn:        fn,
		stringArg: newFnStringBindingArg(fnType, false, tags),
		cond:      NewConditional(),
	}
}

// Options 设置 Option 模式函数的参数绑定
func (c *Configer) Options(options ...*optionArg) *Configer {
	c.optionArg = &fnOptionBindingArg{options}
	return c
}

// Or c=a||b
func (c *Configer) Or() *Configer {
	c.cond.Or()
	return c
}

// And c=a&&b
func (c *Configer) And() *Configer {
	c.cond.And()
	return c
}

// ConditionOn 为 Bean 设置一个 Condition
func (c *Configer) ConditionOn(cond Condition) *Configer {
	c.cond.OnCondition(cond)
	return c
}

// ConditionNot 为 Bean 设置一个取反的 Condition
func (c *Configer) ConditionNot(cond Condition) *Configer {
	c.cond.OnConditionNot(cond)
	return c
}

// ConditionOnProperty 为 Bean 设置一个 PropertyCondition
func (c *Configer) ConditionOnProperty(name string) *Configer {
	c.cond.OnProperty(name)
	return c
}

// ConditionOnMissingProperty 为 Bean 设置一个 MissingPropertyCondition
func (c *Configer) ConditionOnMissingProperty(name string) *Configer {
	c.cond.OnMissingProperty(name)
	return c
}

// ConditionOnPropertyValue 为 Bean 设置一个 PropertyValueCondition
func (c *Configer) ConditionOnPropertyValue(name string, havingValue interface{}) *Configer {
	c.cond.OnPropertyValue(name, havingValue)
	return c
}

// ConditionOnBean 为 Bean 设置一个 BeanCondition
func (c *Configer) ConditionOnBean(selector interface{}) *Configer {
	c.cond.OnBean(selector)
	return c
}

// ConditionOnMissingBean 为 Bean 设置一个 MissingBeanCondition
func (c *Configer) ConditionOnMissingBean(selector interface{}) *Configer {
	c.cond.OnMissingBean(selector)
	return c
}

// ConditionOnExpression 为 Bean 设置一个 ExpressionCondition
func (c *Configer) ConditionOnExpression(expression string) *Configer {
	c.cond.OnExpression(expression)
	return c
}

// ConditionOnMatches 为 Bean 设置一个 FunctionCondition
func (c *Configer) ConditionOnMatches(fn ConditionFunc) *Configer {
	c.cond.OnMatches(fn)
	return c
}

// ConditionOnProfile 为 Bean 设置一个 ProfileCondition
func (c *Configer) ConditionOnProfile(profile string) *Configer {
	c.cond.OnProfile(profile)
	return c
}

// Matches 成功返回 true，失败返回 false
func (c *Configer) Matches(ctx SpringContext) bool {
	return c.cond.Matches(ctx)
}

// run 运行执行器
func (c *Configer) run(ctx *defaultSpringContext) {

	fnValue := reflect.ValueOf(c.fn)
	fnPtr := fnValue.Pointer()
	fnInfo := runtime.FuncForPC(fnPtr)
	file, line := fnInfo.FileLine(fnPtr)
	strCaller := fmt.Sprintf("%s:%d", file, line)

	a := newDefaultBeanAssembly(ctx, nil)
	cr := &defaultCaller{caller: strCaller}

	var in []reflect.Value

	if c.stringArg != nil {
		if v := c.stringArg.Get(a, cr); len(v) > 0 {
			in = append(in, v...)
		}
	}

	if c.optionArg != nil {
		if v := c.optionArg.Get(a, cr); len(v) > 0 {
			in = append(in, v...)
		}
	}

	reflect.ValueOf(c.fn).Call(in)
}
