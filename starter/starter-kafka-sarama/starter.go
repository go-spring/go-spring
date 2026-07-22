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
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Bridge sarama's package-level logger into go-spring's log so
	// connection events (broker connects, metadata refresh, request
	// failures, reconnects) show up alongside application logs.
	sarama.Logger = newSaramaLogger()

	// Register multiple Kafka clients as a group.
	// Each instance is created according to the configuration in "${spring.kafka}".
	// This allows defining multiple Kafka clients dynamically.
	gs.Group("${spring.kafka}", newClient, destroyClient)
}

// newClient creates a shared low-level sarama.Client. Callers derive a
// SyncProducer, Consumer or ConsumerGroup from it via the sarama.*FromClient
// constructors, mirroring franz-go's single-client model. Producer success
// notifications are enabled so the client can back a SyncProducer, and the
// initial consumer offset defaults to the oldest available message.
//
// sarama.NewClient dials the seed brokers and fetches cluster metadata, so a
// misconfigured broker list, bad credentials or TLS mismatch fail fast at
// startup instead of surfacing on the first produce/consume. A defensive
// non-empty Brokers() check guards against future sarama changes that might
// otherwise swallow a fully empty cluster.
func newClient(c Config) (sarama.Client, error) {
	cfg := sarama.NewConfig()
	if c.Version != "" {
		v, err := sarama.ParseKafkaVersion(c.Version)
		if err != nil {
			return nil, errutil.Explain(err, "invalid kafka version: %s", c.Version)
		}
		cfg.Version = v
	}
	cfg.Producer.Return.Successes = true
	cfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	if c.SASL.Enabled {
		if err := applySASL(cfg, c.SASL); err != nil {
			return nil, err
		}
	}

	if c.TLS.Enabled {
		tc, err := c.TLS.Build()
		if err != nil {
			return nil, errutil.Explain(err, "kafka: build TLS")
		}
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = tc
	}

	if err := applyProducer(cfg, c.Producer); err != nil {
		return nil, err
	}

	cl, err := sarama.NewClient(strings.Split(c.Brokers, ","), cfg)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create kafka client: %s", c.Brokers)
	}
	if len(cl.Brokers()) == 0 {
		cl.Close()
		return nil, fmt.Errorf("kafka client has no brokers after metadata fetch: %s", c.Brokers)
	}
	return cl, nil
}

// applySASL configures cfg.Net.SASL fields for the requested mechanism.
// Unsupported mechanisms return an explicit error rather than silently
// falling back to PLAIN.
func applySASL(cfg *sarama.Config, c SASLConfig) error {
	cfg.Net.SASL.Enable = true
	cfg.Net.SASL.User = c.Username
	cfg.Net.SASL.Password = c.Password
	switch strings.ToLower(c.Mechanism) {
	case "", "plain":
		cfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
	case "scram-sha-256":
		cfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
		cfg.Net.SASL.SCRAMClientGeneratorFunc = scramSHA256Generator
	case "scram-sha-512":
		cfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
		cfg.Net.SASL.SCRAMClientGeneratorFunc = scramSHA512Generator
	default:
		return fmt.Errorf("unsupported kafka sasl mechanism: %q", c.Mechanism)
	}
	return nil
}

// applyProducer translates ProducerConfig into cfg.Producer fields.
func applyProducer(cfg *sarama.Config, c ProducerConfig) error {
	if c.Compression != "" {
		codec, err := compressionCodec(c.Compression)
		if err != nil {
			return err
		}
		cfg.Producer.Compression = codec
	}
	switch strings.ToLower(c.RequiredAcks) {
	case "", "all":
		cfg.Producer.RequiredAcks = sarama.WaitForAll
	case "leader":
		cfg.Producer.RequiredAcks = sarama.WaitForLocal
	case "none":
		cfg.Producer.RequiredAcks = sarama.NoResponse
	default:
		return fmt.Errorf("unsupported kafka required-acks: %q", c.RequiredAcks)
	}
	return nil
}

// compressionCodec maps a codec name to a sarama.CompressionCodec.
func compressionCodec(name string) (sarama.CompressionCodec, error) {
	switch strings.ToLower(name) {
	case "none":
		return sarama.CompressionNone, nil
	case "gzip":
		return sarama.CompressionGZIP, nil
	case "snappy":
		return sarama.CompressionSnappy, nil
	case "lz4":
		return sarama.CompressionLZ4, nil
	case "zstd":
		return sarama.CompressionZSTD, nil
	default:
		return sarama.CompressionNone, fmt.Errorf("unsupported kafka compression: %q", name)
	}
}

// destroyClient closes the Kafka client. sarama.Client itself buffers no
// in-flight records; SyncProducer/ConsumerGroup are derived beans and manage
// their own lifecycle.
func destroyClient(cl sarama.Client) error {
	return cl.Close()
}
