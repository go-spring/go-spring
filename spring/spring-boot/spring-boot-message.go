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

package SpringBoot

import (
	"github.com/go-spring/spring-core"
	"github.com/go-spring/spring-message"
)

// ConditionalBindConsumer 为 BindConsumer 添加条件功能
type ConditionalBindConsumer struct {
	*SpringMessage.BindConsumer
	cond *SpringCore.Conditional // 判断条件
}

// Or c=a||b
func (c *ConditionalBindConsumer) Or() *ConditionalBindConsumer {
	c.cond.Or()
	return c
}

// And c=a&&b
func (c *ConditionalBindConsumer) And() *ConditionalBindConsumer {
	c.cond.And()
	return c
}

// ConditionOn 设置一个 Condition
func (c *ConditionalBindConsumer) ConditionOn(cond SpringCore.Condition) *ConditionalBindConsumer {
	c.cond.OnCondition(cond)
	return c
}

// ConditionNot 设置一个取反的 Condition
func (c *ConditionalBindConsumer) ConditionNot(cond SpringCore.Condition) *ConditionalBindConsumer {
	c.cond.OnConditionNot(cond)
	return c
}

// ConditionOnProperty 设置一个 PropertyCondition
func (c *ConditionalBindConsumer) ConditionOnProperty(name string) *ConditionalBindConsumer {
	c.cond.OnProperty(name)
	return c
}

// ConditionOnMissingProperty 设置一个 MissingPropertyCondition
func (c *ConditionalBindConsumer) ConditionOnMissingProperty(name string) *ConditionalBindConsumer {
	c.cond.OnMissingProperty(name)
	return c
}

// ConditionOnPropertyValue 设置一个 PropertyValueCondition
func (c *ConditionalBindConsumer) ConditionOnPropertyValue(name string, havingValue interface{},
	options ...SpringCore.PropertyValueConditionOption) *ConditionalBindConsumer {
	c.cond.OnPropertyValue(name, havingValue, options...)
	return c
}

// ConditionOnOptionalPropertyValue 设置一个 PropertyValueCondition，当属性值不存在时默认条件成立
func (c *ConditionalBindConsumer) ConditionOnOptionalPropertyValue(name string, havingValue interface{}) *ConditionalBindConsumer {
	c.cond.OnOptionalPropertyValue(name, havingValue)
	return c
}

// ConditionOnBean 设置一个 BeanCondition
func (c *ConditionalBindConsumer) ConditionOnBean(selector SpringCore.BeanSelector) *ConditionalBindConsumer {
	c.cond.OnBean(selector)
	return c
}

// ConditionOnMissingBean 设置一个 MissingBeanCondition
func (c *ConditionalBindConsumer) ConditionOnMissingBean(selector SpringCore.BeanSelector) *ConditionalBindConsumer {
	c.cond.OnMissingBean(selector)
	return c
}

// ConditionOnExpression 设置一个 ExpressionCondition
func (c *ConditionalBindConsumer) ConditionOnExpression(expression string) *ConditionalBindConsumer {
	c.cond.OnExpression(expression)
	return c
}

// ConditionOnMatches 设置一个 FunctionCondition
func (c *ConditionalBindConsumer) ConditionOnMatches(fn SpringCore.ConditionFunc) *ConditionalBindConsumer {
	c.cond.OnMatches(fn)
	return c
}

// ConditionOnProfile 设置一个 ProfileCondition
func (c *ConditionalBindConsumer) ConditionOnProfile(profile string) *ConditionalBindConsumer {
	c.cond.OnProfile(profile)
	return c
}

// CheckCondition 成功返回 true，失败返回 false
func (c *ConditionalBindConsumer) CheckCondition(ctx SpringCore.SpringContext) bool {
	return c.cond.Matches(ctx)
}

// BindConsumerMapping 以 BIND 形式注册的消息消费者的映射表
var BindConsumerMapping = map[string]*ConditionalBindConsumer{}

// BindConsumer 注册 BIND 形式的消息消费者
func BindConsumer(topic string, fn interface{}) *ConditionalBindConsumer {
	c := &ConditionalBindConsumer{
		BindConsumer: SpringMessage.BIND(topic, fn),
		cond:         SpringCore.NewConditional(),
	}
	BindConsumerMapping[topic] = c
	return c
}
