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

package StarterRabbitMQProducer

import (
	"context"

	"github.com/go-spring/spring-boot"
	"github.com/go-spring/spring-message"
	"github.com/go-spring/starter-rabbitmq"
	"github.com/streadway/amqp"
)

func init() {
	SpringBoot.RegisterNameBean("amqp-sender", new(Sender)).Export((*SpringMessage.Producer)(nil))
}

type Sender struct {
	Server *StarterRabbitMQ.AMQPServer `autowire:""`
}

func (sender *Sender) SendMessage(ctx context.Context, msg SpringMessage.MessageInterface) error {
	switch message := msg.(type) {
	case *SpringMessage.Message:
		return sender.Server.Channel.Publish(
			"",            // exchange
			message.Topic, // routing key
			false,         // mandatory
			false,         // immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        message.Body,
			})
	}
	return nil
}
