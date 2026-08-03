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

package StarterNeo4j

import (
	"context"
	"net/url"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/cloud/mesh"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Neo4j clients as a group.
	// Each instance is created according to the configuration in "${spring.neo4j}".
	// This allows defining multiple neo4j instances dynamically.
	gs.Group("${spring.neo4j}", newClient, destroyClient)
}

var starterTag = log.RegisterInfraTag("neo4j", "")

// resolvers tracks the discovery-backed resolver behind each client, so
// destroyClient can stop the background watch on teardown. The neo4j driver
// exposes no dialer injection point, so the Resolver is only consulted once at
// startup (to splice a live endpoint into the URI); the watch is kept running
// only to honor the unified lifecycle and is stopped on shutdown.
var resolvers sync.Map // neo4j.DriverWithContext -> *discovery.Resolver

// newClient creates a new Neo4j client based on the provided configuration.
// After the driver is built, connectivity is verified so that misconfiguration
// or an unreachable server fails fast at startup rather than on first query.
//
// Observability note: the neo4j-go-driver speaks the binary Bolt protocol and
// ships no official OpenTelemetry instrumentation, nor a command-monitor hook
// comparable to the SQL/MongoDB drivers, so there is no clean seam to emit
// client spans from the starter. Rather than hand-roll a fragile bridge, tracing
// is left to the application (wrap ExecuteQuery / session calls with an OTel span
// where needed). This is a documented gap, not an oversight.
//
// When c.ServiceName is set and mesh mode is off, a Resolver is built against
// the registered discovery backend (c.Discovery), one endpoint is picked, and
// its address is spliced into the URI host. Because the neo4j driver exposes no
// dialer injection point, this is a one-shot resolution at startup — the
// Resolver is kept alive only to keep the lifecycle uniform with the other
// client starters and is stopped on shutdown. In mesh mode the sidecar owns
// discovery+LB, so the URI is used unchanged. See Config.ServiceName.
func newClient(cp *gs.ContextProvider, c Config) (neo4j.DriverWithContext, error) {
	ctx := cp.Context
	log.Debugf(ctx, starterTag, "creating neo4j client, uri=%s service-name=%s driver=%s", c.URI, c.ServiceName, c.Driver)

	var resolver *discovery.Resolver
	if c.ServiceName != "" && !mesh.Enabled() {
		uri, r, err := resolveURI(ctx, c)
		if err != nil {
			log.Errorf(ctx, starterTag, "neo4j: resolve service-name failed: %v", err)
			return nil, err
		}
		resolver = r
		c.URI = uri
	}

	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx, starterTag, "neo4j driver not found: %s", c.Driver)
		if resolver != nil {
			_ = resolver.Stop()
		}
		return nil, errutil.Explain(nil, "neo4j driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(ctx, c)
	if err != nil {
		log.Errorf(ctx, starterTag, "neo4j: create client failed: %v", err)
		if resolver != nil {
			_ = resolver.Stop()
		}
		return nil, errutil.Explain(err, "failed to create neo4j client")
	}

	// Fail fast: verify the server is reachable before handing out the driver.
	vctx, cancel := verifyContext(ctx, c.SocketConnectTimeout)
	defer cancel()
	if err := client.VerifyConnectivity(vctx); err != nil {
		log.Errorf(ctx, starterTag, "neo4j: verify connectivity failed uri=%s: %v", c.URI, err)
		_ = client.Close(ctx)
		if resolver != nil {
			_ = resolver.Stop()
		}
		return nil, errutil.Explain(err, "failed to verify neo4j connectivity: %s", c.URI)
	}
	if resolver != nil {
		resolvers.Store(client, resolver)
	}
	log.Infof(ctx, starterTag, "neo4j client initialized, uri=%s", c.URI)
	return client, nil
}

// resolveURI builds a discovery Resolver for c.ServiceName, picks one live
// endpoint, and returns c.URI with its host replaced by that endpoint's address
// together with the Resolver (so the caller can stop its background watch on
// shutdown). It fails fast when no backend is registered or the service has no
// eligible endpoints. This is a one-shot pick because the neo4j driver exposes
// no dialer injection point (see Config.ServiceName).
func resolveURI(ctx context.Context, c Config) (string, *discovery.Resolver, error) {
	backend, err := discovery.GetDiscovery(c.Discovery)
	if err != nil {
		return "", nil, err
	}
	r, err := discovery.NewResolver(ctx, backend, c.ServiceName)
	if err != nil {
		return "", nil, errutil.Explain(err, "neo4j: resolve service %s", c.ServiceName)
	}
	ep, err := r.Pick()
	if err != nil {
		_ = r.Stop()
		return "", nil, errutil.Explain(err, "neo4j: pick endpoint for %s", c.ServiceName)
	}
	u, err := url.Parse(c.URI)
	if err != nil {
		_ = r.Stop()
		return "", nil, errutil.Explain(err, "neo4j: parse uri %s", c.URI)
	}
	u.Host = ep.Addr
	return u.String(), r, nil
}

// HealthCheck reports whether the Neo4j driver can reach the server. It is a
// thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client neo4j.DriverWithContext) error {
	return client.VerifyConnectivity(ctx)
}

// verifyContext derives a context for the startup connectivity check, bounded by
// the socket connect timeout when set so the probe cannot hang indefinitely.
func verifyContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(ctx, timeout)
}

// destroyClient closes the Neo4j client and stops any discovery resolver
// behind it.
func destroyClient(client neo4j.DriverWithContext) error {
	if v, ok := resolvers.LoadAndDelete(client); ok {
		_ = v.(*discovery.Resolver).Stop()
		log.Debugf(context.Background(), starterTag, "neo4j client destroyed, discovery resolver stopped")
	}
	return client.Close(context.Background())
}
