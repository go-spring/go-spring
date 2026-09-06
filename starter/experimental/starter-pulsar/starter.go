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

// starter.go is the gs registration + glue concept of this starter: it declares
// the infra log tag and registers the per-instance Pulsar client group under
// "${spring.pulsar}", wiring each Config entry to newClient (the dispatch + probe
// + resilience wiring) and destroyClient (the lifecycle in client.go).
package StarterPulsar

import (
	"github.com/apache/pulsar-client-go/pulsar"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {

	// Register multiple Pulsar clients as a group.
	// Each instance is created according to the configuration in "${spring.pulsar}".
	// This allows defining multiple Pulsar clients dynamically.
	gs.Module(gs.OnProperty("spring.pulsar"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.pulsar}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("?")),
			).Name(name).Destroy(destroyClient).Caller(1)
			return nil
		})
	})
}

// newClient creates a Pulsar client by dispatching to an optional Driver bean,
// which owns full client assembly (ClientOptions, authentication, TLS, metrics
// registry); when no such bean exists the bundled DefaultDriver is used. After
// the client is built it is probed (when FailFast is enabled) so a misconfigured
// broker list, bad credentials or TLS mismatch fail fast at startup instead of
// surfacing on the first produce/consume, then the resilience executor is
// attached.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver) (pulsar.Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating pulsar client, url=%s fail-fast=%v", c.URL, c.FailFast)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, err
	}

	if c.FailFast {
		if _, err = cl.TopicPartitions(c.HealthCheckTopic); err != nil {
			log.Errorf(ctx.Context, log.TagAppDef, "pulsar: fail-fast probe failed on %s (topic=%s): %v", c.URL, c.HealthCheckTopic, err)
			cl.Close()
			shutdownMetrics(cl)
			return nil, errutil.Explain(err, "pulsar broker probe failed on %s (topic=%s)", c.URL, c.HealthCheckTopic)
		}
	}
	if err := applyResilience(c, cl, resilience.ResourceLabel("pulsar", c.URL)); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "pulsar: resilience setup failed: %v", err)
		cl.Close()
		shutdownMetrics(cl)
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "pulsar client initialized, url=%s", c.URL)
	return cl, nil
}
