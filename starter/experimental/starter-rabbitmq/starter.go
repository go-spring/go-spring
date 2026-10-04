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

package StarterRabbitMQ

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/cloud/governance"
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

	// Register multiple RabbitMQ connections as a group.
	// Each instance is created according to the configuration in "${spring.rabbitmq}".
	// This allows defining multiple RabbitMQ connections dynamically.
	gs.Module(gs.OnProperty("spring.rabbitmq.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.rabbitmq.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.rabbitmq.instances", "spring.rabbitmq.default")
		return conf.BindEach(p, "${spring.rabbitmq.instances}", func(name string, c Config) error {
			// The Driver param (index 3) is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver
			// bean when a company provides one, nil otherwise, and newClient
			// falls back to DefaultDriver); set → that bean name, and naming
			// a bean that does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.rabbitmq.instances."+name+".driver:=${spring.rabbitmq.default.driver:=?}}")),
				// The governance center is the family's sole injection point.
			).Name(name).Destroy((*Client).Close).Caller(1)

			// Export the broker-neutral messaging.Driver over this connection as a
			// bean, so consumers (starter-outbox-gorm, app pub/sub) autowire it like
			// any client bean. It shares the connection's bean name; beans are keyed
			// by (name, type), so it stays distinct from the raw *amqp.Connection bean.
			// The load-test convention bean is a NULLABLE injection (index 1):
			// present when the application provides one, absent otherwise, and
			// NewDriver falls back to the canonical convention.
			r.Provide(func(cl *Client, prop traffic.Propagator) messaging.Driver {
				return NewDriver(cl, prop)
			}, gs.TagArg(name), gs.IndexArg(1, gs.TagArg("?"))).Name(name).Caller(1)
			return nil
		})
	})
}

// newClient creates a RabbitMQ connection by dispatching to the injected Driver
// bean (falling back to the bundled DefaultDriver when none is present), which
// owns connection assembly (TLS build + amqp.Dial/DialConfig).
// amqp.Dial/DialConfig perform the TCP + AMQP handshake synchronously, so a bad
// URL, wrong credentials or TLS mismatch fail fast at startup rather than
// surfacing on the first channel/publish.
//
// Assembly completes before the probe: once the connection is built the
// close/block notifiers are bridged into go-spring's log (the observe half) and
// the resilience executor is attached, and only then (when Ping is enabled) is a
// probe channel opened and closed to confirm the AMQP layer is usable. A failed
// probe releases what was just assembled.
//
// center is the governance center bean — the family's sole injection point. It
// is required when linked: "governance off" is spring.governance.enabled=false,
// never an absent bean; a standalone, non-gs caller passes nil, which reads
// unarmed authorities — exactly "governance off".
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, center *governance.Center) (*Client, error) {
	// The connection's identity rides on a context derived here: every line
	// below carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context, log.String("url", c.URL))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.String("vhost", c.Vhost),
			log.Msg("creating rabbitmq connection"),
		}
	})

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	conn, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Error(cctx, log.TagAppDef, err, log.Msg("rabbitmq: create client failed"))
		return nil, errutil.Explain(err, "failed to create rabbitmq client: %s", c.URL)
	}

	// Bridge connection-level events into go-spring's log AND the connection
	// metric. NotifyClose fires once when the connection tears down
	// (server-initiated or network drop); NotifyBlocked fires whenever the
	// broker throttles the publisher due to resource alarms. Both channels are
	// closed by amqp091 on connection shutdown, so the goroutines exit
	// naturally without leaking.
	connState := newConnStateCounter()
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))
	blockCh := conn.NotifyBlocked(make(chan amqp.Blocking, 1))
	go func() {
		for e := range closeCh {
			if e == nil {
				log.Info(cctx, log.TagAppDef, append(connState.record(ctx.Context, connClosed),
					log.Msg("rabbitmq connection closed"))...)
				continue
			}
			log.Warn(cctx, log.TagAppDef, append(connState.record(ctx.Context, connClosed),
				log.Int("code", e.Code),
				log.String("reason", e.Reason),
				log.Bool("server", e.Server),
				log.Bool("recover", e.Recover),
				log.Msg("rabbitmq connection closed"))...)
		}
	}()
	go func() {
		for b := range blockCh {
			if b.Active {
				log.Warn(cctx, log.TagAppDef, append(connState.record(ctx.Context, connBlocked),
					log.String("reason", b.Reason),
					log.Msg("rabbitmq connection blocked"))...)
			} else {
				log.Info(cctx, log.TagAppDef, append(connState.record(ctx.Context, connUnblocked),
					log.Msg("rabbitmq connection unblocked"))...)
			}
		}
	}()

	// Assemble fully before probing: wrap the connection in its client — the
	// chain carries the governance, no registry involved.
	cl := NewClient(conn, resilience.ServiceLabel("rabbitmq", c.Vhost, c.URL), center)
	// Then confirm the AMQP channel layer is usable, not just the TCP handshake
	// (when Ping is enabled). The probe goes straight to the raw connection on
	// purpose: it is a connectivity check, not business traffic, so it must not
	// spend limiter/breaker budget. A failure abandons the connection, so release
	// what was just assembled.
	if c.Ping {
		ch, err := conn.Channel()
		if err != nil {
			log.Error(cctx, log.TagAppDef, err, log.Msg("rabbitmq: open probe channel failed"))
			_ = cl.Close()
			return nil, errutil.Explain(err, "failed to open probe channel: %s", c.URL)
		}
		if err := ch.Close(); err != nil {
			log.Warn(cctx, log.TagAppDef,
				log.Err(err),
				log.Msg("rabbitmq: close probe channel failed"))
		}
	}
	log.Info(cctx, log.TagAppDef, log.Msg("create rabbitmq connection success"))
	return cl, nil
}
