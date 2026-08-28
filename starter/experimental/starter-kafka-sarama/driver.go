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

// driver.go is the "construction seam" concept: the Driver interface + registry +
// DefaultDriver, which owns full client assembly (version/SASL/TLS/producer opts
// + sarama.NewClient). It mirrors starter-kafka's driver.go.
package StarterKafkaSarama

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Driver interface defines how to create a Kafka client (a sarama.Client). It is
// the single extension point for customizing client assembly: a company (or the
// bundled DefaultDriver) implements it once and registers via RegisterDriver;
// callers select one through Config.Driver, which defaults to "DefaultDriver".
type Driver interface {
	CreateClient(ctx context.Context, c Config) (sarama.Client, error)
}

// RegisterDriver registers a Kafka driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("kafka driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new sarama.Client from the provided configuration. It
// owns full client assembly — the Kafka version negotiation, SASL mechanism,
// TLS transport and producer options — but not the startup broker-validation or
// the resilience wiring, which are the starter's lifecycle concerns (see
// newClient in client.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (sarama.Client, error) {
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
			log.Errorf(ctx, log.TagAppDef, "kafka sarama: build TLS failed: %v", err)
			return nil, errutil.Explain(err, "kafka: build TLS")
		}
		cfg.Net.TLS.Enable = true
		cfg.Net.TLS.Config = tc
	}

	if err := applyProducer(cfg, c.Producer); err != nil {
		return nil, err
	}

	return sarama.NewClient(strings.Split(c.Brokers, ","), cfg)
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
