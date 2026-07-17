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
	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Redis clients as a group.
	// Each instance is created according to the configuration in "${spring.go-redis}".
	// This allows defining multiple redis instances dynamically.
	gs.Group("${spring.go-redis}", newClient, destroyClient)
}

// newClient creates a new Redis client based on the provided configuration.
func newClient(c Config) (*redis.Client, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}
	return d.CreateClient(c)
}

// destroyClient closes the Redis client.
func destroyClient(client *redis.Client) error {
	return client.Close()
}
