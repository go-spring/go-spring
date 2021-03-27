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

package mq

import (
	"context"

	"github.com/go-spring/spring-core/json"
)

// Message 简单消息
type Message struct {
	Topic      string
	MessageId  string            // 也叫 Key
	Body       []byte            // 也叫 Value
	Properties map[string]string // 附加的属性对
}

// NewMessage Message 的构造函数
func NewMessage() *Message {
	return &Message{
		Properties: make(map[string]string),
	}
}

// WithTopic 设置 Message 的 Topic。
func (msg *Message) WithTopic(topic string) *Message {
	msg.Topic = topic
	return msg
}

// WithMessageId 设置 Message 的消息序号。
func (msg *Message) WithMessageId(msgId string) *Message {
	msg.MessageId = msgId
	return msg
}

// AddProperty 给 Message 添加属性对。
func (msg *Message) AddProperty(key, value string) *Message {
	msg.Properties[key] = value
	return msg
}

// WithBody 设置 Message 的消息体。
func (msg *Message) WithBody(body []byte) *Message {
	msg.Body = body
	return msg
}

// WithJsonBody 设置 Message 的消息体，序列化出错时将错误内容当作消息体。
func (msg *Message) WithJsonBody(body interface{}) *Message {
	data := json.ToString(body)
	msg.Body = []byte(data)
	return msg
}

// Consumer 消息消费者
type Consumer interface {
	Topics() []string // 声明消费的主题列表
	Consume(ctx context.Context, msg *Message)
}

// MessageInterface 抽象化的消息接口
type MessageInterface interface{}

// Producer 消息生产者，SendMessage 的消息类型不同用途也不同，需要自行判断。
type Producer interface {
	SendMessage(ctx context.Context, msg MessageInterface) error
}
