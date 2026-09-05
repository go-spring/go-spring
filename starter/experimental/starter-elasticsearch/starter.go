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

package StarterElasticsearch

import (
	"context"
	"fmt"

	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/mesh"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	health2 "go-spring.org/starter-elasticsearch/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Elasticsearch clients as a group, one per entry under
	// "${spring.elasticsearch}". A gs.Module (rather than gs.Group) is used so
	// each instance's *elasticsearch.Client bean can be paired with a
	// health.Indicator registered under the same name — and to attach the
	// file:line of this registration to the bean for diagnostics.
	gs.Module(gs.OnProperty("spring.elasticsearch"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.elasticsearch}", func(name string, c Config) error {
			// The wrapper bean owns the resilience executor + discovery watch, so
			// Init arms it (InitMethod) and Close tears it down (Destroy).
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(c)),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name. The wrapper is what is
			// autowired; the embedded *elasticsearch.Client is handed to the
			// indicator.
			r.Provide(func(w *Client) *health.Indicator {
				return health2.NewClientHealth(name, w.Client)
			}, gs.TagArg(name)).Name("elasticsearch:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new Elasticsearch client based on the provided
// configuration. The cluster is probed once at startup so that misconfiguration
// or an unreachable cluster fails fast rather than on first use.
//
// When c.ServiceName is set and mesh mode is off, a by-name loader is built
// against the registered discovery backend (c.Discovery), its current live
// endpoint snapshot is turned into "scheme://host:port" node addresses, and
// those override c.Addresses. Because the elasticsearch client exposes no dialer
// injection point, this is a one-shot resolution at startup; the loader has no
// background watch and no resources to release. In mesh mode the sidecar owns
// discovery+LB, so the static Addresses (or CloudID) are used unchanged. See
// Config.ServiceName.
func newClient(ctx *gs.ContextProvider, c Config) (*Client, error) {
	if c.ServiceName != "" && !mesh.Enabled() {
		addrs, err := resolveAddresses(ctx.Context, c)
		if err != nil {
			return nil, err
		}
		c.Addresses = addrs
	}

	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "elasticsearch driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create elasticsearch client")
	}
	w := &Client{Client: client, cfg: c}
	// The DefaultDriver attaches a dynamic transport (its executor swapped in by
	// Init); pick it up so the wrapper can arm it. Custom drivers may
	// not install one - resilience is then simply unavailable for that client.
	if v, ok := dynamicTransports.LoadAndDelete(client); ok {
		w.dyn = v.(*dynamicTransport)
	}
	if err := HealthCheck(ctx.Context, w); err != nil {
		_ = client.Close(context.Background())
		return nil, errutil.Explain(err, "failed to reach elasticsearch cluster")
	}
	return w, nil
}

// HealthCheck reports whether the Elasticsearch cluster is reachable by issuing
// an Info request. It is a thin readiness probe suitable for wiring into a
// health endpoint. A context is always passed to Info because the transport's
// OpenTelemetry instrumentation derives its span from it and panics on a nil
// parent context.
func HealthCheck(ctx context.Context, client *Client) error {
	res, err := client.Info(client.Info.WithContext(ctx))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("elasticsearch: info returned %s", res.Status())
	}
	return nil
}
