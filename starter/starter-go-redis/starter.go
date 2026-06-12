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

package StarterGoRedis

import (
	"fmt"

	"github.com/go-spring/spring-core/gs"
	"github.com/redis/go-redis/v9"
)

func init() {

	// Register a single default Redis client.
	// This client will only be created if the property "spring.go-redis.addr" is set.
	// It uses the configuration tagged with "${spring.go-redis}" and is named "__default__".
	gs.Provide(newClient, gs.TagArg("${spring.go-redis}")).
		Condition(gs.OnProperty("spring.go-redis.addr")).
		Destroy(destroyClient).
		Name("__default__")

	// Register multiple Redis clients as a group.
	// Each instance is created according to the configuration in "${spring.go-redis.instances}".
	// This allows defining multiple redis instances dynamically.
	gs.Group("${spring.go-redis.instances}", newClient, destroyClient)
}

// newClient creates a new Redis client based on the provided configuration.
func newClient(c Config) (*redis.Client, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, fmt.Errorf("redis driver not found: %s", c.Driver)
	}
	return d.CreateClient(c)
}

// destroyClient closes the Redis client.
func destroyClient(client *redis.Client) error {
	return client.Close()
}
