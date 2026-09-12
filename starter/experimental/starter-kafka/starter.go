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

// starter.go is the DI glue: the gs registration, the newClient dispatch to an
// optional Driver bean, the startup ping + resilience wiring, and the destroy hook.
package StarterKafka

import (
	"context"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/messaging"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Kafka clients as a group.
	// Each instance is created according to the configuration in "${spring.kafka}".
	// This allows defining multiple Kafka clients dynamically.
	gs.Module(gs.OnProperty("spring.kafka.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.kafka.instances}", func(name string, c Config) error {
			// The Driver param (index 3) is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver
			// bean when a company provides one, nil otherwise, and newClient
			// falls back to DefaultDriver); set → that bean name, and naming
			// a bean that does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.kafka.instances."+name+".driver:=${spring.kafka.default.driver:=?}}")),
			).Name(name).Destroy(destroyClient).Caller(1)

			// Export the broker-neutral messaging.Driver over this client as a bean,
			// so consumers (starter-outbox-gorm, app pub/sub) autowire it like any
			// client bean. It shares the connection's bean name; beans are keyed by
			// (name, type), so it stays distinct from the raw *kgo.Client bean.
			r.Provide(func(cl *kgo.Client) messaging.Driver {
				return NewDriver(cl)
			}, gs.TagArg(name)).Name(name).Caller(1)
			return nil
		})
	})
}

// pingTimeout bounds the startup connectivity probe.
const pingTimeout = 10 * time.Second

// newClient creates a Kafka client by dispatching to an optional Driver bean,
// which owns full client assembly (hooks, SASL, TLS, producer options); when no
// such bean exists the bundled DefaultDriver is used. The kotel hooks emit
// producer/consumer spans and client metrics through the OTel globals that
// starter-otel installs; when starter-otel is absent those globals are no-ops,
// so this stays a zero-config opt-in that needs no per-component adaptation.
//
// After the client is built it is pinged so a misconfigured broker list, bad
// credentials or TLS mismatch fail fast at startup instead of surfacing on the
// first produce/consume, then the resilience executor is attached.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver) (*kgo.Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating kafka client, brokers=%s group=%s topic=%s", c.Brokers, c.Group, c.Topic)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "kafka: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create kafka client: %s", c.Brokers)
	}

	pingCtx, cancel := context.WithTimeout(ctx.Context, pingTimeout)
	defer cancel()
	if err = cl.Ping(pingCtx); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "kafka: ping failed: %v", err)
		cl.Close()
		return nil, errutil.Explain(err, "failed to ping kafka: %s", c.Brokers)
	}
	if err := applyResilience(cl, resilience.ResourceLabel("kafka", c.Brokers)); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "kafka: resilience setup failed: %v", err)
		cl.Close()
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "kafka client initialized, brokers=%s", c.Brokers)
	return cl, nil
}

// destroyClient flushes any buffered produce records before closing so
// in-flight messages are not dropped on shutdown. When a resilience executor is
// attached its Close releases any background resources of a production driver.
func destroyClient(cl *kgo.Client) error {
	closeResilience(cl)
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	flushErr := cl.Flush(ctx)
	cl.Close()
	if flushErr != nil {
		// A failed flush means buffered produce records were never delivered:
		// those messages are lost, so shutdown must not swallow the error.
		log.Errorf(context.Background(), log.TagAppDef,
			"kafka: flush before close failed, buffered messages may be LOST: %v", flushErr)
		return flushErr
	}
	return nil
}
