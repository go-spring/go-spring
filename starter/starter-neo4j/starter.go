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
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/spring/gs"
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
func newClient(c Config) (neo4j.DriverWithContext, error) {
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
