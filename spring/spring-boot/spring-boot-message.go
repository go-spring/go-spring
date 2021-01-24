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
	cond SpringCore.Condition // 判断条件
}

// WithCondition 设置一个 Condition
func (c *ConditionalBindConsumer) WithCondition(cond SpringCore.Condition) *ConditionalBindConsumer {
	c.cond = cond
	return c
}

// CheckCondition 成功返回 true，失败返回 false
func (c *ConditionalBindConsumer) CheckCondition(ctx SpringCore.ApplicationContext) bool {
	if c.cond == nil {
		return true
	}
	return c.cond.Matches(ctx)
}

// BindConsumerMapping 以 BIND 形式注册的消息消费者的映射表
var BindConsumerMapping = map[string]*ConditionalBindConsumer{}

// BindConsumer 注册 BIND 形式的消息消费者
func BindConsumer(topic string, fn interface{}) *ConditionalBindConsumer {
	c := &ConditionalBindConsumer{BindConsumer: SpringMessage.BIND(topic, fn)}
	BindConsumerMapping[topic] = c
	return c
}
