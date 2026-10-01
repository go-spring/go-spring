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

package StarterMQTT

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/cloud"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {

	// Register multiple MQTT clients as a group.
	// Each instance is created according to the configuration in "${spring.mqtt}".
	// This allows defining multiple MQTT clients dynamically.
	gs.Module(gs.OnProperty("spring.mqtt.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.mqtt.instances}", func(name string, c Config) error {
			// The Driver param (index 3) is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver
			// bean when a company provides one, nil otherwise, and newClient
			// falls back to DefaultDriver); set → that bean name, and naming
			// a bean that does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.mqtt.instances."+name+".driver:=${spring.mqtt.default.driver:=?}}")),
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
			// (name, type), so it stays distinct from the raw mqtt.Client bean.
			r.Provide(func(cl mqtt.Client) messaging.Driver {
				return NewDriver(cl)
			}, gs.TagArg(name)).Name(name).Caller(1)
			return nil
		})
	})
}

// newClient creates and connects an MQTT client by dispatching to the injected
// Driver bean, which owns full client assembly (broker URL, options, TLS,
// credentials, will) and installs the governance executor — so the client is
// complete when the driver returns it. Only then is it connected, so a
// misconfigured broker URL, bad credentials or TLS mismatch fail fast at startup
// instead of surfacing on the first publish/consume. A failed connect releases
// what was just assembled.
//
// mgr and inj are the governance beans the container injects; the ctor bundles
// them into the cloud.ClientParams it hands the driver, which resolves the
// executor through it — the governed one when mgr is present, the observed-only
// resilience.Unmanaged one otherwise (a standalone, non-gs caller passes nil,
// which is exactly "governance off").
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (mqtt.Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating mqtt client, broker=%s client-id=%s", c.Broker, c.ClientID)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: mgr, Fault: inj})
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mqtt: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create mqtt client: %s", c.Broker)
	}

	// The driver returned the client complete — assembly and governance both
	// applied while it was built — so there is no post-hoc resilience step. Now
	// connect: it is a connectivity check, not business traffic, so it must not
	// spend limiter/breaker budget. A failure abandons the client, so release
	// what was just assembled.
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mqtt: connect failed broker=%s: %v", c.Broker, err)
		closeResilience(client)
		client.Disconnect(250)
		return nil, err
	}
	log.Infof(ctx.Context, log.TagAppDef, "mqtt client initialized, broker=%s", c.Broker)
	return client, nil
}

// destroyClient disconnects the MQTT client, waiting up to 250ms for
// in-flight work to complete. When a resilience executor is attached its Close
// releases any background resources of a production driver.
func destroyClient(client mqtt.Client) error {
	closeResilience(client)
	client.Disconnect(250)
	return nil
}
