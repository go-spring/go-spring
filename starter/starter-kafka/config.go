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
	"time"

	"go-spring.org/stdlib/starter"
)

// Config defines Kafka client configuration.
type Config struct {
	// Brokers is a comma-separated list of seed broker addresses,
	// e.g., "127.0.0.1:9092" or "host1:9092,host2:9092".
	Brokers string `value:"${brokers}"`

	// Topic is the topic to consume from, default is empty.
	// Leave it empty for a produce-only client.
	Topic string `value:"${topic:=}"`

	// Group is the consumer group id, default is empty.
	// It is required to consume as part of a group.
	Group string `value:"${group:=}"`

	// SASL configures SASL authentication, disabled by default.
	SASL SASLConfig `value:"${sasl}"`

	// TLS configures transport encryption, disabled by default. It uses the
	// shared stdlib/starter TLS block so property keys are uniform across
	// starters (cert-file / key-file / ca-file / server-name /
	// insecure-skip-verify).
	TLS starter.TLSConfig `value:"${tls}"`

	// Producer tunes producer-side batching, compression and acks.
	Producer ProducerConfig `value:"${producer}"`
}

// SASLConfig configures SASL authentication. It is shared, by property name,
// with the sarama based starter-kafka-sarama so switching implementations does
// not require re-keying the credentials.
type SASLConfig struct {
	// Enabled turns on SASL authentication, default is false.
	Enabled bool `value:"${enabled:=false}"`

	// Mechanism is the SASL mechanism: "plain", "scram-sha-256" or
	// "scram-sha-512", default is "plain".
	Mechanism string `value:"${mechanism:=plain}"`

	// Username is the SASL username.
	Username string `value:"${username:=}"`

	// Password is the SASL password.
	Password string `value:"${password:=}"`
}

// ProducerConfig tunes the producer. Zero values leave franz-go's own
// defaults in place.
type ProducerConfig struct {
	// Compression is the batch compression codec: "none", "gzip", "snappy",
	// "lz4" or "zstd", default is empty (franz-go default: none).
	Compression string `value:"${compression:=}"`

	// RequiredAcks is the ack policy: "all" (all in-sync replicas), "leader"
	// (leader only) or "none". Default is "all". Anything other than "all"
	// disables idempotent writes as required by the protocol.
	RequiredAcks string `value:"${required-acks:=all}"`

	// MaxBatchBytes caps the size of a produce batch in bytes,
	// 0 keeps the franz-go default.
	MaxBatchBytes int32 `value:"${max-batch-bytes:=0}"`

	// Linger is how long to wait to accumulate a batch before sending,
	// 0 keeps the franz-go default.
	Linger time.Duration `value:"${linger:=0s}"`
}
