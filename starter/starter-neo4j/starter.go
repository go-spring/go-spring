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
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/spring/gs"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Neo4j clients as a group.
	// Each instance is created according to the configuration in "${spring.neo4j}".
	// This allows defining multiple neo4j instances dynamically.
	gs.Group("${spring.neo4j}", newClient, destroyClient)
}

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
// When c.ServiceName is set, the address is resolved once through the registered
// discovery backend (c.Discovery) and spliced into the URI host; see the
// ServiceName field docs for the startup-only limitation.
func newClient(c Config) (neo4j.DriverWithContext, error) {
	if c.ServiceName != "" {
		uri, err := resolveURI(c)
		if err != nil {
			return nil, err
		}
		c.URI = uri
	}

	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "neo4j driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create neo4j client")
	}

	// Fail fast: verify the server is reachable before handing out the driver.
	ctx, cancel := verifyContext(c.SocketConnectTimeout)
	defer cancel()
	if err := client.VerifyConnectivity(ctx); err != nil {
		_ = client.Close(context.Background())
		return nil, errutil.Explain(err, "failed to verify neo4j connectivity: %s", c.URI)
	}
	return client, nil
}

// resolveURI resolves c.ServiceName through the registered discovery backend and
// returns c.URI with its host replaced by a live endpoint. It fails fast when no
// backend is registered or the service currently has no endpoints. Healthy
// endpoints are preferred; backends that do not track health yield all endpoints
// as eligible. This is a one-shot resolution because the neo4j driver exposes no
// dialer injection point (see Config.ServiceName).
func resolveURI(c Config) (string, error) {
	backend, err := discovery.MustGet(c.Discovery)
	if err != nil {
		return "", err
	}
	eps, err := backend.Resolve(context.Background(), c.ServiceName)
	if err != nil {
		return "", errutil.Explain(err, "neo4j: resolve service %s", c.ServiceName)
	}
	if len(eps) == 0 {
		return "", errutil.Explain(nil, "neo4j: discovery %q returned no endpoints for %q", c.Discovery, c.ServiceName)
	}
	addr := eps[0].Addr
	for _, ep := range eps {
		if ep.Healthy {
			addr = ep.Addr
			break
		}
	}
	u, err := url.Parse(c.URI)
	if err != nil {
		return "", errutil.Explain(err, "neo4j: parse uri %s", c.URI)
	}
	u.Host = addr
	return u.String(), nil
}

// HealthCheck reports whether the Neo4j driver can reach the server. It is a
// thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client neo4j.DriverWithContext) error {
	return client.VerifyConnectivity(ctx)
}

// verifyContext derives a context for the startup connectivity check, bounded by
// the socket connect timeout when set so the probe cannot hang indefinitely.
func verifyContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

// destroyClient closes the Neo4j client.
func destroyClient(client neo4j.DriverWithContext) error {
	return client.Close(context.Background())
}
