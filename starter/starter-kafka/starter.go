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

package StarterKafka

import (
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/spring/gs"
)

func init() {

	// Register a single default Kafka client.
	// This client will only be created if the property "spring.kafka.brokers" is set.
	// It uses the configuration tagged with "${spring.kafka}".
	gs.Provide(newClient, gs.TagArg("${spring.kafka}")).
		Condition(gs.OnProperty("spring.kafka.brokers")).
		Destroy(destroyClient)

	// Register multiple Kafka clients as a group.
	// Each instance is created according to the configuration in "${spring.kafka.instances}".
	// This allows defining multiple Kafka clients dynamically.
	gs.Group("${spring.kafka.instances}", newClient, destroyClient)
}

// newClient creates a new Kafka client based on the provided configuration.
func newClient(c Config) (*kgo.Client, error) {
	opts := []kgo.Opt{
		kgo.SeedBrokers(strings.Split(c.Brokers, ",")...),
	}
	if c.Group != "" {
		opts = append(opts, kgo.ConsumerGroup(c.Group))
	}
	if c.Topic != "" {
		opts = append(opts, kgo.ConsumeTopics(c.Topic))
	}
	return kgo.NewClient(opts...)
}

// destroyClient closes the Kafka client.
func destroyClient(cl *kgo.Client) error {
	cl.Close()
	return nil
}
