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

package cond

import (
	"errors"
	"go/token"
	"go/types"
	"strings"

	"github.com/go-spring/spring-core/bean"
	"github.com/go-spring/spring-core/util"
	"github.com/spf13/cast"
)

// ConditionFunc 定义 Condition 接口 Matches 方法的类型
type ConditionFunc func(ctx bean.ConditionContext) bool

// functionCondition 基于 Matches 方法的 Condition 实现
type functionCondition struct {
	fn ConditionFunc
}

// FunctionCondition functionCondition 的构造函数
func FunctionCondition(fn ConditionFunc) *functionCondition {
	return &functionCondition{fn}
}

// Matches 成功返回 true，失败返回 false
func (c *functionCondition) Matches(ctx bean.ConditionContext) bool {
	return c.fn(ctx)
}

// notCondition 对 Condition 取反的 Condition 实现
type notCondition struct {
	cond bean.Condition
}

// NotCondition notCondition 的构造函数
func NotCondition(cond bean.Condition) *notCondition {
	return &notCondition{cond}
}

// Matches 成功返回 true，失败返回 false
func (c *notCondition) Matches(ctx bean.ConditionContext) bool {
	return !c.cond.Matches(ctx)
}

// propertyCondition 基于属性值存在的 Condition 实现
type propertyCondition struct {
	name string
}

// PropertyCondition propertyCondition 的构造函数
func PropertyCondition(name string) *propertyCondition {
	return &propertyCondition{name}
}

// Matches 成功返回 true，失败返回 false
func (c *propertyCondition) Matches(ctx bean.ConditionContext) bool {
	return len(ctx.Properties().Prefix(c.name)) > 0
}

// missingPropertyCondition 基于属性值不存在的 Condition 实现
type missingPropertyCondition struct {
	name string
}

// MissingPropertyCondition missingPropertyCondition 的构造函数
func MissingPropertyCondition(name string) *missingPropertyCondition {
	return &missingPropertyCondition{name}
}

// Matches 成功返回 true，失败返回 false
func (c *missingPropertyCondition) Matches(ctx bean.ConditionContext) bool {
	return len(ctx.Properties().Prefix(c.name)) == 0
}

// propertyValueCondition 基于属性值匹配的 Condition 实现
type propertyValueCondition struct {
	name           string
	havingValue    interface{}
	matchIfMissing bool
}

type PropertyValueConditionOption func(*propertyValueCondition)

// MatchIfMissing 当属性值不存在时是否匹配判断条件
func MatchIfMissing(matchIfMissing bool) PropertyValueConditionOption {
	return func(cond *propertyValueCondition) {
		cond.matchIfMissing = matchIfMissing
	}
}

// PropertyValueCondition propertyValueCondition 的构造函数
func PropertyValueCondition(name string, havingValue interface{},
	options ...PropertyValueConditionOption) *propertyValueCondition {

	cond := &propertyValueCondition{name: name, havingValue: havingValue}
	for _, option := range options {
		option(cond)
	}
	return cond
}

// Matches 成功返回 true，失败返回 false
func (c *propertyValueCondition) Matches(ctx bean.ConditionContext) bool {
	// 参考 /usr/local/go/src/go/types/eval_test.go 示例

	val := ctx.Properties().Get(c.name)
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

// beanCondition 基于 Bean 存在的 Condition 实现
type beanCondition struct {
	selector bean.BeanSelector
}

// BeanCondition beanCondition 的构造函数
func BeanCondition(selector bean.BeanSelector) *beanCondition {
	return &beanCondition{selector}
}

// Matches 成功返回 true，失败返回 false
func (c *beanCondition) Matches(ctx bean.ConditionContext) bool {
	_, ok := ctx.FindBean(c.selector)
	return ok
}

// missingBeanCondition 基于 Bean 不能存在的 Condition 实现
type missingBeanCondition struct {
	selector bean.BeanSelector
}

// MissingBeanCondition missingBeanCondition 的构造函数
func MissingBeanCondition(selector bean.BeanSelector) *missingBeanCondition {
	return &missingBeanCondition{selector}
}

// Matches 成功返回 true，失败返回 false
func (c *missingBeanCondition) Matches(ctx bean.ConditionContext) bool {
	_, ok := ctx.FindBean(c.selector)
	return !ok
}

// expressionCondition 基于表达式的 Condition 实现
type expressionCondition struct {
	expression string
}

// ExpressionCondition expressionCondition 的构造函数
func ExpressionCondition(expression string) *expressionCondition {
	return &expressionCondition{expression}
}

// Matches 成功返回 true，失败返回 false
func (c *expressionCondition) Matches(ctx bean.ConditionContext) bool {
	panic(util.UnimplementedMethod)
}

// profileCondition 基于运行环境匹配的 Condition 实现
type profileCondition struct {
	profile string
}

// ProfileCondition profileCondition 的构造函数
func ProfileCondition(profile string) *profileCondition {
	return &profileCondition{profile}
}

// Matches 成功返回 true，失败返回 false
func (c *profileCondition) Matches(ctx bean.ConditionContext) bool {
	return c.profile == "" || strings.EqualFold(c.profile, ctx.GetProfile())
}

// ConditionOp conditionNode 的计算方式
type ConditionOp int

const (
	ConditionOr   = ConditionOp(1) // 至少一个满足
	ConditionAnd  = ConditionOp(2) // 所有都要满足
	ConditionNone = ConditionOp(3) // 没有一个满足
)

// conditionGroup 基于条件组的 Condition 实现
type conditionGroup struct {
	op   ConditionOp
	cond []bean.Condition
}

// ConditionGroup conditions 的构造函数
func ConditionGroup(op ConditionOp, cond ...bean.Condition) *conditionGroup {
	return &conditionGroup{
		op:   op,
		cond: cond,
	}
}

// Matches 成功返回 true，失败返回 false
func (c *conditionGroup) Matches(ctx bean.ConditionContext) bool {

	if len(c.cond) == 0 {
		panic(errors.New("no condition"))
	}

	switch c.op {
	case ConditionOr:
		for _, c0 := range c.cond {
			if c0.Matches(ctx) {
				return true
			}
		}
		return false
	case ConditionAnd:
		for _, c0 := range c.cond {
			if ok := c0.Matches(ctx); !ok {
				return false
			}
		}
		return true
	case ConditionNone:
		for _, c0 := range c.cond {
			if c0.Matches(ctx) {
				return false
			}
		}
		return true
	}

	panic(errors.New("error condition op mode"))
}

// conditionNode Condition 计算式节点，返回值是 'cond op next'
type conditionNode struct {
	cond bean.Condition // 条件
	op   ConditionOp    // 计算方式
	next *conditionNode // 下一个计算节点
}

// Matches 成功返回 true，失败返回 false
func (c *conditionNode) Matches(ctx bean.ConditionContext) bool {

	if c.cond == nil { // 空节点返回 true
		return true
	}

	if c.next != nil && c.next.cond == nil {
		panic(errors.New("last op need a cond triggered"))
	}

	if r := c.cond.Matches(ctx); c.next != nil {

		switch c.op {
		case ConditionOr: // or
			if r {
				return r
			} else {
				return c.next.Matches(ctx)
			}
		case ConditionAnd: // and
			if r {
				return c.next.Matches(ctx)
			} else {
				return false
			}
		default:
			panic(errors.New("error condition op mode"))
		}

	} else {
		return r
	}
}

// Conditional Condition 计算式
type Conditional struct {
	head *conditionNode
	curr *conditionNode
}

// conditional Conditional 的构造函数
func conditional() *Conditional {
	node := &conditionNode{}
	return &Conditional{head: node, curr: node}
}

// Empty 返回表达式是否为空
func (c *Conditional) Empty() bool {
	return c.head == c.curr
}

// Matches 成功返回 true，失败返回 false
func (c *Conditional) Matches(ctx bean.ConditionContext) bool {
	return c.head.Matches(ctx)
}

// Or c=a||b
func (c *Conditional) Or() *Conditional {
	node := &conditionNode{}
	c.curr.op = ConditionOr
	c.curr.next = node
	c.curr = node
	return c
}

// And c=a&&b
func (c *Conditional) And() *Conditional {
	node := &conditionNode{}
	c.curr.op = ConditionAnd
	c.curr.next = node
	c.curr = node
	return c
}

// On 设置一个 Condition
func On(cond bean.Condition) *Conditional {
	return conditional().OnCondition(cond)
}

// OnCondition 设置一个 Condition
func (c *Conditional) OnCondition(cond bean.Condition) *Conditional {
	if c.curr.cond != nil {
		c.And()
	}
	c.curr.cond = cond
	return c
}

// OnConditionNot 设置一个取反的 Condition
func (c *Conditional) OnConditionNot(cond bean.Condition) *Conditional {
	return c.OnCondition(NotCondition(cond))
}

// OnProperty 返回设置了 propertyCondition 的 Conditional 对象
func OnProperty(name string) *Conditional {
	return conditional().OnProperty(name)
}

// OnProperty 设置一个 propertyCondition
func (c *Conditional) OnProperty(name string) *Conditional {
	return c.OnCondition(PropertyCondition(name))
}

// OnMissingProperty 返回设置了 missingPropertyCondition 的 Conditional 对象
func OnMissingProperty(name string) *Conditional {
	return conditional().OnMissingProperty(name)
}

// OnMissingProperty 设置一个 missingPropertyCondition
func (c *Conditional) OnMissingProperty(name string) *Conditional {
	return c.OnCondition(MissingPropertyCondition(name))
}

// OnPropertyValue 返回设置了 propertyValueCondition 的 Conditional 对象
func OnPropertyValue(name string, havingValue interface{},
	options ...PropertyValueConditionOption) *Conditional {
	return conditional().OnPropertyValue(name, havingValue, options...)
}

// OnPropertyValue 设置一个 propertyValueCondition
func (c *Conditional) OnPropertyValue(name string, havingValue interface{},
	options ...PropertyValueConditionOption) *Conditional {
	return c.OnCondition(PropertyValueCondition(name, havingValue, options...))
}

// OnOptionalPropertyValue 返回属性值不存在时默认条件成立的 Conditional 对象
func OnOptionalPropertyValue(name string, havingValue interface{}) *Conditional {
	return conditional().OnOptionalPropertyValue(name, havingValue)
}

// OnOptionalPropertyValue 设置一个 propertyValueCondition，当属性值不存在时默认条件成立
func (c *Conditional) OnOptionalPropertyValue(name string, havingValue interface{}) *Conditional {
	return c.OnCondition(PropertyValueCondition(name, havingValue, MatchIfMissing(true)))
}

// OnBean 返回设置了 beanCondition 的 Conditional 对象
func OnBean(selector bean.BeanSelector) *Conditional {
	return conditional().OnBean(selector)
}

// OnBean 设置一个 beanCondition
func (c *Conditional) OnBean(selector bean.BeanSelector) *Conditional {
	return c.OnCondition(BeanCondition(selector))
}

// OnMissingBean 返回设置了 missingBeanCondition 的 Conditional 对象
func OnMissingBean(selector bean.BeanSelector) *Conditional {
	return conditional().OnMissingBean(selector)
}

// OnMissingBean 设置一个 missingBeanCondition
func (c *Conditional) OnMissingBean(selector bean.BeanSelector) *Conditional {
	return c.OnCondition(MissingBeanCondition(selector))
}

// OnExpression 返回设置了 expressionCondition 的 Conditional 对象
func OnExpression(expression string) *Conditional {
	return conditional().OnExpression(expression)
}

// OnExpression 设置一个 expressionCondition
func (c *Conditional) OnExpression(expression string) *Conditional {
	return c.OnCondition(ExpressionCondition(expression))
}

// OnMatches 返回设置了 functionCondition 的 Conditional 对象
func OnMatches(fn ConditionFunc) *Conditional {
	return conditional().OnMatches(fn)
}

// OnMatches 设置一个 functionCondition
func (c *Conditional) OnMatches(fn ConditionFunc) *Conditional {
	return c.OnCondition(FunctionCondition(fn))
}

// OnProfile 返回设置了 profileCondition 的 Conditional 对象
func OnProfile(profile string) *Conditional {
	return conditional().OnProfile(profile)
}

// OnProfile 设置一个 profileCondition
func (c *Conditional) OnProfile(profile string) *Conditional {
	return c.OnCondition(ProfileCondition(profile))
}
