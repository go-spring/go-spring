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

package StarterMemcached

import (
	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

func init() {
	// Register multiple Memcached clients as a group.
	// Each instance is created according to the configuration in "${spring.memcached}".
	// This allows defining multiple memcached instances dynamically.
	//
	// The memcache client keeps a lazy connection pool and exposes no Close method,
	// so no destroy callback is needed.
	gs.Group("${spring.memcached}", newClient, nil)
}

// newClient creates a new Memcached client based on the provided configuration.
func newClient(c Config) (*memcache.Client, error) {
	if len(c.Servers) == 0 && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "memcached: one of servers or service-name must be set")
	}
	d, ok := driverRegistry[c.Driver]
	if !ok {
		return nil, errutil.Explain(nil, "memcached driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(c)
	if err != nil {
		return nil, errutil.Explain(err, "failed to create memcached client")
	}
	// Fail fast: probe every configured server with a PING at startup so a
	// misconfigured or unreachable server surfaces during boot rather than on
	// the first request.
	if err := client.Ping(); err != nil {
		return nil, errutil.Explain(err, "memcached: startup ping failed")
	}
	return client, nil
}
