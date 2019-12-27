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
	"reflect"
	"strconv"
	"strings"
)

// fnBindingArg 存储函数的参数绑定
type fnBindingArg interface {
	// Get 获取函数参数的绑定值
	Get(ctx SpringContext, fnType reflect.Type) []reflect.Value
}

// fnStringBindingArg 存储一般函数的参数绑定
type fnStringBindingArg struct {
	fnTags [][]string
}

// newFnStringBindingArg fnStringBindingArg 的构造函数
func newFnStringBindingArg(fnType reflect.Type, tags []string) *fnStringBindingArg {

	fnTags := make([][]string, fnType.NumIn())
	variadic := fnType.IsVariadic() // 可变参数
	numIn := fnType.NumIn()

	if len(tags) > 0 {
		indexed := false // 是否包含序号

		if tag := tags[0]; tag != "" {
			if i := strings.Index(tag, ":"); i > 0 {
				_, err := strconv.Atoi(tag[:i])
				indexed = err == nil
			}
		}

		if indexed { // 有序号
			for _, tag := range tags {
				index := strings.Index(tag, ":")
				if index <= 0 {
					panic(errors.New("tag \"" + tag + "\" should have index"))
				}

				i, err := strconv.Atoi(tag[:index])
				if err != nil {
					panic(errors.New("tag \"" + tag + "\" should have index"))
				}

				if i < 0 || i >= numIn {
					panic(errors.New("error indexed tag \"" + tag + "\""))
				}

				if variadic && i == numIn-1 { // 处理可变参数
					fnTags[i] = append(fnTags[i], tag[index+1:])
				} else {
					fnTags[i] = []string{tag[index+1:]}
				}
			}

		} else { // 无序号
			for i, tag := range tags {
				if index := strings.Index(tag, ":"); index > 0 {
					_, err := strconv.Atoi(tag[:index])
					if err == nil {
						panic(errors.New("tag \"" + tag + "\" shouldn't have index"))
					}
				}

				if variadic && i >= numIn-1 { // 处理可变参数
					fnTags[numIn-1] = append(fnTags[numIn-1], tag)
				} else {
					fnTags[i] = []string{tag}
				}
			}
		}
	}

	return &fnStringBindingArg{fnTags}
}

// Get 获取函数参数的绑定值
func (ca *fnStringBindingArg) Get(ctx SpringContext, fnType reflect.Type) []reflect.Value {
	args := make([]reflect.Value, 0)

	f := func(it reflect.Type, tag string) {
		iv := reflect.New(it).Elem()

		if strings.HasPrefix(tag, "$") {
			bindStructField(ctx, it, iv, "", "", tag)
		} else {
			ctx.GetBeanValue(tag, iv, )
		}

		args = append(args, iv)
	}

	for i, tags := range ca.fnTags {
		it := fnType.In(i)
		if len(tags) == 0 {
			f(it, "")
		} else {
			for _, tag := range tags {
				f(it, tag)
			}
		}
	}

	return args
}

// fnOptionBindingArg 存储 Option 模式函数的参数绑定
type fnOptionBindingArg struct {
	options []*optionArg
}

// Get 获取函数参数的绑定值
func (ca *fnOptionBindingArg) Get(ctx SpringContext, _ reflect.Type) []reflect.Value {
	args := make([]reflect.Value, 0)
	for _, option := range ca.options {
		if arg := option.call(ctx); arg.IsValid() {
			args = append(args, arg)
		}
	}
	return args
}

// optionArg Option 函数的绑定参数
type optionArg struct {
	*Conditional // 判断条件

	fn  interface{}
	arg fnBindingArg
}

// NewOptionArg optionArg 的构造函数
func NewOptionArg(fn interface{}, tags ...string) *optionArg {
	fnType := reflect.TypeOf(fn)

	// 判断是否是合法的 Option 函数
	validOptionFunc := func() bool {

		// 必须是函数
		if fnType.Kind() != reflect.Func {
			return false
		}

		// 只能有一个返回值
		if fnType.NumOut() != 1 {
			return false
		}

		return true
	}

	if ok := validOptionFunc(); !ok {
		panic(errors.New("option func must be func(...)option"))
	}

	return &optionArg{
		Conditional: NewConditional(),
		fn:          fn,
		arg:         newFnStringBindingArg(fnType, tags),
	}
}

// Or c=a||b
func (arg *optionArg) Or() *optionArg {
	arg.Conditional.Or()
	return arg
}

// And c=a&&b
func (arg *optionArg) And() *optionArg {
	arg.Conditional.And()
	return arg
}

// ConditionOn 为 optionArg 设置一个 Condition
func (arg *optionArg) ConditionOn(cond Condition) *optionArg {
	arg.Conditional.OnCondition(cond)
	return arg
}

// ConditionOnProperty 为 optionArg 设置一个 PropertyCondition
func (arg *optionArg) ConditionOnProperty(name string) *optionArg {
	arg.Conditional.OnProperty(name)
	return arg
}

// ConditionOnMissingProperty 为 optionArg 设置一个 MissingPropertyCondition
func (arg *optionArg) ConditionOnMissingProperty(name string) *optionArg {
	arg.Conditional.OnMissingProperty(name)
	return arg
}

// ConditionOnPropertyValue 为 optionArg 设置一个 PropertyValueCondition
func (arg *optionArg) ConditionOnPropertyValue(name string, havingValue interface{}) *optionArg {
	arg.Conditional.OnPropertyValue(name, havingValue)
	return arg
}

// ConditionOnBean 为 optionArg 设置一个 BeanCondition
func (arg *optionArg) ConditionOnBean(beanId string) *optionArg {
	arg.Conditional.OnBean(beanId)
	return arg
}

// ConditionOnMissingBean 为 optionArg 设置一个 MissingBeanCondition
func (arg *optionArg) ConditionOnMissingBean(beanId string) *optionArg {
	arg.Conditional.OnMissingBean(beanId)
	return arg
}

// ConditionOnExpression 为 optionArg 设置一个 ExpressionCondition
func (arg *optionArg) ConditionOnExpression(expression string) *optionArg {
	arg.Conditional.OnExpression(expression)
	return arg
}

// ConditionOnMatches 为 optionArg 设置一个 FunctionCondition
func (arg *optionArg) ConditionOnMatches(fn ConditionFunc) *optionArg {
	arg.Conditional.OnMatches(fn)
	return arg
}

// OnProfile 设置一个 ProfileCondition
func (arg *optionArg) OnProfile(profile string) *optionArg {
	arg.Conditional.OnProperty(profile)
	return arg
}

// call 获取 optionArg 的运算值
func (arg *optionArg) call(ctx SpringContext) reflect.Value {

	// 判断 Option 条件是否成立
	if ok := arg.Conditional.Matches(ctx); !ok {
		return reflect.Value{}
	}

	fnValue := reflect.ValueOf(arg.fn)
	fnType := fnValue.Type()

	in := arg.arg.Get(ctx, fnType)
	out := fnValue.Call(in)
	return out[0]
}
