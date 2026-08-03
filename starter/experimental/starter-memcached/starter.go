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
	"context"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/log"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
)

var starterTag = log.RegisterInfraTag("memcached", "")

func init() {
	// Register multiple Memcached clients as a group.
	// Each instance is created according to the configuration in "${spring.memcached}".
	// This allows defining multiple memcached instances dynamically.
	//
	// The memcache client keeps a lazy connection pool and exposes no Close
	// method, so the destroy callback only stops any discovery Resolver watch
	// behind the client (added when ServiceName is set).
	gs.Group("${spring.memcached}", newClient, destroyClient)
}

// newClient creates a new Memcached client based on the provided configuration.
func newClient(cp *gs.ContextProvider, c Config) (*memcache.Client, error) {
	ctx := cp.Context
	log.Debugf(ctx, starterTag, "creating memcached client, servers=%v service-name=%s", c.Servers, c.ServiceName)

	if len(c.Servers) == 0 && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "memcached: one of servers or service-name must be set")
	}
	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx, starterTag, "memcached driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "memcached driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(ctx, c)
	if err != nil {
		log.Errorf(ctx, starterTag, "memcached: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create memcached client")
	}
	// Fail fast: probe every configured server with a PING at startup so a
	// misconfigured or unreachable server surfaces during boot rather than on
	// the first request.
	if err := client.Ping(); err != nil {
		log.Errorf(ctx, starterTag, "memcached: startup ping failed: %v", err)
		return nil, errutil.Explain(err, "memcached: startup ping failed")
	}
	log.Infof(ctx, starterTag, "memcached client initialized, servers=%v", c.Servers)
	return client, nil
}

// destroyClient stops any discovery Resolver watch behind the client. The
// memcache client itself keeps a lazy connection pool with no Close method, so
// nothing is closed here — only the background watch (if any) is released.
func destroyClient(client *memcache.Client) error {
	if v, ok := resolvers.LoadAndDelete(client); ok {
		_ = v.(*discovery.Resolver).Stop()
		log.Debugf(context.Background(), starterTag, "memcached client destroyed, discovery resolver stopped")
	}
	return nil
}
