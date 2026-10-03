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

// starter.go is the gs registration + glue concept: it binds each
// ${spring.nats} entry to a *Conn bean (built by newConn in driver.go) and its
// destroy callback ((*Conn).Destroy in client.go).
package StarterNats

import (
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/messaging"
	"go-spring.org/cloud/traffic"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple NATS connections as a group.
	// Each instance is created according to the configuration in
	// "${spring.nats}", allowing multiple connections dynamically.
	gs.Module(gs.OnProperty("spring.nats.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.nats.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.nats.instances", "spring.nats.default")
		return conf.BindEach(p, "${spring.nats.instances}", func(name string, c Config) error {
			// The Driver param (index 3) is selected by the entry's ${driver}
			// key: unset → "?" (nullable by-type — injects the single Driver
			// bean when a company provides one, nil otherwise, and newConn
			// falls back to DefaultDriver); set → that bean name, and naming
			// a bean that does not exist fails loud. The trailing governance
			// center (index 4) is the family's sole injection point: newConn
			// reads the resilience/fault authorities from it and bundles them
			// into the cloud.ClientParams it hands the Driver.
			r.Provide(newConn,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.nats.instances."+name+".driver:=${spring.nats.default.driver:=?}}")),
			).Name(name).Destroy((*Conn).Destroy).Caller(1)

			// Export the broker-neutral messaging.Driver over this connection as a
			// bean, so consumers (starter-outbox-gorm, app pub/sub) autowire it like
			// any client bean. It shares the connection's bean name; beans are keyed
			// by (name, type), so it stays distinct from the raw *Conn bean. The
			// traffic.Propagator (index 1) is a NULLABLE injection: the single
			// propagator bean when the application provides one, nil otherwise (the
			// driver then falls back to traffic.NewDefaultPropagator).
			r.Provide(func(conn *Conn, prop traffic.Propagator) messaging.Driver {
				return NewDriver(conn, prop)
			}, gs.TagArg(name), gs.IndexArg(1, gs.TagArg("?"))).Name(name).Caller(1)

			// Contribute a health indicator for this instance unless the user
			// disabled it (health=false), injecting the connection just
			// registered above by name. Its probe only calls HealthCheck (see
			// health.go).
			if c.Health {
				r.Provide(func(conn *Conn) *health.Indicator {
					return NewClientHealth(name, conn)
				}, gs.TagArg(name)).Name("nats:" + name)
			}
			return nil
		})
	})
}
