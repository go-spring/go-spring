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

package StarterInfluxdb

import (
	"context"

	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var starterTag = log.RegisterAppTag("influxdb", "")

func init() {
	// Register multiple InfluxDB clients as a group, one per entry under
	// "${spring.influxdb}". A gs.Module (rather than gs.Group) is used so each
	// instance's *Client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration
	// to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.influxdb.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.influxdb.instances}", func(name string, c Config) error {
			// The Driver bean is selected by the entry's ${driver} key: unset →
			// "?" (nullable by-type — injects the single Driver bean when one is
			// provided, nil otherwise, and the ctor falls back to the bundled
			// DefaultDriver); set → that bean name, and naming a bean that does
			// not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.influxdb.instances."+name+".driver:=${spring.influxdb.default.driver:=?}}")),
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(3, gs.TagArg("")), // *resilience.Manager
				gs.IndexArg(4, gs.TagArg("")), // *fault.Injector
			).Name(name).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. Skipped when c.Health is
			// false.
			if c.Health {
				r.Provide(func(w *Client) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("influxdb:" + name).Caller(1)
			}
			return nil
		})
	})
}

// newClient creates a new InfluxDB client based on the provided configuration.
// The Driver returns the client COMPLETE — governance (the declaration+
// resilience transport) is applied while it is built — and when c.Ping is set
// this ctor afterwards probes the server once, so that misconfiguration or an
// unreachable server fails fast rather than on first use; the probe is off by
// default, so a server that is not up yet does not block startup. There is no
// Init hook: the client is complete when the driver returns it.
//
// mgr and inj are the governance beans the container injects (both nil in a
// standalone, non-gs call); the ctor bundles them into the
// [cloud.ClientParams] it hands the driver, which passes it to [NewClient] —
// so the client is assembled complete in one step, with the zero bundle
// degrading to an observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	log.Debugf(ctx.Context, starterTag, "creating influxdb client, url=%s org=%s bucket=%s", c.ServerURL, c.Org, c.Bucket)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	w, err := d.CreateClient(ctx.Context, c, cloud.ClientParams{Resilience: mgr, Fault: inj})
	if err != nil {
		return nil, err
	}
	// The Driver returned the client complete — governance applied while it was
	// built — so the probe below already runs through the assembled transport.
	// Fail fast: probe the server once at startup. The probe goes straight to the
	// raw client (a connectivity check, see [HealthCheck]); on failure the client
	// just assembled is released, executor and connection together. Off by
	// default (c.Ping): a server that is not up yet must not block startup.
	if c.Ping {
		if err := HealthCheck(ctx.Context, w); err != nil {
			_ = w.Destroy()
			return nil, errutil.Explain(err, "failed to reach influxdb server %s", c.ServerURL)
		}
	}
	return w, nil
}

// HealthCheck reports whether the InfluxDB server is reachable and healthy.
// It is a thin readiness probe suitable for wiring into a health endpoint, and
// the single place an InfluxDB liveness check is defined. It calls /health on
// the raw client, which verifies reachability, authentication setup and server
// status in one round trip; it must reflect the backend rather than the rate
// limiter, without feeding the operation metrics or the breaker's statistics.
func HealthCheck(ctx context.Context, client *Client) error {
	hc, err := client.Health(ctx)
	if err != nil {
		return err
	}
	return healthError(hc)
}
