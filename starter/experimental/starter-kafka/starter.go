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
	"go-spring.org/cloud"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/traffic"
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
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.kafka.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.kafka.instances", "spring.kafka.default")
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
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(4, gs.TagArg("")), // *resilience.Manager
				gs.IndexArg(5, gs.TagArg("")), // *fault.Injector
			).Name(name).Destroy(destroyClient).Caller(1)

			// Export the broker-neutral messaging.Driver over this client as a bean,
			// so consumers (starter-outbox-gorm, app pub/sub) autowire it like any
			// client bean. It shares the connection's bean name; beans are keyed by
			// (name, type), so it stays distinct from the raw *kgo.Client bean. The
			// traffic.Propagator (index 1) is a NULLABLE injection: the single
			// propagator bean when the application provides one, nil otherwise (the
			// driver then falls back to traffic.NewDefaultPropagator).
			r.Provide(func(cl *kgo.Client, prop traffic.Propagator) messaging.Driver {
				return NewDriver(cl, prop)
			}, gs.TagArg(name), gs.IndexArg(1, gs.TagArg("?"))).Name(name).Caller(1)
			return nil
		})
	})
}

// pingTimeout bounds the startup connectivity probe.
const pingTimeout = 10 * time.Second

// newClient creates a Kafka client by dispatching to an optional Driver bean,
// which owns full client assembly (hooks, SASL, TLS, producer options) and,
// last of all, attaches the governance bundle — so the client it returns is
// complete. When no such bean exists the bundled DefaultDriver is used. The
// kotel hooks emit producer/consumer spans and client metrics through the OTel
// globals that starter-otel installs; when starter-otel is absent those globals
// are no-ops, so this stays a zero-config opt-in that needs no per-component
// adaptation.
//
// Assembly happens in one place and one order: the driver builds and completes
// the client (governance attached inside it), and only then is it pinged (when
// Ping is enabled), so a misconfigured broker list, bad credentials or TLS
// mismatch fail fast at startup instead of surfacing on the first
// produce/consume. A failed ping releases the executor the driver attached
// before abandoning the client.
//
// mgr and inj are the governance beans gs injects; they are bundled into the
// [cloud.ClientParams] handed to the driver, which applies them while
// building.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver,
	mgr *resilience.Manager, inj *fault.Injector) (*kgo.Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating kafka client, brokers=%s group=%s topic=%s", c.Brokers, c.Group, c.Topic)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c, cloud.ClientParams{Resilience: mgr, Fault: inj})
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "kafka: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create kafka client: %s", c.Brokers)
	}

	// The driver returned the client complete — governance attached while it was
	// built. Then probe connectivity (when Ping is enabled). The probe goes
	// straight to the raw client on purpose: it is a connectivity check, not
	// business traffic, so it must not spend limiter/breaker budget. A failure
	// abandons the client, so release the executor the driver attached.
	if c.Ping {
		pingCtx, cancel := context.WithTimeout(ctx.Context, pingTimeout)
		defer cancel()
		if err = cl.Ping(pingCtx); err != nil {
			log.Errorf(ctx.Context, log.TagAppDef, "kafka: ping failed: %v", err)
			closeResilience(cl)
			cl.Close()
			return nil, errutil.Explain(err, "failed to ping kafka: %s", c.Brokers)
		}
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
