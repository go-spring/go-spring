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
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/messaging"
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
				// The governance beans are NULLABLE injections: they exist
				// whenever starter-governance is in the container, which is the
				// normal case, and are absent from a container without it. Without
				// the "?" gs would treat an absent bean as a wiring error and the
				// app would not boot — turning "governance is off" into "governance
				// must be imported", which is not the contract. applyResilience
				// treats a nil bean as an unarmed authority, i.e. a transparent
				// pass-through.
				gs.IndexArg(4, gs.TagArg("?")), // mgr *resilience.Manager
				gs.IndexArg(5, gs.TagArg("?")), // inj *fault.Injector
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
// credentials, will). After the client is built it is connected so a misconfigured
// broker URL, bad credentials or TLS mismatch fail fast at startup instead of
// surfacing on the first publish/consume, then the resilience executor is
// attached from the injected governance beans (mgr, inj) — both nil in a
// standalone, non-gs call, which applyResilience treats as "governance off".
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (mqtt.Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating mqtt client, broker=%s client-id=%s", c.Broker, c.ClientID)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mqtt: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create mqtt client: %s", c.Broker)
	}

	// Build the package-level span-helper observers (StartPublishSpan /
	// StartConsumeSpan) from the current OTel meter provider; the first wired
	// client wins.
	buildObservers()

	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mqtt: connect failed broker=%s: %v", c.Broker, err)
		return nil, err
	}
	if err := applyResilience(client, resilience.ServiceLabel("mqtt", c.Broker), mgr, inj); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "mqtt: resilience setup failed: %v", err)
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
