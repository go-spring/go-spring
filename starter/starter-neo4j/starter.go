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
func newClient(c Config) (neo4j.DriverWithContext, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "neo4j driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create neo4j client")
	}
	return client, nil
}

// destroyClient closes the Neo4j client.
func destroyClient(client neo4j.DriverWithContext) error {
	return client.Close(context.Background())
}
