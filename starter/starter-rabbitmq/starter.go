/*
 * Copyright 2025 The Go-Spring Authors.
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

package StarterRabbitMQ

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/spring/gs"
)

func init() {

	// Register a single default RabbitMQ connection.
	// This connection will only be created if the property "spring.rabbitmq.url" is set.
	// It uses the configuration tagged with "${spring.rabbitmq}" and is named "__default__".
	gs.Provide(newClient, gs.TagArg("${spring.rabbitmq}")).
		Condition(gs.OnProperty("spring.rabbitmq.url")).
		Destroy(destroyClient).
		Name("__default__")

	// Register multiple RabbitMQ connections as a group.
	// Each instance is created according to the configuration in "${spring.rabbitmq.instances}".
	// This allows defining multiple RabbitMQ connections dynamically.
	gs.Group("${spring.rabbitmq.instances}", newClient, destroyClient)
}

// newClient creates a new RabbitMQ connection based on the provided configuration.
func newClient(c Config) (*amqp.Connection, error) {
	if c.Heartbeat > 0 || c.Vhost != "" {
		return amqp.DialConfig(c.URL, amqp.Config{
			Vhost:     c.Vhost,
			Heartbeat: c.Heartbeat,
		})
	}
	return amqp.Dial(c.URL)
}

// destroyClient closes the RabbitMQ connection.
func destroyClient(conn *amqp.Connection) error {
	return conn.Close()
}
