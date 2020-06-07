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

// runnable 执行器，不能返回 error 以外的其他值
type runnable struct {
	fn        interface{}
	stringArg *fnStringBindingArg // 普通参数绑定
	optionArg *fnOptionBindingArg // Option 绑定

	withReceiver bool          // 函数是否包含接收者，也可以假装第一个参数是接收者
	receiver     reflect.Value // 接收者的值
}

// run 运行执行器
func (r *runnable) run(assembly *defaultBeanAssembly) error {

	// 获取函数定义所在的文件及其行号信息
	fnValue := reflect.ValueOf(r.fn)
	fnPtr := fnValue.Pointer()
	fnInfo := runtime.FuncForPC(fnPtr)
	file, line := fnInfo.FileLine(fnPtr)
	fileLine := fmt.Sprintf("%s:%d", file, line)

	// 组装 fn 调用所需的参数列表
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

	// 调用 fn 函数
	out := reflect.ValueOf(r.fn).Call(in)

	// 获取 error 返回值
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

// Configer 配置函数，不立即执行
type Configer struct {
	runnable
	name   string
	cond   *Conditional // 判断条件
	before []string     // 位于哪些配置函数之前
	after  []string     // 位于哪些配置函数之后
}

// newConfiger Configer 的构造函数，fn 不能返回 error 以外的其他值
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

// ConditionOn 为 Configer 设置一个 Condition
func (c *Configer) ConditionOn(cond Condition) *Configer {
	c.cond.OnCondition(cond)
	return c
}

// ConditionNot 为 Configer 设置一个取反的 Condition
func (c *Configer) ConditionNot(cond Condition) *Configer {
	c.cond.OnConditionNot(cond)
	return c
}

// ConditionOnProperty 为 Configer 设置一个 PropertyCondition
func (c *Configer) ConditionOnProperty(name string) *Configer {
	c.cond.OnProperty(name)
	return c
}

// ConditionOnMissingProperty 为 Configer 设置一个 MissingPropertyCondition
func (c *Configer) ConditionOnMissingProperty(name string) *Configer {
	c.cond.OnMissingProperty(name)
	return c
}

// ConditionOnPropertyValue 为 Configer 设置一个 PropertyValueCondition
func (c *Configer) ConditionOnPropertyValue(name string, havingValue interface{}) *Configer {
	c.cond.OnPropertyValue(name, havingValue)
	return c
}

// ConditionOnBean 为 Configer 设置一个 BeanCondition
func (c *Configer) ConditionOnBean(selector BeanSelector) *Configer {
	c.cond.OnBean(selector)
	return c
}

// ConditionOnMissingBean 为 Configer 设置一个 MissingBeanCondition
func (c *Configer) ConditionOnMissingBean(selector BeanSelector) *Configer {
	c.cond.OnMissingBean(selector)
	return c
}

// ConditionOnExpression 为 Configer 设置一个 ExpressionCondition
func (c *Configer) ConditionOnExpression(expression string) *Configer {
	c.cond.OnExpression(expression)
	return c
}

// ConditionOnMatches 为 Configer 设置一个 FunctionCondition
func (c *Configer) ConditionOnMatches(fn ConditionFunc) *Configer {
	c.cond.OnMatches(fn)
	return c
}

// ConditionOnProfile 为 Configer 设置一个 ProfileCondition
func (c *Configer) ConditionOnProfile(profile string) *Configer {
	c.cond.OnProfile(profile)
	return c
}

// checkCondition 成功返回 true，失败返回 false
func (c *Configer) checkCondition(ctx SpringContext) bool {
	return c.cond.Matches(ctx)
}

// Before 设置当前 Configer 在某些 Configer 之前执行
func (c *Configer) Before(configers ...string) *Configer {
	c.before = configers
	return c
}

// After 设置当前 Configer 在某些 Configer 之后执行
func (c *Configer) After(configers ...string) *Configer {
	c.after = configers
	return c
}

// sortConfigers 对 Configer 列表进行排序
func sortConfigers(configers *list.List) *list.List {

	// 待排列表
	toSort := list.New()
	toSort.PushBackList(configers)

	// 已排序列表
	sorted := list.New()

	// 正在处理的列表
	processing := list.New()

	for toSort.Len() > 0 { // 每次循环选出依赖链条最前端的元素
		sortConfigersByAfter(configers, toSort, sorted, processing, nil)
	}
	return sorted
}

// sortConfigersByAfter 选出依赖链条最前端的元素
func sortConfigersByAfter(configers *list.List, toSort *list.List, sorted *list.List, processing *list.List, current *Configer) {

	// 选出待排元素
	if current == nil {
		current = (toSort.Remove(toSort.Front())).(*Configer)
	}

	processing.PushBack(current)

	// 遍历当前 Configer 依赖的 Configer 列表
	for e := getBeforeConfigers(configers, current).Front(); e != nil; e = e.Next() {
		c := e.Value.(*Configer)

		// 自己不可能是自己前面的元素，除非出现了循环依赖，因此抛出 Panic
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

		if !inSorted && inToSort { // 递归处理当前 Configer 的依赖并进行排序
			sortConfigersByAfter(configers, toSort, sorted, processing, c)
		}
	}

	// 排序完成，从正在排序、待排序列表删除，然后添加到已排序列表
	{
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
