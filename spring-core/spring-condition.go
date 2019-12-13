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
// 定义 Condition 接口，当判断条件返回 true 时 Bean 才会真正注册到 IoC 容器。
//
type Condition interface {
	Matches(ctx SpringContext) bool
}

//
// 定义 Condition 接口 Matches 方法的类型
//
type ConditionFunc func(ctx SpringContext) bool

//
// 基于 Matches 方法的 Condition 实现
//
type FunctionCondition struct {
	fn ConditionFunc
}

//
// 工厂函数
//
func NewFunctionCondition(fn ConditionFunc) *FunctionCondition {
	if fn == nil {
		panic("fn can't be null")
	}
	return &FunctionCondition{fn}
}

func (c *FunctionCondition) Matches(ctx SpringContext) bool {
	return c.fn(ctx)
}

//
// 基于属性值存在的 Condition 实现
//
type PropertyCondition struct {
	name string
}

//
// 工厂函数
//
func NewPropertyCondition(name string) *PropertyCondition {
	return &PropertyCondition{name}
}

func (c *PropertyCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.GetDefaultProperty(c.name, "")
	return ok
}

//
// 基于属性值不存在的 Condition 实现
//
type MissingPropertyCondition struct {
	name string
}

//
// 工厂函数
//
func NewMissingPropertyCondition(name string) *MissingPropertyCondition {
	return &MissingPropertyCondition{name}
}

func (c *MissingPropertyCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.GetDefaultProperty(c.name, "")
	return !ok
}

//
// 基于属性值匹配的 Condition 实现
//
type PropertyValueCondition struct {
	name        string
	havingValue interface{}
}

//
// 工厂函数
//
func NewPropertyValueCondition(name string, havingValue interface{}) *PropertyValueCondition {
	return &PropertyValueCondition{name, havingValue}
}

func (c *PropertyValueCondition) Matches(ctx SpringContext) bool {
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
// 基于 Bean 的 Condition 实现
//
type BeanCondition struct {
	beanId string
}

//
// 工厂函数
//
func NewBeanCondition(beanId string) *BeanCondition {
	return &BeanCondition{beanId}
}

func (c *BeanCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.FindBeanByName(c.beanId)
	return ok
}

//
// 基于 Missing Bean 的 Condition 实现
//
type MissingBeanCondition struct {
	beanId string
}

//
// 工厂函数
//
func NewMissingBeanCondition(beanId string) *MissingBeanCondition {
	return &MissingBeanCondition{beanId}
}

func (c *MissingBeanCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.FindBeanByName(c.beanId)
	return !ok
}

//
// 基于表达式的 Condition 实现
//
type ExpressionCondition struct {
	expression string
}

//
// 工厂函数
//
func NewExpressionCondition(expression string) *ExpressionCondition {
	return &ExpressionCondition{expression}
}

func (c *ExpressionCondition) Matches(ctx SpringContext) bool {
	panic(SpringConst.UNIMPLEMENTED_METHOD)
}

type OpMode int

const (
	OpMode_None = OpMode(0)
	OpMode_Or   = OpMode(1)
	OpMode_And  = OpMode(2)
)

//
// 定义 Condition 表达式的节点
//
type ConditionNode struct {
	next *ConditionNode // 下一个节点
	op   OpMode         // 计算方式
	cond Condition      // 条件
}

//
// 工厂函数
//
func NewConditionNode() *ConditionNode {
	return &ConditionNode{
		op: OpMode_None,
	}
}

func (c *ConditionNode) Matches(ctx SpringContext) bool {

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
// 定义 Condition 表达式
//
type Conditional struct {
	node *ConditionNode
	curr *ConditionNode
}

//
// 工厂函数
//
func NewConditional() *Conditional {
	node := NewConditionNode()
	return &Conditional{
		node: node,
		curr: node,
	}
}

func (c *Conditional) Matches(ctx SpringContext) bool {
	return c.node.Matches(ctx)
}

func (c *Conditional) checkCondition() {
	if c.curr.cond != nil {
		panic("condition already set")
	}
}

//
// c=a||b
//
func (c *Conditional) Or() *Conditional {
	node := NewConditionNode()
	c.curr.op = OpMode_Or
	c.curr.next = node
	c.curr = node
	return c
}

//
// c=a&&b
//
func (c *Conditional) And() *Conditional {
	node := NewConditionNode()
	c.curr.op = OpMode_And
	c.curr.next = node
	c.curr = node
	return c
}

func (c *Conditional) OnCondition(cond Condition) *Conditional {
	c.checkCondition()
	c.curr.cond = cond
	return c
}

func (c *Conditional) OnProperty(name string) *Conditional {
	c.checkCondition()
	c.curr.cond = NewPropertyCondition(name)
	return c
}

func (c *Conditional) OnMissingProperty(name string) *Conditional {
	c.checkCondition()
	c.curr.cond = NewMissingPropertyCondition(name)
	return c
}

func (c *Conditional) OnPropertyValue(name string, havingValue interface{}) *Conditional {
	c.checkCondition()
	c.curr.cond = NewPropertyValueCondition(name, havingValue)
	return c
}

func (c *Conditional) OnBean(beanId string) *Conditional {
	c.checkCondition()
	c.curr.cond = NewBeanCondition(beanId)
	return c
}

func (c *Conditional) OnMissingBean(beanId string) *Conditional {
	c.checkCondition()
	c.curr.cond = NewMissingBeanCondition(beanId)
	return c
}

func (c *Conditional) OnExpression(expression string) *Conditional {
	c.checkCondition()
	c.curr.cond = NewExpressionCondition(expression)
	return c
}

func (c *Conditional) OnMatches(fn ConditionFunc) *Conditional {
	c.checkCondition()
	c.curr.cond = NewFunctionCondition(fn)
	return c
}
