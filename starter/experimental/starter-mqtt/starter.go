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
	"go-spring.org/cloud/governance"
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
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.mqtt.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.mqtt.instances", "spring.mqtt.default")
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
				// The governance center is the family's sole injection point: it hands
				// out the resilience/fault/loadbalance authorities.
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
// center is the governance center the container injects — the family's sole
// injection point; the ctor reads the resilience and fault authorities from it
// and bundles them into the cloud.ClientParams it hands the driver, which
// resolves the executor through it — the governed one when the resilience
// authority is present, the observed-only resilience.Unmanaged one otherwise (a
// standalone, non-gs caller passes nil, which is exactly "governance off").
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, center *governance.Center) (mqtt.Client, error) {
	// The client's identity rides on a context derived here: every line below
	// carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context, log.String("broker", c.Broker))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.String("client_id", c.ClientID),
			log.Msg("creating mqtt client"),
		}
	})

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{Resilience: center.Resilience(), Fault: center.Fault()})
	if err != nil {
		log.Error(cctx, log.TagAppDef, err, log.Msg("mqtt: create client failed"))
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
		log.Error(cctx, log.TagAppDef, err, log.Msg("mqtt: connect failed"))
		closeResilience(client)
		client.Disconnect(250)
		return nil, err
	}
	log.Info(cctx, log.TagAppDef, log.Msg("create mqtt client success"))
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
