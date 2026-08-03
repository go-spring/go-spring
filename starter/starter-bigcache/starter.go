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
	"context"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/log"
	"go-spring.org/spring/data/cache"
	"go-spring.org/spring/gs"
	cache2 "go-spring.org/starter-bigcache/cache"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

var starterTag = log.RegisterInfraTag("bigcache", "")

func init() {
	// Register multiple BigCache instances as a group.
	// Each instance is created according to the configuration in "${spring.bigcache}".
	// This allows defining multiple in-memory caches dynamically.
	//
	// BigCache spawns a background eviction goroutine, so Close must be called
	// on shutdown to release it — the destroy callback handles that.
	gs.Group("${spring.bigcache}", newClient, destroyClient)
}

// init registers the "bigcache" cache driver so a *bigcache.BigCache registered
// under ${spring.bigcache} can be exposed as a cache.Cache via:
//
//	spring.cache.<name>.driver = bigcache:<bigcache-instance-name>
//
// The beanID selects which BigCache bean to wrap; the implementation lives in
// starter-bigcache/cache.
func init() {
	cache.RegisterDriver("bigcache", func(beanID string) gs.ModuleFunc {
		return func(r gs.BeanProvider, p flatten.Storage) error {
			r.Provide(cache2.NewCache, gs.TagArg(beanID)).Name(beanID)
			return nil
		}
	})
}

// newClient creates a new BigCache instance based on the provided configuration.
func newClient(cp *gs.ContextProvider, c Config) (*bigcache.BigCache, error) {
	ctx := cp.Context
	log.Debugf(ctx, starterTag, "creating bigcache instance, shards=%d max-size=%d", c.Shards, c.MaxEntrySize)

	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx, starterTag, "bigcache driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "bigcache driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(ctx, c)
	if err != nil {
		log.Errorf(ctx, starterTag, "bigcache: create instance failed: %v", err)
		return nil, errutil.Explain(err, "failed to create bigcache instance")
	}
	log.Infof(ctx, starterTag, "bigcache instance initialized, shards=%d", c.Shards)
	return client, nil
}

// destroyClient closes the BigCache instance, stopping its background cleaner.
func destroyClient(client *bigcache.BigCache) error {
	log.Debugf(context.Background(), starterTag, "bigcache instance destroyed")
	return client.Close()
}
