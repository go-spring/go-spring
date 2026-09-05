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
// DefaultDriver, which owns connection assembly (TLS build + amqp dial). It
// mirrors starter-kafka's driver.go.
package StarterRabbitMQ

import (
	"context"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Driver interface defines how to create a RabbitMQ connection (an
// *amqp.Connection). It is the single extension point for customizing client
// assembly: a company (or the bundled DefaultDriver) implements it once and
// registers via RegisterDriver; callers select one through Config.Driver, which
// defaults to "DefaultDriver".
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*amqp.Connection, error)
}

// RegisterDriver registers a RabbitMQ driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("rabbitmq driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new *amqp.Connection from the provided configuration.
// It owns connection assembly — the TLS build and the amqp.Dial/DialConfig
// (which performs the TCP + AMQP handshake synchronously) — but not the probe
// channel, the notifier log bridge, or the resilience wiring, which are the
// starter's lifecycle concerns (see newClient in starter.go).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*amqp.Connection, error) {
	tc, err := c.TLS.BuildClient()
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "rabbitmq: build TLS failed: %v", err)
		return nil, errutil.Explain(err, "rabbitmq: build TLS")
	}
	useTLS := tc != nil || strings.HasPrefix(strings.ToLower(c.URL), "amqps://")

	var conn *amqp.Connection
	if useTLS || c.Heartbeat > 0 || c.Vhost != "" {
		cfg := amqp.Config{
			Vhost:     c.Vhost,
			Heartbeat: c.Heartbeat,
		}
		if tc != nil {
			cfg.TLSClientConfig = tc
		}
		conn, err = amqp.DialConfig(c.URL, cfg)
	} else {
		conn, err = amqp.Dial(c.URL)
	}
	if err != nil {
		log.Errorf(ctx, log.TagAppDef, "rabbitmq: dial failed url=%s: %v", c.URL, err)
		return nil, errutil.Explain(err, "failed to dial rabbitmq: %s", c.URL)
	}
	return conn, nil
}
