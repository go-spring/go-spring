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

package boot

import (
	"github.com/go-spring/spring-core/mq"
)

// ConsumerMapping 以 BIND 形式注册的消息消费者的映射表
var ConsumerMapping = map[string]*mq.BindConsumer{}

// BindConsumer 注册 BIND 形式的消息消费者
func BindConsumer(topic string, fn interface{}) *mq.BindConsumer {
	c := mq.BIND(topic, fn)
	ConsumerMapping[topic] = c
	return c
}
