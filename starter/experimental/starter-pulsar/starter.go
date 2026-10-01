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

	// Register multiple Pulsar clients as a group.
	// Each instance is created according to the configuration in "${spring.pulsar}".
	// This allows defining multiple Pulsar clients dynamically.
	gs.Module(gs.OnProperty("spring.pulsar.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.pulsar.instances}", func(name string, c Config) error {
			// The Driver param (index 3) is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver
			// bean when a company provides one, nil otherwise, and newClient
			// falls back to DefaultDriver); set → that bean name, and naming
			// a bean that does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.pulsar.instances."+name+".driver:=${spring.pulsar.default.driver:=?}}")),
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
			// (name, type), so it stays distinct from the raw pulsar.Client bean.
			// The load-test convention bean is a NULLABLE injection (index 1):
			// present when the application provides one, absent otherwise, and
			// NewDriver falls back to the canonical convention.
			r.Provide(func(cl pulsar.Client, prop traffic.Propagator) messaging.Driver {
				return NewDriver(cl, prop)
			}, gs.TagArg(name), gs.IndexArg(1, gs.TagArg("?"))).Name(name).Caller(1)
			return nil
		})
	})
}

// newClient creates a Pulsar client by dispatching to an optional Driver bean,
// which owns full client assembly (ClientOptions, authentication, TLS, metrics
// registry, and the governance executor); when no such bean exists the bundled
// DefaultDriver is used. The driver returns the client COMPLETE — the identity
// and the resilience executor are applied while it is built — so the starter
// never patches it afterwards. mgr and inj are the governance beans the
// container injects; the ctor bundles them into the [cloud.ClientParams] it
// hands the driver.
//
// Assembly completes before the probe: the client is probed (when FailFast is
// enabled) only after the driver has returned it, so a misconfigured broker
// list, bad credentials or TLS mismatch fail fast at startup instead of
// surfacing on the first produce/consume. A failed probe releases what was just
// assembled.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (pulsar.Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating pulsar client, url=%s fail-fast=%v", c.URL, c.FailFast)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: mgr, Fault: inj})
	if err != nil {
		return nil, err
	}

	// The driver returned the client complete — identity and the governance
	// executor applied while it was built. Probe it (when FailFast is enabled).
	// The probe goes straight to the raw client on purpose: it is a connectivity
	// check, not business traffic, so it must not spend limiter/breaker budget. A
	// failure abandons the client, so release what was just assembled.
	if c.FailFast {
		if _, err = cl.TopicPartitions(c.HealthCheckTopic); err != nil {
			log.Errorf(ctx.Context, log.TagAppDef, "pulsar: fail-fast probe failed on %s (topic=%s): %v", c.URL, c.HealthCheckTopic, err)
			closeResilience(cl)
			cl.Close()
			shutdownMetrics(cl)
			return nil, errutil.Explain(err, "pulsar broker probe failed on %s (topic=%s)", c.URL, c.HealthCheckTopic)
		}
	}
	log.Infof(ctx.Context, log.TagAppDef, "pulsar client initialized, url=%s", c.URL)
	return cl, nil
}
