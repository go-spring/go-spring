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

// driver.go is the "construction seam" concept: the Driver interface + the
// bundled DefaultDriver, which owns full client assembly (broker URL, options,
// TLS, credentials, will + mqtt.NewClient). It mirrors starter-kafka's driver.go.
package StarterMQTT

import (
	"context"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go-spring.org/cloud"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create an MQTT client (a mqtt.Client). It is an
// OPTIONAL CONTAINER BEAN: a company or umbrella starter may provide its own
// Driver bean (its constructor returns StarterMQTT.Driver); when none is present,
// starter-mqtt falls back to the bundled [DefaultDriver] inside client assembly.
// A custom driver is a bean, so it may inject the configuration/beans it needs —
// e.g. company config bound from a properties file at wiring time.
//
// CreateClient returns the client COMPLETE: the connection assembly and the
// governance executor are both applied while it is built — see [AttachGovernance],
// which fixes the executor — and nothing patches the client afterwards.
//
// params supplies the container's service-governance capabilities (see
// cloud.ClientParams); it is one struct rather than a parameter per capability
// so this interface — which every company driver implements — stays stable as
// capabilities are added. A driver that has no use for one of its fields simply
// ignores it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.mqtt} is built through it, and per-instance differences are expressed
// through [Config].
//
// The connection-lifecycle callbacks are part of the driver's assembly: paho's
// Set*Handler options only take effect at construction, so connection-layer
// reporting (log + metric, see connStateCounter) cannot be layered on afterwards
// from the starter's side. A custom Driver that omits them therefore reports no
// connection events at all.
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (mqtt.Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient assembles a new mqtt.Client from the provided configuration. It
// owns full client assembly — the broker URL, client id, credentials, clean
// session, keep-alive, connect timeout, connection-lifecycle log bridge, TLS and
// will — and installs the governance executor, so the returned client is complete
// (see [Driver]). The broker connect/ping is the starter's lifecycle concern (see
// newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(c.Broker).
		SetClientID(c.ClientID).
		SetUsername(c.Username).
		SetPassword(c.Password).
		SetCleanSession(c.CleanSession).
		SetKeepAlive(c.KeepAlive).
		SetConnectTimeout(c.ConnectTimeout)

	// Connection-lifecycle events are counted as well as logged, from this one
	// place (see connStateCounter for why the pair cannot be split), so the
	// client's health shows up both alongside app logs and on a dashboard.
	connState := newConnStateCounter()
	opts.SetOnConnectHandler(func(_ mqtt.Client) {
		log.Info(ctx, log.TagAppDef, append(connState.record(ctx, connConnected),
			log.String("broker", c.Broker),
			log.Msg("mqtt connected"))...)
	})
	opts.SetConnectionLostHandler(func(_ mqtt.Client, err error) {
		log.Warn(ctx, log.TagAppDef, append(connState.record(ctx, connLost),
			log.Err(err),
			log.Msg("mqtt connection lost"))...)
	})
	opts.SetReconnectingHandler(func(_ mqtt.Client, _ *mqtt.ClientOptions) {
		log.Info(ctx, log.TagAppDef, append(connState.record(ctx, connReconnecting),
			log.String("broker", c.Broker),
			log.Msg("mqtt reconnecting"))...)
	})

	tlsCfg, err := c.TLS.BuildClient()
	if err != nil {
		log.Error(ctx, log.TagAppDef, err, log.Msg("mqtt: build TLS failed"))
		return nil, errutil.Explain(err, "mqtt: build TLS")
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	if c.Will.Topic != "" {
		opts.SetWill(c.Will.Topic, c.Will.Payload, c.Will.QoS, c.Will.Retained)
	}

	cl := mqtt.NewClient(opts)
	// Governance is applied HERE, while the client is built, so the returned
	// client is complete and nothing patches it afterwards. paho's mqtt.Client is
	// an interface with no room for an executor field, so the executor is held
	// beside it (see [AttachGovernance]); the broker scopes limiter/breaker
	// state per broker rather than per topic. A hand-built client passes the zero
	// Governance, whose executor degrades to observed-only rather than none.
	AttachGovernance(cl, c.Broker, params)
	return cl, nil
}
