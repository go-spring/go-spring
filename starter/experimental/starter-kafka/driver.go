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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + the bundled DefaultDriver, which owns full client assembly
// (SASL/TLS/hooks/producer opts + kgo.NewClient). It mirrors starter-redigo's
// driver.go.
package StarterKafka

import (
	"context"
	"fmt"
	"strings"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
	"github.com/twmb/franz-go/plugin/kotel"
	"go-spring.org/cloud"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a Kafka client (a *kgo.Client). It is
// an OPTIONAL CONTAINER BEAN: a company or umbrella starter may provide its own
// Driver bean (its constructor returns StarterKafka.Driver); when none is
// present, starter-kafka falls back to the bundled [DefaultDriver] inside client
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// CreateClient returns the client COMPLETE. params supplies the container's
// service-governance capabilities (see [cloud.ClientParams]), which the
// driver applies WHILE it builds — by calling [AttachGovernance] — so nothing
// patches the client afterwards. Attaching is part of the contract, not an
// optional extra: the guard is indexed by the raw *kgo.Client the driver
// returns, and it is the only handle this package has on that client for the
// produce/consume path. A driver that returns without attaching leaves its
// client ungoverned.
//
// params is one struct rather than a parameter per capability so this interface —
// which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its fields simply ignores it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.kafka} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*kgo.Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new *kgo.Client from the provided configuration. It
// owns full client assembly — the kotel trace/metric hooks, the log bridge,
// consumer group/topic, SASL mechanism, TLS and producer options — so the returned
// client is complete. The startup ping is deliberately not here: it is the
// starter's lifecycle concern (see newClient in starter.go).
//
// The per-message access log is no longer a client hook (the observe hook was
// removed): the resilience executor emits it from the declared operation (see
// observe.go and command.go), so the hook set here is kotel's alone.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*kgo.Client, error) {
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
		tc, err := c.TLS.BuildClient()
		if err != nil {
			log.Errorf(ctx, log.TagAppDef, "kafka: build TLS failed: %v", err)
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
		return nil, err
	}
	// The driver builds the RAW client; params stays in the interface for the
	// drivers that need it, but the bundled default ignores it — governance is
	// applied one step up, by [NewClient] while it wraps the client in its
	// chain.
	_ = params // see above
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
		return nil, errutil.Explain(nil, "unsupported kafka sasl mechanism: %q", c.Mechanism)
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
		return nil, errutil.Explain(nil, "unsupported kafka required-acks: %q", c.RequiredAcks)
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
		return kgo.CompressionCodec{}, errutil.Explain(nil, "unsupported kafka compression: %q", name)
	}
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
