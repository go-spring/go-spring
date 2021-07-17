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

// TODO 提取公共部分到 spring-rabbitmq 包
package StarterRabbitMQServer

import (
	"github.com/go-spring/spring-core/gs"
	"github.com/streadway/amqp"
)

func init() {
	gs.Provide(CreateServer).Name("amqp-server").Destroy(DestroyServer)
}

type AMQPServerConfig struct {
	URL         string   `value:"${amqp.server.url}"`
	QueueTopics []string `value:"${amqp.queue.topics}"`
}

type AMQPServer struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
}

// CreateServer 创建 AMQPServer 对象，采用预先声明的方式避免运行时锁消耗
func CreateServer(config AMQPServerConfig) (*AMQPServer, error) {

	conn, err := amqp.Dial(config.URL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	for _, topic := range config.QueueTopics {
		_, err = ch.QueueDeclare(
			topic, // name
			false, // durable
			false, // delete when unused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
		if err != nil {
			return nil, err
		}
	}

	return &AMQPServer{conn, ch}, nil
}

// DestroyServer 销毁 AMQPServer 对象
func DestroyServer(server *AMQPServer) {
	if server.Channel != nil {
		_ = server.Channel.Close()
	}
	if server.Connection != nil {
		_ = server.Connection.Close()
	}
}
