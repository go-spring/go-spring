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

package StarterRabbitMQConsumer

import (
	"context"

	"github.com/go-spring/spring-core/boot"
	"github.com/go-spring/spring-core/log"
	"github.com/go-spring/spring-core/mq"
	"github.com/go-spring/starter-rabbitmq"
)

func init() {
	boot.RegisterNameBean("amqp-consumer-starter", new(Starter)).
		Export((*boot.ApplicationEvent)(nil))
}

type Starter struct {
	Server *StarterRabbitMQ.AMQPServer `autowire:""`
}

func (starter *Starter) OnStartApplication(ctx boot.ApplicationContext) {

	cMap := map[string][]mq.Consumer{}
	{
		var consumers []mq.Consumer
		_ = boot.CollectBeans(&consumers)

		for _, c := range boot.BindConsumerMapping {
			if c.CheckCondition(ctx) {
				consumers = append(consumers, c)
			}
		}

		for _, consumer := range consumers {
			for _, topic := range consumer.Topics() {
				cMap[topic] = append(cMap[topic], consumer)
			}
		}
	}

	go func() {
		// TODO 使用 goroutine 池提高消费速率
		for topic, consumers := range cMap {
			delivery, err := starter.Server.Channel.Consume(
				topic, // queue
				"",    // consumer
				true,  // auto-ack
				false, // exclusive
				false, // no-local
				false, // no-wait
				nil,   // args
			)
			if err != nil {
				log.Error(err)
				continue
			}
			d := <-delivery
			msg := mq.NewMessage().WithBody(d.Body).WithTopic(topic)
			for _, c := range consumers {
				c.Consume(context.TODO(), msg)
			}
		}
	}()
}

func (starter *Starter) OnStopApplication(ctx boot.ApplicationContext) {

}
