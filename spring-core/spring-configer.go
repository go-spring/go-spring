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
	"container/list"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
)

// runnable 不带返回值的执行器
type runnable struct {
	fn        interface{}
	stringArg *fnStringBindingArg // 普通参数绑定
	optionArg *fnOptionBindingArg // Option 绑定

	withReceiver bool // 函数类型是否包含接收者
	receiver     reflect.Value
}

// run 运行执行器
func (r *runnable) run(assembly *defaultBeanAssembly) error {

	fnValue := reflect.ValueOf(r.fn)
	fnPtr := fnValue.Pointer()
	fnInfo := runtime.FuncForPC(fnPtr)
	file, line := fnInfo.FileLine(fnPtr)
	fileLine := fmt.Sprintf("%s:%d", file, line)

	var in []reflect.Value

	if r.withReceiver {
		in = append(in, r.receiver)
	}

	if r.stringArg != nil {
		if v := r.stringArg.Get(assembly, fileLine); len(v) > 0 {
			in = append(in, v...)
		}
	}

	if r.optionArg != nil {
		if v := r.optionArg.Get(assembly, fileLine); len(v) > 0 {
			in = append(in, v...)
		}
	}

	out := reflect.ValueOf(r.fn).Call(in)

	if n := len(out); n == 0 {
		return nil
	} else if n == 1 {
		if o := out[0]; o.Type() == errorType {
			if i := o.Interface(); i == nil {
				return nil
			} else {
				return i.(error)
			}
		}
	}

	panic(errors.New("error func type"))
}

// Configer 封装配置函数
type Configer struct {
	runnable
	name   string
	cond   *Conditional // 判断条件
	before []string
	after  []string
}

// newConfiger Configer 的构造函数
func newConfiger(name string, fn interface{}, tags []string) *Configer {

	fnType := reflect.TypeOf(fn)
	if fnType.Kind() != reflect.Func {
		panic(errors.New("fn must be a func"))
	}

	return &Configer{
		name: name,
		runnable: runnable{
			fn:        fn,
			stringArg: newFnStringBindingArg(fnType, false, tags),
		},
		cond: NewConditional(),
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
func (c *Configer) ConditionOnBean(selector BeanSelector) *Configer {
	c.cond.OnBean(selector)
	return c
}

// ConditionOnMissingBean 为 Bean 设置一个 MissingBeanCondition
func (c *Configer) ConditionOnMissingBean(selector BeanSelector) *Configer {
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

// checkCondition 成功返回 true，失败返回 false
func (c *Configer) checkCondition(ctx SpringContext) bool {
	return c.cond.Matches(ctx)
}

// Before 在某个 Configer 之前执行
func (c *Configer) Before(configers ...string) *Configer {
	c.before = configers
	return c
}

// After 在某个 Configer 之后执行
func (c *Configer) After(configers ...string) *Configer {
	c.after = configers
	return c
}

// sortConfigers 对 Configer 列表进行排序
func sortConfigers(configers *list.List) *list.List {
	toSort := list.New()
	toSort.PushBackList(configers)
	sorted := list.New()
	processing := list.New()
	for toSort.Len() > 0 {
		sortConfigersByAfter(configers, toSort, sorted, processing, nil)
	}
	return sorted
}

// sortConfigersByAfter 每次选出依赖联调最前端的元素
func sortConfigersByAfter(configers *list.List, toSort *list.List, sorted *list.List, processing *list.List, current *Configer) {
	if current == nil {
		current = (toSort.Remove(toSort.Front())).(*Configer)
	}
	processing.PushBack(current)
	for e := getBeforeConfigers(configers, current).Front(); e != nil; e = e.Next() {
		c := e.Value.(*Configer)

		// 自己不可能是自己前面的元素，除非出现了循环依赖
		for p := processing.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Configer); pc == c {
				// 打印循环依赖的路径
				sb := strings.Builder{}
				for t := p; t != nil; t = t.Next() {
					sb.WriteString(t.Value.(*Configer).name)
					sb.WriteString(" -> ")
				}
				sb.WriteString(pc.name)
				panic(fmt.Errorf("found cycle config: %s", sb.String()))
			}
		}

		inSorted := false
		for p := sorted.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Configer); pc == c {
				inSorted = true
				break
			}
		}

		inToSort := false
		for p := toSort.Front(); p != nil; p = p.Next() {
			if pc := p.Value.(*Configer); pc == c {
				inToSort = true
				break
			}
		}

		if !inSorted && inToSort { // 如果已经排好序了就不用管了
			sortConfigersByAfter(configers, toSort, sorted, processing, c)
		}
	}

	for p := processing.Front(); p != nil; p = p.Next() {
		if pc := p.Value.(*Configer); pc == current {
			processing.Remove(p)
			break
		}
	}

	for p := toSort.Front(); p != nil; p = p.Next() {
		if pc := p.Value.(*Configer); pc == current {
			toSort.Remove(p)
			break
		}
	}

	sorted.PushBack(current)
}

// getBeforeConfigers 获取当前 Configer 依赖的 Configer 列表
func getBeforeConfigers(configers *list.List, current *Configer) *list.List {
	result := list.New()
	for e := configers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*Configer)

		// 检查是否在当前 Configer 的前面
		for _, name := range c.before {
			if current.name == name {
				result.PushBack(c)
			}
		}

		// 检查是否在当前 Configer 的前面
		for _, name := range current.after {
			if c.name == name {
				result.PushBack(c)
			}
		}
	}
	return result
}
