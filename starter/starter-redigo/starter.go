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

package StarterRedigo

import (
	"github.com/gomodule/redigo/redis"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {

	// Register a single default Redis client.
	// This client will only be created if the property "spring.redigo.addr" is set.
	// It uses the configuration tagged with "${spring.redigo}".
	gs.Provide(newClient, gs.TagArg("${spring.redigo}")).
		Condition(gs.OnProperty("spring.redigo.addr")).
		Destroy(destroyClient)

	// Register multiple Redis clients as a group.
	// Each instance is created according to the configuration in "${spring.redigo.instances}".
	// This allows defining multiple redis instances dynamically.
	gs.Group("${spring.redigo.instances}", newClient, destroyClient)
}

// newClient creates a new Redis client based on the provided configuration.
func newClient(c Config) (*redis.Pool, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}
	pool, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create redis client")
	}
	return pool, nil
}

// destroyClient closes the Redis client.
func destroyClient(pool *redis.Pool) error {
	return pool.Close()
}
