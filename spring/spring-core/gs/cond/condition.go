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

// Package cond 提供了判断 bean 注册是否有效的条件。
package cond

import (
	"errors"
	"go/token"
	"go/types"
	"strings"

	"github.com/go-spring/spring-boost/cast"
	"github.com/go-spring/spring-boost/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/bean"
)

// SpringProfilesActive 当前应用的 profile 配置。
const SpringProfilesActive = "spring.profiles.active"

// Context IoC 容器对 cond 模块提供的最小功能集。
type Context interface {

	// Prop 获取 key 对应的属性值，注意 key 是大小写敏感的。当 key 对应的属性
	// 值存在时，或者 key 对应的属性值不存在但设置了默认值时，该方法返回 string
	// 类型的数据，当 key 对应的属性值不存在且没有设置默认值时该方法返回 nil。
	// 因此可以通过判断该方法的返回值是否为 nil 来判断 key 对应的属性值是否存在。
	Prop(key string, opts ...conf.GetOption) interface{}

	// Find 查找符合条件的 bean 对象，注意该函数只能保证返回的 bean 是有效的,
	// 即未被标记为删除的，而不能保证已经完成属性绑定和依赖注入。
	Find(selector bean.Selector) ([]bean.Definition, error)
}

// Condition 条件接口，条件成立 Matches 方法返回 true，否则返回 false。
type Condition interface {
	Matches(ctx Context) (bool, error)
}

type Matches func(ctx Context) (bool, error)

// onMatches 基于 Matches 方法的 Condition 实现。
type onMatches struct{ fn Matches }

func (c *onMatches) Matches(ctx Context) (bool, error) {
	return c.fn(ctx)
}

// not 对一个条件进行取反的 Condition 实现。
type not struct{ c Condition }

// Not 返回对一个条件取反后的 Condition 对象。
func Not(c Condition) *not {
	return &not{c: c}
}

func (c *not) Matches(ctx Context) (bool, error) {
	ok, err := c.c.Matches(ctx)
	return !ok, err
}

// onProperty 基于属性值匹配的 Condition 实现。
type onProperty struct {
	name           string
	havingValue    string
	matchIfMissing bool
}

func (c *onProperty) Matches(ctx Context) (bool, error) {
	// 参考 /usr/local/go/src/go/types/eval_test.go 示例

	val := ctx.Prop(c.name)
	if val == nil {
		return c.matchIfMissing, nil
	}

	if !strings.Contains(c.havingValue, "$") {
		return val == c.havingValue, nil
	}

	expr := strings.Replace(c.havingValue, "$", cast.ToString(val), -1)
	ret, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expr)
	if err != nil {
		return false, err
	}

	return cast.ToBoolE(ret.Value.String())
}

// onBean 基于符合条件的 bean 必须存在的 Condition 实现。
type onBean struct{ selector bean.Selector }

func (c *onBean) Matches(ctx Context) (bool, error) {
	beans, err := ctx.Find(c.selector)
	return len(beans) > 0, err
}

// onMissingBean 基于符合条件的 bean 必须不存在的 Condition 实现。
type onMissingBean struct{ selector bean.Selector }

func (c *onMissingBean) Matches(ctx Context) (bool, error) {
	beans, err := ctx.Find(c.selector)
	return len(beans) == 0, err
}

// onSingleCandidate 基于符合条件的 bean 只有一个的 Condition 实现。
type onSingleCandidate struct{ selector bean.Selector }

func (c *onSingleCandidate) Matches(ctx Context) (bool, error) {
	beans, err := ctx.Find(c.selector)
	return len(beans) == 1, err
}

// onExpression 基于表达式的 Condition 实现。
type onExpression struct{ expression string }

func (c *onExpression) Matches(ctx Context) (bool, error) {
	return false, util.UnimplementedMethod
}

// Operator 条件操作符，包含 Or、And、None 三种。
type Operator int

const (
	Or   = Operator(1) // 条件成立必须至少一个满足。
	And  = Operator(2) // 条件成立必须所有都要满足。
	None = Operator(3) // 条件成立必须没有一个满足。
)

// group 基于条件组的 Condition 实现。
type group struct {
	op   Operator
	cond []Condition
}

// Group 返回基于条件组的 Condition 对象。
func Group(op Operator, cond ...Condition) *group {
	return &group{op: op, cond: cond}
}

func (g *group) Matches(ctx Context) (bool, error) {

	if len(g.cond) == 0 {
		return false, errors.New("no condition in group")
	}

	switch g.op {
	case Or:
		for _, c := range g.cond {
			if ok, err := c.Matches(ctx); err != nil {
				return false, err
			} else if ok {
				return true, nil
			}
		}
		return false, nil
	case And:
		for _, c := range g.cond {
			if ok, err := c.Matches(ctx); err != nil {
				return false, err
			} else if !ok {
				return false, nil
			}
		}
		return true, nil
	case None:
		for _, c := range g.cond {
			if ok, err := c.Matches(ctx); err != nil {
				return false, err
			} else if ok {
				return false, nil
			}
		}
		return true, nil
	}

	return false, errors.New("error condition operator")
}

// node 基于条件链的 Condition 实现。
type node struct {
	cond Condition // 条件
	op   Operator  // 操作符
	next *node     // 下一个节点
}

func (n *node) Matches(ctx Context) (bool, error) {

	if n.cond == nil { // 空节点返回 true
		return true, nil
	}

	ok, err := n.cond.Matches(ctx)
	if err != nil {
		return false, err
	}

	if n.next == nil {
		return ok, nil
	} else if n.next.cond == nil {
		return false, errors.New("no condition in last node")
	}

	switch n.op {
	case Or:
		if ok {
			return ok, nil
		} else {
			return n.next.Matches(ctx)
		}
	case And:
		if ok {
			return n.next.Matches(ctx)
		} else {
			return false, nil
		}
	}

	return false, errors.New("error condition operator")
}

// conditional Condition 计算式。
type conditional struct {
	head *node
	curr *node
}

// New 返回 Condition 计算式对象。
func New() *conditional {
	n := &node{}
	return &conditional{head: n, curr: n}
}

func (c *conditional) Matches(ctx Context) (bool, error) {
	return c.head.Matches(ctx)
}

// Or 添加一个 or 操作符。
func (c *conditional) Or() *conditional {
	n := &node{}
	c.curr.op = Or
	c.curr.next = n
	c.curr = n
	return c
}

// And 添加一个 and 操作符。
func (c *conditional) And() *conditional {
	n := &node{}
	c.curr.op = And
	c.curr.next = n
	c.curr = n
	return c
}

// On 返回一个以给定 Condition 为开始条件的计算式。
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

type PropertyOption func(*onProperty)

// MatchIfMissing 当属性不存在时条件成立。
func MatchIfMissing() PropertyOption {
	return func(c *onProperty) {
		c.matchIfMissing = true
	}
}

// HavingValue 当 havingValue 与属性值相同时条件成立。
func HavingValue(havingValue string) PropertyOption {
	return func(c *onProperty) {
		c.havingValue = havingValue
	}
}

// OnProperty 返回一个以 onProperty 为开始条件的计算式。
func OnProperty(name string, options ...PropertyOption) *conditional {
	return New().OnProperty(name, options...)
}

// OnProperty 添加一个 onProperty 条件。
func (c *conditional) OnProperty(name string, options ...PropertyOption) *conditional {
	cond := &onProperty{name: name}
	for _, option := range options {
		option(cond)
	}
	return c.On(cond)
}

// OnBean 返回一个以 onBean 为开始条件的计算式。
func OnBean(selector bean.Selector) *conditional {
	return New().OnBean(selector)
}

// OnBean 添加一个 onBean 条件。
func (c *conditional) OnBean(selector bean.Selector) *conditional {
	return c.On(&onBean{selector: selector})
}

// OnMissingBean 返回一个以 onMissingBean 为开始条件的计算式。
func OnMissingBean(selector bean.Selector) *conditional {
	return New().OnMissingBean(selector)
}

// OnMissingBean 添加一个 onMissingBean 条件。
func (c *conditional) OnMissingBean(selector bean.Selector) *conditional {
	return c.On(&onMissingBean{selector: selector})
}

// OnSingleCandidate 返回一个以 onSingleCandidate 为开始条件的计算式。
func OnSingleCandidate(selector bean.Selector) *conditional {
	return New().OnSingleCandidate(selector)
}

// OnSingleCandidate 添加一个 onMissingBean 条件。
func (c *conditional) OnSingleCandidate(selector bean.Selector) *conditional {
	return c.On(&onSingleCandidate{selector: selector})
}

// OnExpression 返回一个以 onExpression 为开始条件的计算式。
func OnExpression(expression string) *conditional {
	return New().OnExpression(expression)
}

// OnExpression 添加一个 onExpression 条件。
func (c *conditional) OnExpression(expression string) *conditional {
	return c.On(&onExpression{expression: expression})
}

// OnMatches 返回一个以 onMatches 为开始条件的计算式。
func OnMatches(fn Matches) *conditional {
	return New().OnMatches(fn)
}

// OnMatches 添加一个 onMatches 条件。
func (c *conditional) OnMatches(fn Matches) *conditional {
	return c.On(&onMatches{fn: fn})
}

// OnProfile 返回一个以 spring.profile 属性值是否匹配为开始条件的计算式。
func OnProfile(profile string) *conditional {
	return New().OnProfile(profile)
}

// OnProfile 添加一个 spring.profile 属性值是否匹配的条件。
func (c *conditional) OnProfile(profile string) *conditional {
	return c.OnProperty(SpringProfilesActive, HavingValue(profile))
}
