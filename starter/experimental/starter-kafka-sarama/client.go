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

// client.go is the "resource entity" concept of this starter: the low-level
// *sarama.Client every producer/consumer derives from, plus its lifecycle
// (newClient dispatch + broker-validation, and destroyClient teardown). Client
// assembly itself — including the governance executed from the driver — lives in
// driver.go (Driver interface).
// The per-produce command seam lives in command.go.
package StarterKafkaSarama

import (
	"github.com/IBM/sarama"
	"go-spring.org/cloud"
	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// newClient creates a shared low-level sarama.Client by dispatching to an
// optional Driver bean, which owns full client assembly (version, SASL, TLS,
// producer options); when no such bean exists the bundled DefaultDriver is used.
// Callers derive a SyncProducer, Consumer or ConsumerGroup
// from it via the sarama.*FromClient constructors, mirroring franz-go's
// single-client model. Producer success notifications are enabled so the client
// can back a SyncProducer, and the initial consumer offset defaults to the
// oldest available message.
//
// sarama.NewClient dials the seed brokers and fetches cluster metadata, so a
// misconfigured broker list, bad credentials or TLS mismatch fail fast at
// startup instead of surfacing on the first produce/consume. The Driver returns
// the client complete — identity and governance both applied while it was built
// (see [Driver.CreateClient]) — so this ctor only probes the metadata afterwards
// (when Ping is enabled): a defensive non-empty Brokers() test that guards
// against sarama changes which might otherwise swallow a fully empty cluster. A
// failed probe releases what was just assembled.
//
// center is the governance center the container injects — the family's sole
// injection point; the ctor reads the resilience and fault authorities from it
// and bundles them into the [cloud.ClientParams] it hands the driver, which
// attaches the executor from it — so the client is assembled complete in one
// step, with the zero bundle degrading to an observed-only, loudly-unmanaged
// executor. A standalone, non-gs caller passes nil, which is exactly "governance
// off".
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, center *governance.Center) (sarama.Client, error) {
	// The client's identity rides on a context derived here: every line below
	// carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context, log.String("brokers", c.Brokers))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{log.Msg("creating kafka sarama client")}
	})

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()})
	if err != nil {
		log.Error(cctx, log.TagAppDef,
			log.Err(err),
			log.Msg("kafka sarama: create client failed"))
		return nil, errutil.Explain(err, "failed to create kafka client: %s", c.Brokers)
	}
	// The Driver returned the client complete — governance was attached while it
	// was built. Only the defensive metadata probe runs after this (when Ping is
	// enabled); a failure releases what was assembled.
	if c.Ping {
		if len(cl.Brokers()) == 0 {
			closeResilience(cl)
			cl.Close()
			log.Error(cctx, log.TagAppDef,
				log.Msg("kafka sarama: no brokers after metadata fetch"))
			return nil, errutil.Explain(nil, "kafka client has no brokers after metadata fetch: %s", c.Brokers)
		}
	}
	log.Info(cctx, log.TagAppDef, log.Msg("kafka sarama client initialized"))
	return cl, nil
}

// destroyClient closes the Kafka client. sarama.Client itself buffers no
// in-flight records; SyncProducer/ConsumerGroup are derived beans and manage
// their own lifecycle. When a resilience executor is attached its Close
// releases any background resources of a production driver.
func destroyClient(cl sarama.Client) error {
	closeResilience(cl)
	return cl.Close()
}
