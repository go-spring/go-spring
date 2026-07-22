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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/franz-go/plugin/kotel"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Kafka clients as a group.
	// Each instance is created according to the configuration in "${spring.kafka}".
	// This allows defining multiple Kafka clients dynamically.
	gs.Group("${spring.kafka}", newClient, destroyClient)
}

// pingTimeout bounds the startup connectivity probe.
const pingTimeout = 10 * time.Second

// newClient creates a new Kafka client, bridged into go-spring's unified
// observability. The kotel hooks emit producer/consumer spans and client
// metrics through the OTel globals that starter-otel installs; when starter-otel
// is absent those globals are no-ops, so this stays a zero-config opt-in that
// needs no per-component adaptation.
//
// After the client is built it is pinged so a misconfigured broker list, bad
// credentials or TLS mismatch fail fast at startup instead of surfacing on the
// first produce/consume.
func newClient(c Config) (*kgo.Client, error) {
	kt := kotel.NewKotel(
		kotel.WithTracer(kotel.NewTracer()),
		kotel.WithMeter(kotel.NewMeter()),
	)
	opts := []kgo.Opt{
		kgo.SeedBrokers(strings.Split(c.Brokers, ",")...),
		kgo.WithHooks(kt.Hooks()...),
		kgo.WithLogger(newLogger()),
	}
	if c.Group != "" {
		opts = append(opts, kgo.ConsumerGroup(c.Group))
	}
	if c.Topic != "" {
		opts = append(opts, kgo.ConsumeTopics(c.Topic))
	}

	if c.SASL.Enabled {
		mech, err := saslMechanism(c.SASL)
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.SASL(mech))
	}

	if c.TLS.Enabled {
		tc, err := c.TLS.Build()
		if err != nil {
			return nil, errutil.Explain(err, "kafka: build TLS")
		}
		opts = append(opts, kgo.DialTLSConfig(tc))
	}

	producerOpts, err := producerOpts(c.Producer)
	if err != nil {
		return nil, err
	}
	opts = append(opts, producerOpts...)

	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create kafka client: %s", c.Brokers)
	}

	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if err = cl.Ping(ctx); err != nil {
		cl.Close()
		return nil, errutil.Explain(err, "failed to ping kafka: %s", c.Brokers)
	}
	return cl, nil
}

// saslMechanism builds the franz-go SASL mechanism from the configuration.
func saslMechanism(c SASLConfig) (sasl.Mechanism, error) {
	switch strings.ToLower(c.Mechanism) {
	case "", "plain":
		return plain.Auth{User: c.Username, Pass: c.Password}.AsMechanism(), nil
	case "scram-sha-256":
		return scram.Auth{User: c.Username, Pass: c.Password}.AsSha256Mechanism(), nil
	case "scram-sha-512":
		return scram.Auth{User: c.Username, Pass: c.Password}.AsSha512Mechanism(), nil
	default:
		return nil, fmt.Errorf("unsupported kafka sasl mechanism: %q", c.Mechanism)
	}
}

// producerOpts translates ProducerConfig into franz-go producer options.
func producerOpts(c ProducerConfig) ([]kgo.Opt, error) {
	var opts []kgo.Opt

	if c.Compression != "" {
		codec, err := compressionCodec(c.Compression)
		if err != nil {
			return nil, err
		}
		opts = append(opts, kgo.ProducerBatchCompression(codec))
	}

	switch strings.ToLower(c.RequiredAcks) {
	case "", "all":
		opts = append(opts, kgo.RequiredAcks(kgo.AllISRAcks()))
	case "leader":
		opts = append(opts, kgo.RequiredAcks(kgo.LeaderAck()), kgo.DisableIdempotentWrite())
	case "none":
		opts = append(opts, kgo.RequiredAcks(kgo.NoAck()), kgo.DisableIdempotentWrite())
	default:
		return nil, fmt.Errorf("unsupported kafka required-acks: %q", c.RequiredAcks)
	}

	if c.MaxBatchBytes > 0 {
		opts = append(opts, kgo.ProducerBatchMaxBytes(c.MaxBatchBytes))
	}
	if c.Linger > 0 {
		opts = append(opts, kgo.ProducerLinger(c.Linger))
	}
	return opts, nil
}

// compressionCodec maps a codec name to a franz-go CompressionCodec.
func compressionCodec(name string) (kgo.CompressionCodec, error) {
	switch strings.ToLower(name) {
	case "none":
		return kgo.NoCompression(), nil
	case "gzip":
		return kgo.GzipCompression(), nil
	case "snappy":
		return kgo.SnappyCompression(), nil
	case "lz4":
		return kgo.Lz4Compression(), nil
	case "zstd":
		return kgo.ZstdCompression(), nil
	default:
		return kgo.CompressionCodec{}, fmt.Errorf("unsupported kafka compression: %q", name)
	}
}

// destroyClient flushes any buffered produce records before closing so
// in-flight messages are not dropped on shutdown.
func destroyClient(cl *kgo.Client) error {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	_ = cl.Flush(ctx)
	cl.Close()
	return nil
}

// logger bridges franz-go's internal client logs into go-spring's log so
// connection events (broker connects, request failures, reconnects) show up
// alongside application logs.
type logger struct{}

func newLogger() kgo.Logger { return logger{} }

func (logger) Level() kgo.LogLevel { return kgo.LogLevelInfo }

func (logger) Log(level kgo.LogLevel, msg string, keyvals ...any) {
	ctx := context.Background()
	line := msg
	if len(keyvals) > 0 {
		line = fmt.Sprintf("%s %v", msg, keyvals)
	}
	switch level {
	case kgo.LogLevelError:
		log.Errorf(ctx, log.TagAppDef, "kafka: %s", line)
	case kgo.LogLevelWarn:
		log.Warnf(ctx, log.TagAppDef, "kafka: %s", line)
	default:
		log.Infof(ctx, log.TagAppDef, "kafka: %s", line)
	}
}
