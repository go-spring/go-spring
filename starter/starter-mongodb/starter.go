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

package StarterMongoDB

import (
	"context"
	"fmt"
	"time"

	"go-spring.org/spring/gs"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func init() {

	// Register multiple MongoDB clients as a group.
	// Each instance is created according to the configuration in "${spring.mongodb.instances}".
	// This allows defining multiple MongoDB clients dynamically.
	gs.Group("${spring.mongodb.instances}", newClient, destroyClient)
}

// newClient creates a new MongoDB client based on the provided configuration.
// After the client is built it is pinged so that misconfiguration or an
// unreachable server fails fast at startup rather than on first use.
func newClient(c Config) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(c.URI)
	if c.ConnectTimeout > 0 {
		opts.SetConnectTimeout(c.ConnectTimeout)
	}
	if c.ServerSelectionTimeout > 0 {
		opts.SetServerSelectionTimeout(c.ServerSelectionTimeout)
	}
	if c.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(c.MaxPoolSize)
	}
	opts.SetMinPoolSize(c.MinPoolSize)
	if c.MaxConnIdleTime > 0 {
		opts.SetMaxConnIdleTime(c.MaxConnIdleTime)
	}
	if c.Username != "" {
		opts.SetAuth(options.Credential{
			Username:      c.Username,
			Password:      c.Password,
			AuthSource:    c.AuthSource,
			AuthMechanism: c.AuthMechanism,
		})
	}
	tlsCfg, err := c.TLS.build()
	if err != nil {
		return nil, err
	}
	if tlsCfg != nil {
		opts.SetTLSConfig(tlsCfg)
	}

	client, err := mongo.Connect(opts)
	if err != nil {
		return nil, fmt.Errorf("mongodb: create client: %w", err)
	}

	// Fail fast: verify the server is reachable before handing out the client.
	ctx, cancel := pingContext(c.ConnectTimeout)
	defer cancel()
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("mongodb: ping %s: %w", c.URI, err)
	}
	return client, nil
}

// HealthCheck reports whether the MongoDB client can reach the server. It is a
// thin readiness probe suitable for wiring into a health endpoint.
func HealthCheck(ctx context.Context, client *mongo.Client) error {
	return client.Ping(ctx, nil)
}

// pingContext derives a context for the startup ping, bounded by the connect
// timeout when set so the probe cannot hang indefinitely.
func pingContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return context.WithTimeout(context.Background(), timeout)
}

// destroyClient disconnects the MongoDB client.
func destroyClient(client *mongo.Client) error {
	return client.Disconnect(context.Background())
}
