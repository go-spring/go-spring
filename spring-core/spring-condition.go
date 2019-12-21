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
	"go/token"
	"go/types"
	"strings"

	"github.com/go-spring/go-spring-parent/spring-const"
	"github.com/spf13/cast"
)

//
// Condition 定义 Condition 接口，当判断条件返回 true 时 Bean 才会真正注册到 IoC 容器。
//
type Condition interface {
	Matches(ctx SpringContext) bool
}

//
// ConditionFunc 定义 Condition 接口 Matches 方法的类型
//
type ConditionFunc func(ctx SpringContext) bool

//
// functionCondition 基于 Matches 方法的 Condition 实现
//
type functionCondition struct {
	fn ConditionFunc
}

//
// NewFunctionCondition 工厂函数
//
func NewFunctionCondition(fn ConditionFunc) *functionCondition {
	if fn == nil {
		panic("fn can't be null")
	}
	return &functionCondition{fn}
}

//
// Matches
//
func (c *functionCondition) Matches(ctx SpringContext) bool {
	return c.fn(ctx)
}

//
// propertyCondition 基于属性值存在的 Condition 实现
//
type propertyCondition struct {
	name string
}

//
// NewPropertyCondition 工厂函数
//
func NewPropertyCondition(name string) *propertyCondition {
	return &propertyCondition{name}
}

//
// Matches
//
func (c *propertyCondition) Matches(ctx SpringContext) bool {
	return len(ctx.GetPrefixProperties(c.name)) > 0
}

//
// missingPropertyCondition 基于属性值不存在的 Condition 实现
//
type missingPropertyCondition struct {
	name string
}

//
// NewMissingPropertyCondition 工厂函数
//
func NewMissingPropertyCondition(name string) *missingPropertyCondition {
	return &missingPropertyCondition{name}
}

//
// Matches
//
func (c *missingPropertyCondition) Matches(ctx SpringContext) bool {
	return len(ctx.GetPrefixProperties(c.name)) <= 0
}

//
// propertyValueCondition 基于属性值匹配的 Condition 实现
//
type propertyValueCondition struct {
	name        string
	havingValue interface{}
}

//
// NewPropertyValueCondition 工厂函数
//
func NewPropertyValueCondition(name string, havingValue interface{}) *propertyValueCondition {
	return &propertyValueCondition{name, havingValue}
}

//
// Matches
//
func (c *propertyValueCondition) Matches(ctx SpringContext) bool {
	// 参考 /usr/local/go/src/go/types/eval_test.go 示例

	val, ok := ctx.GetDefaultProperty(c.name, "")
	if !ok { // 不存在直接返回 false
		return false
	}

	// 不是字符串则直接比较
	expectValue, ok := c.havingValue.(string)
	if !ok {
		return val == c.havingValue
	}

	expr := strings.Replace(expectValue, "$", cast.ToString(val), -1)
	gotTv, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expr)
	if err != nil {
		panic(err)
	}
	return gotTv.Value.String() == "true"
}

//
// beanCondition 基于 Bean 的 Condition 实现
//
type beanCondition struct {
	beanId string
}

//
// NewBeanCondition 工厂函数
//
func NewBeanCondition(beanId string) *beanCondition {
	return &beanCondition{beanId}
}

//
// Matches
//
func (c *beanCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.FindBeanByName(c.beanId)
	return ok
}

//
// missingBeanCondition 基于 Missing Bean 的 Condition 实现
//
type missingBeanCondition struct {
	beanId string
}

//
// NewMissingBeanCondition 工厂函数
//
func NewMissingBeanCondition(beanId string) *missingBeanCondition {
	return &missingBeanCondition{beanId}
}

//
// Matches
//
func (c *missingBeanCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.FindBeanByName(c.beanId)
	return !ok
}

//
// expressionCondition 基于表达式的 Condition 实现
//
type expressionCondition struct {
	expression string
}

//
// NewExpressionCondition 工厂函数
//
func NewExpressionCondition(expression string) *expressionCondition {
	return &expressionCondition{expression}
}

//
// Matches
//
func (c *expressionCondition) Matches(ctx SpringContext) bool {
	panic(SpringConst.UNIMPLEMENTED_METHOD)
}

type OpMode int

const (
	OpMode_None = OpMode(0)
	OpMode_Or   = OpMode(1)
	OpMode_And  = OpMode(2)
)

//
// conditionNode 定义 Condition 表达式的节点
//
type conditionNode struct {
	next *conditionNode // 下一个节点
	op   OpMode         // 计算方式
	cond Condition      // 条件
}

//
// newConditionNode 工厂函数
//
func newConditionNode() *conditionNode {
	return &conditionNode{
		op: OpMode_None,
	}
}

//
// Matches
//
func (c *conditionNode) Matches(ctx SpringContext) bool {

	if c.next != nil && c.next.cond == nil {
		panic("last op need a cond triggered")
	}

	if c.cond == nil && c.op == OpMode_None {
		return true
	}

	if r := c.cond.Matches(ctx); c.next != nil {

		switch c.op {
		case OpMode_Or: // or
			if r {
				return r
			} else {
				return c.next.Matches(ctx)
			}
		case OpMode_And: // and
			if r {
				return c.next.Matches(ctx)
			} else {
				return false
			}
		default:
			panic("error condition node op mode")
		}

	} else {
		return r
	}
}

//
// conditional 定义 Condition 表达式
//
type conditional struct {
	node *conditionNode
	curr *conditionNode
}

//
// NewConditional 工厂函数
//
func NewConditional() *conditional {
	node := newConditionNode()
	return &conditional{
		node: node,
		curr: node,
	}
}

//
// Matches
//
func (c *conditional) Matches(ctx SpringContext) bool {
	return c.node.Matches(ctx)
}

//
// checkCondition
//
func (c *conditional) checkCondition() {
	if c.curr.cond != nil {
		panic("condition already set")
	}
}

//
// Or c=a||b
//
func (c *conditional) Or() *conditional {
	node := newConditionNode()
	c.curr.op = OpMode_Or
	c.curr.next = node
	c.curr = node
	return c
}

//
// And c=a&&b
//
func (c *conditional) And() *conditional {
	node := newConditionNode()
	c.curr.op = OpMode_And
	c.curr.next = node
	c.curr = node
	return c
}

//
// OnCondition
//
func (c *conditional) OnCondition(cond Condition) *conditional {
	c.checkCondition()
	c.curr.cond = cond
	return c
}

//
// OnProperty
//
func (c *conditional) OnProperty(name string) *conditional {
	c.checkCondition()
	c.curr.cond = NewPropertyCondition(name)
	return c
}

//
// OnMissingProperty
//
func (c *conditional) OnMissingProperty(name string) *conditional {
	c.checkCondition()
	c.curr.cond = NewMissingPropertyCondition(name)
	return c
}

//
// OnPropertyValue
//
func (c *conditional) OnPropertyValue(name string, havingValue interface{}) *conditional {
	c.checkCondition()
	c.curr.cond = NewPropertyValueCondition(name, havingValue)
	return c
}

//
// OnBean
//
func (c *conditional) OnBean(beanId string) *conditional {
	c.checkCondition()
	c.curr.cond = NewBeanCondition(beanId)
	return c
}

//
// OnMissingBean
//
func (c *conditional) OnMissingBean(beanId string) *conditional {
	c.checkCondition()
	c.curr.cond = NewMissingBeanCondition(beanId)
	return c
}

//
// OnExpression
//
func (c *conditional) OnExpression(expression string) *conditional {
	c.checkCondition()
	c.curr.cond = NewExpressionCondition(expression)
	return c
}

//
// OnMatches
//
func (c *conditional) OnMatches(fn ConditionFunc) *conditional {
	c.checkCondition()
	c.curr.cond = NewFunctionCondition(fn)
	return c
}
