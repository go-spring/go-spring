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
	"context"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/log"
	"go-spring.org/spring/gs"
)

func init() {

	// Register multiple MQTT clients as a group.
	// Each instance is created according to the configuration in "${spring.mqtt.instances}".
	// This allows defining multiple MQTT clients dynamically.
	gs.Group("${spring.mqtt.instances}", newClient, destroyClient)
}

// newClient creates and connects an MQTT client based on the provided configuration.
func newClient(c Config) (mqtt.Client, error) {
	ctx := context.Background()

	opts := mqtt.NewClientOptions().
		AddBroker(c.Broker).
		SetClientID(c.ClientID).
		SetUsername(c.Username).
		SetPassword(c.Password).
		SetCleanSession(c.CleanSession).
		SetKeepAlive(c.KeepAlive).
		SetConnectTimeout(c.ConnectTimeout)

	// Bridge connection-lifecycle events into go-spring's log so the client's
	// health (default auto-reconnect stays on) shows up alongside app logs.
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		log.Infof(ctx, log.TagAppDef, "mqtt connected to %q", c.Broker)
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		log.Warnf(ctx, log.TagAppDef, "mqtt connection lost: %v", err)
	})
	opts.SetReconnectingHandler(func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		log.Infof(ctx, log.TagAppDef, "mqtt reconnecting to %q", c.Broker)
	})

	tlsCfg, err := c.TLS.tlsConfig()
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	if c.Will.Topic != "" {
		opts.SetWill(c.Will.Topic, c.Will.Payload, c.Will.QoS, c.Will.Retained)
	}

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		return nil, err
	}
	return client, nil
}

// destroyClient disconnects the MQTT client, waiting up to 250ms for
// in-flight work to complete.
func destroyClient(client mqtt.Client) error {
	client.Disconnect(250)
	return nil
}
