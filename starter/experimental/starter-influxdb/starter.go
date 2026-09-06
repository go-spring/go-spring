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

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-influxdb/health"
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
	gs.Module(gs.OnProperty("spring.influxdb"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.influxdb}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor, so Init arms it
			// (InitMethod) and Destroy tears it down.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("?")),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(w *Client) *health.Indicator {
				return health2.NewClientHealth(name, w.Client)
			}, gs.TagArg(name)).Name("influxdb:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new InfluxDB client based on the provided
// configuration. The server is probed once at startup so that
// misconfiguration or an unreachable server fails fast rather than on first
// use.
func newClient(ctx *gs.ContextProvider, c Config, d Driver) (*Client, error) {
	log.Debugf(ctx.Context, starterTag, "creating influxdb client, url=%s org=%s bucket=%s", c.ServerURL, c.Org, c.Bucket)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	cl, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, err
	}
	w := &Client{Client: cl, cfg: c}
	// The DefaultDriver attaches a dynamic transport (its executor swapped in
	// by Init); pick it up so the wrapper can arm it. Custom drivers may not
	// install one — resilience is then simply unavailable for that client.
	if v, ok := dynamicTransports.LoadAndDelete(cl); ok {
		w.dyn = v.(*dynamicTransport)
	}
	if err := HealthCheck(ctx.Context, w.Client); err != nil {
		w.Client.Close()
		return nil, errutil.Explain(err, "failed to reach influxdb server %s", c.ServerURL)
	}
	return w, nil
}

// HealthCheck reports whether the InfluxDB server is reachable and healthy.
// It is a thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client influxdb2.Client) error {
	hc, err := client.Health(ctx)
	if err != nil {
		return err
	}
	return health2.HealthError(hc)
}
