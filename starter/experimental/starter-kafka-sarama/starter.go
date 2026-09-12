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

// starter.go is the "registration + glue" concept of this starter: the gs.Module
// that binds one *sarama.Client per entry under ${spring.kafka-sarama}, and the
// bridge from sarama's package-level logger into go-spring's log.
package StarterKafkaSarama

import (
	"github.com/IBM/sarama"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Bridge sarama's package-level logger into go-spring's log so
	// connection events (broker connects, metadata refresh, request
	// failures, reconnects) show up alongside application logs.
	sarama.Logger = newSaramaLogger()

	// Register multiple Kafka clients as a group.
	// Each instance is created according to the configuration in "${spring.kafka-sarama}".
	// This allows defining multiple Kafka (sarama) clients dynamically.
	gs.Module(gs.OnProperty("spring.kafka-sarama.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.kafka-sarama.instances}", func(name string, c Config) error {
			// The Driver param (index 3) is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver
			// bean when a company provides one, nil otherwise, and newClient
			// falls back to DefaultDriver); set → that bean name, and naming
			// a bean that does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.kafka-sarama.instances."+name+".driver:=${spring.kafka-sarama.default.driver:=?}}")),
			).Name(name).Destroy(destroyClient).Caller(1)
			return nil
		})
	})
}
