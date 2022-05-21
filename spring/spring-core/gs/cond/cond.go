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

//go:generate mockgen -build_flags="-mod=mod" -package=cond -source=cond.go -destination=cond_mock.go

// Package cond provides many conditions used when registering bean.
package cond

import (
	"errors"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"github.com/go-spring/spring-base/util"
	"github.com/go-spring/spring-core/conf"
	"github.com/go-spring/spring-core/gs/gsutil"
)

// Context defines some methods of IoC container that conditions use.
type Context interface {
	// Has returns whether the IoC container has a property.
	Has(key string) bool
	// Prop returns the property's value when the IoC container has it, or
	// returns empty string when the IoC container doesn't have it.
	Prop(key string, opts ...conf.GetOption) string
	// Find returns bean definitions that matched with the bean selector.
	Find(selector gsutil.BeanSelector) ([]gsutil.BeanDefinition, error)
}

// Condition is used when registering a bean to determine whether it's valid.
type Condition interface {
	Matches(ctx Context) (bool, error)
}

type FuncCond func(ctx Context) (bool, error)

func (c FuncCond) Matches(ctx Context) (bool, error) {
	return c(ctx)
}

// OK returns a Condition that always returns true.
func OK() Condition {
	return FuncCond(func(ctx Context) (bool, error) {
		return true, nil
	})
}

// not is a Condition that negating to another.
type not struct {
	c Condition
}

// Not returns a Condition that negating to another.
func Not(c Condition) Condition {
	return &not{c: c}
}

func (c *not) Matches(ctx Context) (bool, error) {
	ok, err := c.c.Matches(ctx)
	return !ok, err
}

// onProperty is a Condition that checks a property and its value.
type onProperty struct {
	name           string
	havingValue    string
	matchIfMissing bool
}

func (c *onProperty) Matches(ctx Context) (bool, error) {

	if !ctx.Has(c.name) {
		return c.matchIfMissing, nil
	}

	if c.havingValue == "" {
		return true, nil
	}

	val := ctx.Prop(c.name)
	if !strings.HasPrefix(c.havingValue, "go:") {
		return val == c.havingValue, nil
	}

	var expr string
	if _, err := strconv.ParseBool(val); err == nil {
		expr = strings.ReplaceAll(c.havingValue[3:], "$", val)
	} else if _, err = strconv.ParseFloat(val, 64); err == nil {
		expr = strings.ReplaceAll(c.havingValue[3:], "$", val)
	} else {
		expr = strings.ReplaceAll(c.havingValue[3:], "$", strconv.Quote(val))
	}

	ret, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expr)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(ret.Value.String())
}

// onMissingProperty is a Condition that returns true when a property doesn't exist.
type onMissingProperty struct {
	name string
}

func (c *onMissingProperty) Matches(ctx Context) (bool, error) {
	return !ctx.Has(c.name), nil
}

// onBean is a Condition that returns true when finding more than one beans.
type onBean struct {
	selector gsutil.BeanSelector
}

func (c *onBean) Matches(ctx Context) (bool, error) {
	beans, err := ctx.Find(c.selector)
	return len(beans) > 0, err
}

// onMissingBean is a Condition that returns true when finding no beans.
type onMissingBean struct {
	selector gsutil.BeanSelector
}

func (c *onMissingBean) Matches(ctx Context) (bool, error) {
	beans, err := ctx.Find(c.selector)
	return len(beans) == 0, err
}

// onSingleBean is a Condition that returns true when finding only one bean.
type onSingleBean struct {
	selector gsutil.BeanSelector
}

func (c *onSingleBean) Matches(ctx Context) (bool, error) {
	beans, err := ctx.Find(c.selector)
	return len(beans) == 1, err
}

// onExpression is a Condition that returns true when an expression is true.
type onExpression struct {
	expression string
}

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

// OnProperty returns a Condition that checks a property and its value.
func OnProperty(name string, options ...PropertyOption) *conditional {
	return New().OnProperty(name, options...)
}

// OnProperty adds a Condition that checks a property and its value.
func (c *conditional) OnProperty(name string, options ...PropertyOption) *conditional {
	cond := &onProperty{name: name}
	for _, option := range options {
		option(cond)
	}
	return c.On(cond)
}

// OnMissingProperty returns a Condition that returns true when a property doesn't exist.
func OnMissingProperty(name string) *conditional {
	return New().OnMissingProperty(name)
}

// OnMissingProperty adds a Condition that returns true when a property doesn't exist.
func (c *conditional) OnMissingProperty(name string) *conditional {
	return c.On(&onMissingProperty{name: name})
}

// OnBean returns a Condition that returns true when finding more than one beans.
func OnBean(selector gsutil.BeanSelector) *conditional {
	return New().OnBean(selector)
}

// OnBean adds a Condition that returns true when finding more than one beans.
func (c *conditional) OnBean(selector gsutil.BeanSelector) *conditional {
	return c.On(&onBean{selector: selector})
}

// OnMissingBean returns a Condition that returns true when finding no beans.
func OnMissingBean(selector gsutil.BeanSelector) *conditional {
	return New().OnMissingBean(selector)
}

// OnMissingBean adds a Condition that returns true when finding no beans.
func (c *conditional) OnMissingBean(selector gsutil.BeanSelector) *conditional {
	return c.On(&onMissingBean{selector: selector})
}

// OnSingleBean returns a Condition that returns true when finding only one bean.
func OnSingleBean(selector gsutil.BeanSelector) *conditional {
	return New().OnSingleBean(selector)
}

// OnSingleBean adds a Condition that returns true when finding only one bean.
func (c *conditional) OnSingleBean(selector gsutil.BeanSelector) *conditional {
	return c.On(&onSingleBean{selector: selector})
}

// OnExpression returns a Condition that returns true when an expression is true.
func OnExpression(expression string) *conditional {
	return New().OnExpression(expression)
}

// OnExpression adds a Condition that returns true when an expression is true.
func (c *conditional) OnExpression(expression string) *conditional {
	return c.On(&onExpression{expression: expression})
}

// OnMatches 返回一个以 onMatches 为开始条件的计算式。
func OnMatches(fn func(ctx Context) (bool, error)) *conditional {
	return New().OnMatches(fn)
}

// OnMatches 添加一个 onMatches 条件。
func (c *conditional) OnMatches(fn func(ctx Context) (bool, error)) *conditional {
	return c.On(FuncCond(fn))
}

// OnProfile 返回一个以 spring.profile 属性值是否匹配为开始条件的计算式。
func OnProfile(profile string) *conditional {
	return New().OnProfile(profile)
}

// OnProfile 添加一个 spring.profile 属性值是否匹配的条件。
func (c *conditional) OnProfile(profile string) *conditional {
	return c.OnProperty("spring.profiles.active", HavingValue(profile))
}
