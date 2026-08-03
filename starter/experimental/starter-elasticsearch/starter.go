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
	"sync"

	"github.com/elastic/go-elasticsearch/v8"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/mesh"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Elasticsearch clients as a group.
	// Each instance is created according to the configuration in "${spring.elasticsearch}".
	// This allows defining multiple elasticsearch instances dynamically.
	gs.Group("${spring.elasticsearch}", newClient, destroyClient)
}

var starterTag = log.RegisterInfraTag("elasticsearch", "")

// resolvers tracks the discovery-backed resolver behind each client, so
// destroyClient can stop the background watch on teardown. The elasticsearch
// client exposes no dialer injection point, so the Resolver is only consulted
// once at startup (to derive the node address list); the watch is kept running
// only to honor the unified lifecycle and is stopped on shutdown.
var resolvers sync.Map // *elasticsearch.Client -> *discovery.Resolver

// newClient creates a new Elasticsearch client based on the provided
// configuration. The cluster is probed once at startup so that misconfiguration
// or an unreachable cluster fails fast rather than on first use.
//
// When c.ServiceName is set and mesh mode is off, a Resolver is built against
// the registered discovery backend (c.Discovery), its current endpoint snapshot
// is turned into "scheme://host:port" node addresses, and those override
// c.Addresses. Because the elasticsearch client exposes no dialer injection
// point, this is a one-shot resolution at startup — the Resolver is kept alive
// only to keep the lifecycle uniform with the other client starters and is
// stopped on shutdown. In mesh mode the sidecar owns discovery+LB, so the static
// Addresses (or CloudID) are used unchanged. See Config.ServiceName.
func newClient(cp *gs.ContextProvider, c Config) (*elasticsearch.Client, error) {
	ctx := cp.Context
	var resolver *discovery.Resolver
	if c.ServiceName != "" && !mesh.Enabled() {
		addrs, r, err := resolveAddresses(ctx, c)
		if err != nil {
			return nil, err
		}
		resolver = r
		c.Addresses = addrs
	}

	d, ok := driverRegistry[c.Driver]
	if !ok {
		if resolver != nil {
			_ = resolver.Stop()
		}
		return nil, errutil.Explain(nil, "elasticsearch driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(ctx, c)
	if err != nil {
		if resolver != nil {
			_ = resolver.Stop()
		}
		return nil, errutil.Explain(err, "failed to create elasticsearch client")
	}
	if err := HealthCheck(ctx, client); err != nil {
		_ = client.Close(context.Background())
		if resolver != nil {
			_ = resolver.Stop()
		}
		return nil, errutil.Explain(err, "failed to reach elasticsearch cluster")
	}
	if resolver != nil {
		resolvers.Store(client, resolver)
	}
	return client, nil
}

// resolveAddresses builds a discovery Resolver for c.ServiceName and returns the
// current endpoint snapshot as "scheme://host:port" node addresses together with
// the Resolver (so the caller can stop its background watch on shutdown). It
// fails fast when no backend is registered or the service has no endpoints.
func resolveAddresses(ctx context.Context, c Config) ([]string, *discovery.Resolver, error) {
	backend, err := discovery.GetDiscovery(c.Discovery)
	if err != nil {
		return nil, nil, err
	}
	r, err := discovery.NewResolver(ctx, backend, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, nil, errutil.Explain(err, "elasticsearch: resolve service %s", c.ServiceName)
	}
	eps := r.Endpoints()
	if len(eps) == 0 {
		_ = r.Stop()
		return nil, nil, errutil.Explain(nil, "elasticsearch: discovery %q returned no endpoints for %q", c.Discovery, c.ServiceName)
	}
	addrs := make([]string, 0, len(eps))
	for _, ep := range eps {
		addrs = append(addrs, fmt.Sprintf("%s://%s", c.DiscoveryScheme, ep.Addr))
	}
	return addrs, r, nil
}

// HealthCheck reports whether the Elasticsearch cluster is reachable by issuing
// an Info request. It is a thin readiness probe suitable for wiring into a
// health endpoint. A context is always passed to Info because the transport's
// OpenTelemetry instrumentation derives its span from it and panics on a nil
// parent context.
func HealthCheck(ctx context.Context, client *elasticsearch.Client) error {
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

// destroyClient releases the Elasticsearch client and stops any discovery
// resolver behind it.
func destroyClient(client *elasticsearch.Client) error {
	if v, ok := resolvers.LoadAndDelete(client); ok {
		_ = v.(*discovery.Resolver).Stop()
		log.Debugf(context.Background(), starterTag, "elasticsearch client destroyed, discovery resolver stopped")
	}
	return client.Close(context.Background())
}
