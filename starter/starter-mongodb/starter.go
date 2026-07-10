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

	"go-spring.org/spring/gs"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

func init() {

	// Register a single default MongoDB client.
	// This client will only be created if the property "spring.mongodb.uri" is set.
	// It uses the configuration tagged with "${spring.mongodb}" and is named "__default__".
	gs.Provide(newClient, gs.TagArg("${spring.mongodb}")).
		Condition(gs.OnProperty("spring.mongodb.uri")).
		Destroy(destroyClient).
		Name("__default__")

	// Register multiple MongoDB clients as a group.
	// Each instance is created according to the configuration in "${spring.mongodb.instances}".
	// This allows defining multiple MongoDB clients dynamically.
	gs.Group("${spring.mongodb.instances}", newClient, destroyClient)
}

// newClient creates a new MongoDB client based on the provided configuration.
func newClient(c Config) (*mongo.Client, error) {
	opts := options.Client().ApplyURI(c.URI)
	if c.ConnectTimeout > 0 {
		opts.SetConnectTimeout(c.ConnectTimeout)
	}
	if c.MaxPoolSize > 0 {
		opts.SetMaxPoolSize(c.MaxPoolSize)
	}
	opts.SetMinPoolSize(c.MinPoolSize)
	return mongo.Connect(opts)
}

// destroyClient disconnects the MongoDB client.
func destroyClient(client *mongo.Client) error {
	return client.Disconnect(context.Background())
}
