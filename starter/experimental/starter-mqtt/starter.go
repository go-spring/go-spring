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
	"go-spring.org/cloud/governance/resilience"
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
	gs.Module(gs.OnProperty("spring.mqtt"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.mqtt}", func(name string, c Config) error {
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("?")),
			).Name(name).Destroy(destroyClient).Caller(1)
			return nil
		})
	})
}

// newClient creates and connects an MQTT client by dispatching to the injected
// Driver bean, which owns full client assembly (broker URL, options, TLS,
// credentials, will). After the client is built it is connected so a misconfigured
// broker URL, bad credentials or TLS mismatch fail fast at startup instead of
// surfacing on the first publish/consume, then the resilience executor is
// attached.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver) (mqtt.Client, error) {
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
	if err := applyResilience(c, client, resilience.ResourceLabel("mqtt", c.Broker)); err != nil {
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
