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
// (newClient dispatch + broker-validation + resilience wiring, and destroyClient
// teardown). Client assembly itself lives in driver.go (Driver interface).
// The per-produce command seam lives in command.go.
package StarterKafkaSarama

import (
	"fmt"

	"github.com/IBM/sarama"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

// newClient creates a shared low-level sarama.Client by dispatching to the
// configured Driver, which owns full client assembly (version, SASL, TLS,
// producer options). Callers derive a SyncProducer, Consumer or ConsumerGroup
// from it via the sarama.*FromClient constructors, mirroring franz-go's
// single-client model. Producer success notifications are enabled so the client
// can back a SyncProducer, and the initial consumer offset defaults to the
// oldest available message.
//
// sarama.NewClient dials the seed brokers and fetches cluster metadata, so a
// misconfigured broker list, bad credentials or TLS mismatch fail fast at
// startup instead of surfacing on the first produce/consume. A defensive
// non-empty Brokers() check guards against future sarama changes that might
// otherwise swallow a fully empty cluster.
func newClient(ctx *gs.ContextProvider, name string, c Config) (sarama.Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating kafka sarama client, brokers=%s", c.Brokers)

	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx.Context, log.TagAppDef, "kafka driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "kafka driver not found: %s", c.Driver)
	}
	cl, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "kafka sarama: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create kafka client: %s", c.Brokers)
	}
	if len(cl.Brokers()) == 0 {
		cl.Close()
		log.Errorf(ctx.Context, log.TagAppDef, "kafka sarama: no brokers after metadata fetch: %s", c.Brokers)
		return nil, fmt.Errorf("kafka client has no brokers after metadata fetch: %s", c.Brokers)
	}
	if err := applyResilience(c, cl, resilience.ResourceLabel("kafka", c.Brokers)); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "kafka sarama: resilience setup failed: %v", err)
		_ = cl.Close()
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "kafka sarama client initialized, brokers=%s", c.Brokers)
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
