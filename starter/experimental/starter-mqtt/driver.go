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

// driver.go is the "construction seam" concept: the Driver interface + registry +
// DefaultDriver, which owns full client assembly (broker URL, options, TLS,
// credentials, will + mqtt.NewClient). It mirrors starter-kafka's driver.go.
package StarterMQTT

import (
	"context"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Driver interface defines how to create an MQTT client (a mqtt.Client). It is
// the single extension point for customizing client assembly: a company (or the
// bundled DefaultDriver) implements it once and registers via RegisterDriver;
// callers select one through Config.Driver, which defaults to "DefaultDriver".
type Driver interface {
	CreateClient(ctx context.Context, c Config) (mqtt.Client, error)
}

// RegisterDriver registers an MQTT driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("mqtt driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient assembles a new mqtt.Client from the provided configuration. It
// owns full client assembly — the broker URL, client id, credentials, clean
// session, keep-alive, connect timeout, connection-lifecycle log bridge, TLS and
// will — but not the broker connect/ping or the resilience wiring, which are the
// starter's lifecycle concerns (see newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (mqtt.Client, error) {
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

	tlsCfg, err := c.TLS.Build()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "mqtt: build TLS failed: %v", err)
		return nil, errutil.Explain(err, "mqtt: build TLS")
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	if c.Will.Topic != "" {
		opts.SetWill(c.Will.Topic, c.Will.Payload, c.Will.QoS, c.Will.Retained)
	}

	return mqtt.NewClient(opts), nil
}
