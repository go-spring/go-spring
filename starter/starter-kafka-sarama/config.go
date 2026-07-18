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

	// SASL configures SASL authentication, disabled by default.
	SASL SASLConfig `value:"${sasl}"`

	// TLS configures transport encryption, disabled by default.
	TLS TLSConfig `value:"${tls}"`

	// Producer tunes producer-side compression and acks.
	Producer ProducerConfig `value:"${producer}"`
}

// SASLConfig configures SASL authentication. It is shared, by property name,
// with the franz-go based starter-kafka so switching implementations does
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

// TLSConfig configures TLS transport encryption. When Enabled is true the
// client dials brokers over TLS; certificate files are optional and only
// needed for a custom CA or mutual TLS.
type TLSConfig struct {
	// Enabled turns on TLS for broker connections, default is false.
	Enabled bool `value:"${enabled:=false}"`

	// CACert is the path to a PEM CA bundle used to verify the broker
	// certificate; empty uses the system roots.
	CACert string `value:"${ca-cert:=}"`

	// ClientCert and ClientKey are the PEM client certificate/key pair for
	// mutual TLS; both empty disables client authentication.
	ClientCert string `value:"${client-cert:=}"`
	ClientKey  string `value:"${client-key:=}"`

	// InsecureSkipVerify disables broker certificate verification. Never
	// enable it outside development, default is false.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`
}

// ProducerConfig tunes the producer. Zero values leave sarama's own defaults
// in place.
type ProducerConfig struct {
	// Compression is the batch compression codec: "none", "gzip", "snappy",
	// "lz4" or "zstd", default is empty (sarama default: none).
	Compression string `value:"${compression:=}"`

	// RequiredAcks is the ack policy: "all" (all in-sync replicas), "leader"
	// (leader only) or "none". Default is "all".
	RequiredAcks string `value:"${required-acks:=all}"`
}
