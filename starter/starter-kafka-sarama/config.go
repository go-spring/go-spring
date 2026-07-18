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

// Config defines Kafka client configuration.
//
// It shares the "spring.kafka" property prefix with the franz-go based
// starter-kafka, so switching between the two implementations only requires
// swapping the imported package. The franz-go only "topic"/"group" keys are
// simply ignored here, and the sarama only "version" key is ignored there.
type Config struct {
	// Brokers is a comma-separated list of seed broker addresses,
	// e.g., "127.0.0.1:9092" or "host1:9092,host2:9092".
	Brokers string `value:"${brokers}"`

	// Version is the Kafka protocol version to negotiate, e.g. "3.7.0".
	// sarama requires this to match the target cluster for features such as
	// consumer groups to behave correctly. Defaults to sarama's own default.
	Version string `value:"${version:=}"`
}
