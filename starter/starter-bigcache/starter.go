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

package StarterBigCache

import (
	"github.com/allegro/bigcache/v3"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple BigCache instances as a group.
	// Each instance is created according to the configuration in "${spring.bigcache}".
	// This allows defining multiple in-memory caches dynamically.
	//
	// BigCache spawns a background eviction goroutine, so Close must be called
	// on shutdown to release it — the destroy callback handles that.
	gs.Group("${spring.bigcache}", newClient, destroyClient)
}

// newClient creates a new BigCache instance based on the provided configuration.
func newClient(c Config) (*bigcache.BigCache, error) {
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "bigcache driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create bigcache instance")
	}
	return client, nil
}

// destroyClient closes the BigCache instance, stopping its background cleaner.
func destroyClient(client *bigcache.BigCache) error {
	return client.Close()
}
