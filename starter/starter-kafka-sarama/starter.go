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

package StarterKafkaSarama

import (
	"strings"

	"github.com/IBM/sarama"
	"go-spring.org/spring/gs"
)

func init() {

	// Register multiple Kafka clients as a group.
	// Each instance is created according to the configuration in "${spring.kafka.instances}".
	// This allows defining multiple Kafka clients dynamically.
	gs.Group("${spring.kafka.instances}", newClient, destroyClient)
}

// newClient creates a shared low-level sarama.Client. Callers derive a
// SyncProducer, Consumer or ConsumerGroup from it via the sarama.*FromClient
// constructors, mirroring franz-go's single-client model. Producer success
// notifications are enabled so the client can back a SyncProducer, and the
// initial consumer offset defaults to the oldest available message.
func newClient(c Config) (sarama.Client, error) {
	cfg := sarama.NewConfig()
	if c.Version != "" {
		v, err := sarama.ParseKafkaVersion(c.Version)
		if err != nil {
			return nil, err
		}
		cfg.Version = v
	}
	cfg.Producer.Return.Successes = true
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	return sarama.NewClient(strings.Split(c.Brokers, ","), cfg)
}

// destroyClient closes the Kafka client.
func destroyClient(cl sarama.Client) error {
	return cl.Close()
}
