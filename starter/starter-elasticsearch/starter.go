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

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Elasticsearch clients as a group.
	// Each instance is created according to the configuration in "${spring.elasticsearch}".
	// This allows defining multiple elasticsearch instances dynamically.
	gs.Group("${spring.elasticsearch}", newClient, destroyClient)
}

// newClient creates a new Elasticsearch client based on the provided
// configuration. The cluster is probed once at startup so that misconfiguration
// or an unreachable cluster fails fast rather than on first use.
//
// When c.ServiceName is set, the node addresses are resolved once through the
// registered discovery backend (c.Discovery) and override c.Addresses; see the
// ServiceName field docs for the startup-only limitation. When c.ServiceName is
// empty the static Addresses (or CloudID) are used unchanged.
func newClient(c Config) (*elasticsearch.Client, error) {
	if c.ServiceName != "" {
		addrs, err := resolveAddresses(c)
		if err != nil {
			return nil, err
		}
		c.Addresses = addrs
	}

	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "elasticsearch driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create elasticsearch client")
	}
	if err := HealthCheck(client); err != nil {
		return nil, errutil.Explain(err, "failed to reach elasticsearch cluster")
	}
	return client, nil
}

// resolveAddresses resolves c.ServiceName through the registered discovery
// backend and returns the endpoints as "scheme://host:port" node addresses. It
// fails fast when no backend is registered or the service currently has no
// endpoints. This is a one-shot resolution at startup (see Config.ServiceName).
func resolveAddresses(c Config) ([]string, error) {
	backend, err := discovery.MustGet(c.Discovery)
	if err != nil {
		return nil, err
	}
	eps, err := backend.Resolve(context.Background(), c.ServiceName)
	if err != nil {
		return nil, errutil.Explain(err, "elasticsearch: resolve service %s", c.ServiceName)
	}
	if len(eps) == 0 {
		return nil, errutil.Explain(nil, "elasticsearch: discovery %q returned no endpoints for %q", c.Discovery, c.ServiceName)
	}
	addrs := make([]string, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, fmt.Sprintf("%s://%s", c.DiscoveryScheme, ep.Addr))
	}
	return addrs, nil
}

// HealthCheck reports whether the Elasticsearch cluster is reachable by issuing
// an Info request. It is a thin readiness probe suitable for wiring into a
// health endpoint. A context is always passed to Info because the transport's
// OpenTelemetry instrumentation derives its span from it and panics on a nil
// parent context.
func HealthCheck(client *elasticsearch.Client) error {
	res, err := client.Info(client.Info.WithContext(context.Background()))
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.IsError() {
		return fmt.Errorf("elasticsearch: info returned %s", res.Status())
	}
	return nil
}

// destroyClient releases the Elasticsearch client. The v8 client holds no
// closable resources (its transport uses net/http with idle-connection reuse),
// so there is nothing to close; this callback exists only to keep the
// group's lifecycle handling uniform with the other clients.
func destroyClient(client *elasticsearch.Client) error {
	return nil
}
