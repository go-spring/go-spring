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

type Message interface {
	Topic() string
	ID() string
	Body() []byte
	Extra() map[string]string
}

type message struct {
	topic string            // 消息主题
	id    string            // Key
	body  []byte            // Value
	extra map[string]string // 额外信息
}

// NewMessage 创建新的消息对象。
func NewMessage() *message {
	return &message{}
}

// Topic 返回消息的主题。
func (msg *message) Topic() string {
	return msg.topic
}

// WithTopic 设置消息的主题。
func (msg *message) WithTopic(topic string) *message {
	msg.topic = topic
	return msg
}

// ID 返回消息的序号。
func (msg *message) ID() string {
	return msg.id
}

// WithID 设置消息的序号。
func (msg *message) WithID(id string) *message {
	msg.id = id
	return msg
}

// Body 返回消息的内容。
func (msg *message) Body() []byte {
	return msg.body
}

// WithBody 设置消息的内容。
func (msg *message) WithBody(body []byte) *message {
	msg.body = body
	return msg
}

// Extra 返回消息的额外信息。
func (msg *message) Extra() map[string]string {
	return msg.extra
}

// WithExtra 为消息添加额外的信息。
func (msg *message) WithExtra(key, value string) *message {
	if msg.extra == nil {
		msg.extra = make(map[string]string)
	}
	msg.extra[key] = value
	return msg
}
