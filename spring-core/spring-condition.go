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
	"go/token"
	"go/types"
	"strings"

	"github.com/go-spring/go-spring-parent/spring-const"
	"github.com/spf13/cast"
)

// Condition 定义一个判断条件
type Condition interface {
	// 成功返回 true，失败返回 false
	Matches(ctx SpringContext) bool
}

// ConditionFunc 定义 Condition 接口 Matches 方法的类型
type ConditionFunc func(ctx SpringContext) bool

// functionCondition 基于 Matches 方法的 Condition 实现
type functionCondition struct {
	fn ConditionFunc
}

// NewFunctionCondition functionCondition 的构造函数
func NewFunctionCondition(fn ConditionFunc) *functionCondition {
	if fn == nil {
		panic(errors.New("fn can't be nil"))
	}
	return &functionCondition{fn}
}

// Matches 成功返回 true，失败返回 false
func (c *functionCondition) Matches(ctx SpringContext) bool {
	return c.fn(ctx)
}

// propertyCondition 基于属性值存在的 Condition 实现
type propertyCondition struct {
	name string
}

// NewPropertyCondition propertyCondition 的构造函数
func NewPropertyCondition(name string) *propertyCondition {
	return &propertyCondition{name}
}

// Matches 成功返回 true，失败返回 false
func (c *propertyCondition) Matches(ctx SpringContext) bool {
	return len(ctx.GetPrefixProperties(c.name)) > 0
}

// missingPropertyCondition 基于属性值不存在的 Condition 实现
type missingPropertyCondition struct {
	name string
}

// NewMissingPropertyCondition missingPropertyCondition 的构造函数
func NewMissingPropertyCondition(name string) *missingPropertyCondition {
	return &missingPropertyCondition{name}
}

// Matches 成功返回 true，失败返回 false
func (c *missingPropertyCondition) Matches(ctx SpringContext) bool {
	return len(ctx.GetPrefixProperties(c.name)) <= 0
}

// propertyValueCondition 基于属性值匹配的 Condition 实现
type propertyValueCondition struct {
	name        string
	havingValue interface{}
}

// NewPropertyValueCondition propertyValueCondition 的构造函数
func NewPropertyValueCondition(name string, havingValue interface{}) *propertyValueCondition {
	return &propertyValueCondition{name, havingValue}
}

// Matches 成功返回 true，失败返回 false
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

	// 字符串不是表达式的话则直接比较
	if ok = strings.Contains(expectValue, "$"); !ok {
		return val == expectValue
	}

	expr := strings.Replace(expectValue, "$", cast.ToString(val), -1)
	gotTv, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expr)
	if err != nil {
		panic(err)
	}
	return gotTv.Value.String() == "true"
}

// beanCondition 基于 Bean 存在的 Condition 实现
type beanCondition struct {
	beanId string
}

// NewBeanCondition beanCondition 的构造函数
func NewBeanCondition(beanId string) *beanCondition {
	return &beanCondition{beanId}
}

// Matches 成功返回 true，失败返回 false
func (c *beanCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.FindBeanByName(c.beanId)
	return ok
}

// missingBeanCondition 基于 Bean 不能存在的 Condition 实现
type missingBeanCondition struct {
	beanId string
}

// NewMissingBeanCondition missingBeanCondition 的构造函数
func NewMissingBeanCondition(beanId string) *missingBeanCondition {
	return &missingBeanCondition{beanId}
}

// Matches 成功返回 true，失败返回 false
func (c *missingBeanCondition) Matches(ctx SpringContext) bool {
	_, ok := ctx.FindBeanByName(c.beanId)
	return !ok
}

// expressionCondition 基于表达式的 Condition 实现
type expressionCondition struct {
	expression string
}

// NewExpressionCondition expressionCondition 的构造函数
func NewExpressionCondition(expression string) *expressionCondition {
	return &expressionCondition{expression}
}

// Matches 成功返回 true，失败返回 false
func (c *expressionCondition) Matches(ctx SpringContext) bool {
	panic(SpringConst.UNIMPLEMENTED_METHOD)
}

// 定义 conditionNode 的计算方式
type opMode int

const (
	opMode_None = opMode(0) // 默认值
	opMode_Or   = opMode(1) // 或
	opMode_And  = opMode(2) // 且
)

// conditionNode 定义 Condition 计算的节点
type conditionNode struct {
	next *conditionNode // 下一个计算节点
	op   opMode         // 计算方式
	cond Condition      // 条件
}

// newConditionNode conditionNode 的构造函数
func newConditionNode() *conditionNode {
	return &conditionNode{
		op: opMode_None,
	}
}

// Matches 成功返回 true，失败返回 false
func (c *conditionNode) Matches(ctx SpringContext) bool {

	if c.next != nil && c.next.cond == nil {
		panic(errors.New("last op need a cond triggered"))
	}

	if c.cond == nil && c.op == opMode_None {
		return true
	}

	if r := c.cond.Matches(ctx); c.next != nil {

		switch c.op {
		case opMode_Or: // or
			if r {
				return r
			} else {
				return c.next.Matches(ctx)
			}
		case opMode_And: // and
			if r {
				return c.next.Matches(ctx)
			} else {
				return false
			}
		default:
			panic(errors.New("error condition node op mode"))
		}

	} else {
		return r
	}
}

// conditional 定义 Condition 计算式
type conditional struct {
	node *conditionNode
	curr *conditionNode
}

// NewConditional conditional 的构造函数
func NewConditional() *conditional {
	node := newConditionNode()
	return &conditional{
		node: node,
		curr: node,
	}
}

// Matches 成功返回 true，失败返回 false
func (c *conditional) Matches(ctx SpringContext) bool {
	return c.node.Matches(ctx)
}

// Or c=a||b
func (c *conditional) Or() *conditional {
	node := newConditionNode()
	c.curr.op = opMode_Or
	c.curr.next = node
	c.curr = node
	return c
}

// And c=a&&b
func (c *conditional) And() *conditional {
	node := newConditionNode()
	c.curr.op = opMode_And
	c.curr.next = node
	c.curr = node
	return c
}

// OnCondition 设置一个 Condition
func (c *conditional) OnCondition(cond Condition) *conditional {
	if c.curr.cond != nil {
		panic(errors.New("condition already set"))
	}
	c.curr.cond = cond
	return c
}

// OnProperty 设置一个 propertyCondition
func (c *conditional) OnProperty(name string) *conditional {
	return c.OnCondition(NewPropertyCondition(name))
}

// OnMissingProperty 设置一个 missingPropertyCondition
func (c *conditional) OnMissingProperty(name string) *conditional {
	return c.OnCondition(NewMissingPropertyCondition(name))
}

// OnPropertyValue 设置一个 propertyValueCondition
func (c *conditional) OnPropertyValue(name string, havingValue interface{}) *conditional {
	return c.OnCondition(NewPropertyValueCondition(name, havingValue))
}

// OnBean 设置一个 beanCondition
func (c *conditional) OnBean(beanId string) *conditional {
	return c.OnCondition(NewBeanCondition(beanId))
}

// OnMissingBean 设置一个 missingBeanCondition
func (c *conditional) OnMissingBean(beanId string) *conditional {
	return c.OnCondition(NewMissingBeanCondition(beanId))
}

// OnExpression 设置一个 expressionCondition
func (c *conditional) OnExpression(expression string) *conditional {
	return c.OnCondition(NewExpressionCondition(expression))
}

// OnMatches 设置一个 functionCondition
func (c *conditional) OnMatches(fn ConditionFunc) *conditional {
	return c.OnCondition(NewFunctionCondition(fn))
}
