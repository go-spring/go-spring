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

// Package cond 实现了多种条件。
package cond

import (
	"errors"
	"go/token"
	"go/types"
	"strings"

	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

// Context IoC 容器对 cond 模块提供的最小功能集。
type Context interface {

	// Prop 返回 key 转为小写后精确匹配的属性值，不存在返回 nil。
	Prop(key string, opts ...conf.GetOption) interface{}

	// Find 返回符合条件的 Bean 集合，不保证返回的 Bean 已经完成注入和绑定过程。
	Find(selector bean.Selector) ([]bean.Definition, error)
}

// Condition 定义条件接口，条件成立 Matches 函数返回 true，否则返回 false。
type Condition interface {
	Matches(ctx Context) bool
}

type Matches func(ctx Context) bool

// onMatches 基于 Matches 方法的 Condition 实现。
type onMatches struct{ fn Matches }

func (c *onMatches) Matches(ctx Context) bool {
	return c.fn(ctx)
}

// not 对一个条件进行取反的 Condition 实现。
type not struct{ c Condition }

// Not 对一个条件进行取反。
func Not(c Condition) *not {
	return &not{c: c}
}

func (c *not) Matches(ctx Context) bool {
	return !c.c.Matches(ctx)
}

// onProperty 基于属性值存在的 Condition 实现。
type onProperty struct{ name string }

func (c *onProperty) Matches(ctx Context) bool {
	return ctx.Prop(c.name) != nil
}

// onMissingProperty 基于属性值不存在的 Condition 实现。
type onMissingProperty struct{ name string }

func (c *onMissingProperty) Matches(ctx Context) bool {
	return ctx.Prop(c.name) == nil
}

// onPropertyValue 基于属性值匹配的 Condition 实现。
type onPropertyValue struct {
	name           string
	havingValue    interface{}
	matchIfMissing bool
}

type PropertyValueOption func(*onPropertyValue)

// MatchIfMissing 当属性值不存在时是否匹配条件
func MatchIfMissing(matchIfMissing bool) PropertyValueOption {
	return func(c *onPropertyValue) {
		c.matchIfMissing = matchIfMissing
	}
}

func (c *onPropertyValue) Matches(ctx Context) bool {
	// 参考 /usr/local/go/src/go/types/eval_test.go 示例

	val := ctx.Prop(c.name)
	if val == nil { // 不存在返回默认值
		return c.matchIfMissing
	}

	// 不是字符串则直接比较
	expectValue, ok := c.havingValue.(string)
	if !ok {
		return val == c.havingValue
	}

	// 字符串不是表达式的话则直接比较
	if ok = strings.Contains(expectValue, "$"); !ok {
		return val == expectValue
	}

	expr := strings.Replace(expectValue, "$", cast.ToString(val), -1)
	if ret, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expr); err == nil {
		return ret.Value.String() == "true"
	} else {
		panic(err)
	}
}

// onBean 基于 Bean 存在的 Condition 实现。
type onBean struct{ selector bean.Selector }

func (c *onBean) Matches(ctx Context) bool {
	b, _ := ctx.Find(c.selector)
	return len(b) == 1
}

// onMissingBean 基于 Bean 不能存在的 Condition 实现。
type onMissingBean struct{ selector bean.Selector }

func (c *onMissingBean) Matches(ctx Context) bool {
	b, _ := ctx.Find(c.selector)
	return len(b) == 0
}

// onExpression 基于表达式的 Condition 实现。
type onExpression struct{ expression string }

func (c *onExpression) Matches(ctx Context) bool {
	panic(util.UnimplementedMethod)
}

// Operator 条件操作符，包含 Or、And、None 三种。
type Operator int

const (
	Or   = Operator(1) // 至少一个满足
	And  = Operator(2) // 所有都要满足
	None = Operator(3) // 没有一个满足
)

// group 基于条件组的 Condition 实现。
type group struct {
	op   Operator
	cond []Condition
}

// Group group 的构造函数。
func Group(op Operator, cond ...Condition) *group {
	return &group{op: op, cond: cond}
}

func (g *group) Matches(ctx Context) bool {

	if len(g.cond) == 0 {
		panic(errors.New("no condition in group"))
	}

	switch g.op {
	case Or:
		for _, c := range g.cond {
			if c.Matches(ctx) {
				return true
			}
		}
		return false
	case And:
		for _, c := range g.cond {
			if ok := c.Matches(ctx); !ok {
				return false
			}
		}
		return true
	case None:
		for _, c := range g.cond {
			if c.Matches(ctx) {
				return false
			}
		}
		return true
	}

	panic(errors.New("error condition operator"))
}

// node 基于条件表达式的 Condition 实现。
type node struct {
	cond Condition // 条件
	op   Operator  // 操作符
	next *node     // 下个表达式节点
}

func (n *node) Matches(ctx Context) bool {

	if n.cond == nil { // 空节点返回 true
		return true
	}

	r := n.cond.Matches(ctx)

	if n.next == nil {
		return r
	} else if n.next.cond == nil {
		panic(errors.New("no condition in last node"))
	}

	switch n.op {
	case Or: // or
		if r {
			return r
		} else {
			return n.next.Matches(ctx)
		}
	case And: // and
		if r {
			return n.next.Matches(ctx)
		} else {
			return false
		}
	default:
		panic(errors.New("error condition operator"))
	}
}

// conditional Condition 计算式
type conditional struct {
	head *node
	curr *node
}

// New conditional 的构造函数
func New() *conditional {
	n := &node{}
	return &conditional{head: n, curr: n}
}

func (c *conditional) Matches(ctx Context) bool {
	return c.head.Matches(ctx)
}

// Or c=a||b
func (c *conditional) Or() *conditional {
	n := &node{}
	c.curr.op = Or
	c.curr.next = n
	c.curr = n
	return c
}

// And c=a&&b
func (c *conditional) And() *conditional {
	n := &node{}
	c.curr.op = And
	c.curr.next = n
	c.curr = n
	return c
}

// On 返回一个条件。
func On(cond Condition) *conditional {
	return New().On(cond)
}

// On 添加一个条件。
func (c *conditional) On(cond Condition) *conditional {
	if c.curr.cond != nil {
		c.And()
	}
	c.curr.cond = cond
	return c
}

// OnProperty 返回一个 onProperty 条件。
func OnProperty(name string) *conditional {
	return New().OnProperty(name)
}

// OnProperty 添加一个 onProperty 条件。
func (c *conditional) OnProperty(name string) *conditional {
	return c.On(&onProperty{name: name})
}

// OnMissingProperty 返回一个 onMissingProperty  条件。
func OnMissingProperty(name string) *conditional {
	return New().OnMissingProperty(name)
}

// OnMissingProperty 添加一个 onMissingProperty 条件。
func (c *conditional) OnMissingProperty(name string) *conditional {
	return c.On(&onMissingProperty{name: name})
}

// OnPropertyValue 返回一个 onPropertyValue 条件。
func OnPropertyValue(name string, havingValue interface{}, options ...PropertyValueOption) *conditional {
	return New().OnPropertyValue(name, havingValue, options...)
}

// OnPropertyValue 添加一个 onPropertyValue 条件。
func (c *conditional) OnPropertyValue(name string, havingValue interface{}, options ...PropertyValueOption) *conditional {
	cond := &onPropertyValue{name: name, havingValue: havingValue}
	for _, option := range options {
		option(cond)
	}
	return c.On(cond)
}

// OnOptionalPropertyValue 返回一个 onPropertyValue 条件，并且当属性值不存在时默认条件成立。
func OnOptionalPropertyValue(name string, havingValue interface{}) *conditional {
	return New().OnOptionalPropertyValue(name, havingValue)
}

// OnOptionalPropertyValue 添加一个 onPropertyValue 条件，并且当属性值不存在时默认条件成立。
func (c *conditional) OnOptionalPropertyValue(name string, havingValue interface{}) *conditional {
	return c.OnPropertyValue(name, havingValue, MatchIfMissing(true))
}

// OnBean 返回一个 onBean 条件。
func OnBean(selector bean.Selector) *conditional {
	return New().OnBean(selector)
}

// OnBean 添加一个 onBean 条件。
func (c *conditional) OnBean(selector bean.Selector) *conditional {
	return c.On(&onBean{selector: selector})
}

// OnMissingBean 返回一个 onMissingBean 条件。
func OnMissingBean(selector bean.Selector) *conditional {
	return New().OnMissingBean(selector)
}

// OnMissingBean 添加一个 onMissingBean 条件。
func (c *conditional) OnMissingBean(selector bean.Selector) *conditional {
	return c.On(&onMissingBean{selector: selector})
}

// OnExpression 返回一个 onExpression 条件。
func OnExpression(expression string) *conditional {
	return New().OnExpression(expression)
}

// OnExpression 添加一个 onExpression 条件。
func (c *conditional) OnExpression(expression string) *conditional {
	return c.On(&onExpression{expression: expression})
}

// OnMatches 返回一个 onMatches 条件。
func OnMatches(fn Matches) *conditional {
	return New().OnMatches(fn)
}

// OnMatches 添加一个 onMatches 条件。
func (c *conditional) OnMatches(fn Matches) *conditional {
	return c.On(&onMatches{fn: fn})
}

// OnProfile 返回一个 onProfile 条件。
func OnProfile(profile string) *conditional {
	return New().OnProfile(profile)
}

// OnProfile 添加一个 onProfile 条件。
func (c *conditional) OnProfile(profile string) *conditional {
	return c.On(&onPropertyValue{name: conf.SpringProfile, havingValue: profile})
}
