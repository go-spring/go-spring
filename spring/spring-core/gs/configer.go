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

package gs

import (
	"container/list"
	"errors"
	"reflect"

	"github.com/go-spring/spring-core/arg"
	"github.com/go-spring/spring-core/cond"
	"github.com/go-spring/spring-core/util"
)

// Configer 配置函数，所谓配置函数是指可以接受一些 Bean 作为入参的函数，使用场景大多
// 是在 Bean 初始化之后对 Bean 进行二次配置，可以作为框架配置能力的补充，但是要慎用！
type Configer struct {
	fn     *arg.Callable
	name   string
	cond   cond.Condition
	before []string // 位于哪些配置函数之前
	after  []string // 位于哪些配置函数之后
}

// config Configer 的构造函数，fn 只能返回 error 或者没有返回。
func config(fn interface{}, args ...arg.Arg) *Configer {
	t := reflect.TypeOf(fn)
	if util.FuncType(t) {
		if util.ReturnNothing(t) || util.ReturnOnlyError(t) {
			return &Configer{
				fn: arg.Bind(fn, false, args),
			}
		}
	}
	panic(errors.New("fn should be func() or func()error"))
}

// WithName 为 Configer 设置一个名称。
func (c *Configer) WithName(name string) *Configer {
	c.name = name
	return c
}

// WithCond 为 Configer 设置一个 Condition。
func (c *Configer) WithCond(cond cond.Condition) *Configer {
	c.cond = cond
	return c
}

// Before 设置当前 Configer 在哪些 Configer 之前执行。
func (c *Configer) Before(configers ...string) *Configer {
	c.before = append(c.before, configers...)
	return c
}

// After 设置当前 Configer 在哪些 Configer 之后执行。
func (c *Configer) After(configers ...string) *Configer {
	c.after = append(c.after, configers...)
	return c
}

// getBeforeConfigers 获取 i 之前的 Configer 列表，用于 internal/sort 排序。
func getBeforeConfigers(configers *list.List, i interface{}) *list.List {

	result := list.New()
	current := i.(*Configer)
	for e := configers.Front(); e != nil; e = e.Next() {
		c := e.Value.(*Configer)

		// 检查 c 是否在 current 的前面
		for _, name := range c.before {
			if current.name == name {
				result.PushBack(c)
			}
		}

		// 检查 current 是否在 c 的后面
		for _, name := range current.after {
			if c.name == name {
				result.PushBack(c)
			}
		}
	}
	return result
}
