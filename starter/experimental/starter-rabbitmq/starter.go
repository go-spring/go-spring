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
	"go-spring.org/cloud/governance/resilience"
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
	gs.Module(gs.OnProperty("spring.rabbitmq"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.rabbitmq}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("?")),
			).Name(name).Destroy(destroyClient).Caller(1)
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
// Once the connection is built a probe channel is opened and closed to confirm
// the AMQP layer is usable, then close/block notifiers are bridged into
// go-spring's log so broker-driven events land alongside app logs, and finally
// the resilience executor is attached.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver) (*amqp.Connection, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating rabbitmq connection, url=%s vhost=%s", c.URL, c.Vhost)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	conn, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "rabbitmq: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create rabbitmq client: %s", c.URL)
	}

	// Confirm the AMQP channel layer is usable, not just the TCP handshake.
	ch, err := conn.Channel()
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "rabbitmq: open probe channel failed url=%s: %v", c.URL, err)
		_ = conn.Close()
		return nil, errutil.Explain(err, "failed to open probe channel: %s", c.URL)
	}
	if err := ch.Close(); err != nil {
		log.Warnf(ctx.Context, log.TagAppDef, "rabbitmq: close probe channel failed url=%s: %v", c.URL, err)
	}

	// Bridge connection-level events into go-spring's log. NotifyClose fires
	// once when the connection tears down (server-initiated or network drop);
	// NotifyBlocked fires whenever the broker throttles the publisher due to
	// resource alarms. Both channels are closed by amqp091 on connection
	// shutdown, so the goroutines exit naturally without leaking.
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))
	blockCh := conn.NotifyBlocked(make(chan amqp.Blocking, 1))
	go func() {
		for e := range closeCh {
			if e == nil {
				log.Infof(ctx.Context, log.TagAppDef, "rabbitmq connection closed: %s", c.URL)
				continue
			}
			log.Warnf(ctx.Context, log.TagAppDef, "rabbitmq connection closed: code=%d reason=%q server=%t recover=%t",
				e.Code, e.Reason, e.Server, e.Recover)
		}
	}()
	go func() {
		for b := range blockCh {
			if b.Active {
				log.Warnf(ctx.Context, log.TagAppDef, "rabbitmq connection blocked: %s", b.Reason)
			} else {
				log.Infof(ctx.Context, log.TagAppDef, "rabbitmq connection unblocked")
			}
		}
	}()

	log.Infof(ctx.Context, log.TagAppDef, "rabbitmq connection initialized, url=%s", c.URL)
	if err := applyResilience(c, conn, resilience.ResourceLabel("rabbitmq", c.Vhost, c.URL)); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "rabbitmq: resilience setup failed: %v", err)
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

// destroyClient closes the RabbitMQ connection. amqp091 closes the notifier
// channels as part of Close, which drains the log-bridging goroutines. When a
// resilience executor is attached its Close releases any background resources
// of a production driver.
func destroyClient(conn *amqp.Connection) error {
	closeResilience(conn)
	return conn.Close()
}
