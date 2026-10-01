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
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple BigCache instances as a group, one per entry under
	// "${spring.bigcache}". A gs.Module (rather than gs.Group) is used so each
	// instance's name is available to label its OTel metrics - and to attach
	// the file:line of this registration to the bean for diagnostics.
	//
	// BigCache spawns a background eviction goroutine, so Close must be called
	// on shutdown to release it - the destroy callback handles that.
	gs.Module(gs.OnProperty("spring.bigcache.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.bigcache.instances}", func(name string, c Config) error {
			// IndexArg places name (index 1) and c (index 2) explicitly, leaving
			// index 0 (*gs.ContextProvider) to be autowired - the documented
			// pattern for a ctor whose first param is ContextProvider. The
			// Driver param (index 3) is selected by the entry's ${driver} key:
			// unset → "?" (nullable by-type — injects the single Driver bean
			// when a company provides one, nil otherwise, and newClient falls
			// back to DefaultDriver); set → that bean name, and naming a bean
			// that does not exist fails loud. The trailing governance beans
			// (*resilience.Manager / *fault.Injector) are injected nullable ("?"),
			// since starter-governance may legitimately be absent from the
			// container.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.bigcache.instances."+name+".driver:=${spring.bigcache.default.driver:=?}}")),
				// The governance beans are REQUIRED: this starter blank-imports
				// starter-governance, so "governance off" is spring.governance.enabled=false,
				// never an absent bean.
				gs.IndexArg(4, gs.TagArg("")),
				gs.IndexArg(5, gs.TagArg("")),
			).Name(name).Init((*Cache).Init).Destroy((*Cache).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(c *Cache) *health.Indicator { return NewBigCacheHealth(name, c.BigCache) }, gs.TagArg(name)).Name("bigcache:" + name).Caller(1)
			// Expose this instance as a cache.Cache (the adapter lives in
			// this package's bytecache.go). Named "bigcache:<name>" — cache.Cache
			// is a shared type across backend starters, so the prefix keeps the
			// (name, type) key unique. Un-injected, the bean never instantiates.
			r.Provide(func(c *Cache) *cache.Cache {
				return cache.New(NewByteCache(c.BigCache))
			}, gs.TagArg(name)).Name("bigcache:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new BigCache instance based on the provided configuration,
// wrapped so Get/Set/Delete flow through the module-local observe layer. The
// cache-statistics gauges are registered by Init, which is where the
// registration is paired with the Destroy that takes it away again.
//
// mgr and inj are the governance beans the container injects (both nil in a
// standalone, non-gs call); they are retained on the Cache for Init
// (InitMethod) to arm the resilience executor with.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, mgr *resilience.Manager, inj *fault.Injector) (*Cache, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating bigcache instance, name=%s shards=%d max-size=%d", name, c.Shards, c.MaxEntrySize)

	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "bigcache: create instance failed: %v", err)
		return nil, errutil.Explain(err, "failed to create bigcache instance")
	}
	log.Infof(ctx.Context, log.TagAppDef, "bigcache instance initialized, name=%s shards=%d", name, c.Shards)
	// Return the wrapper; gs calls Init (InitMethod) after this returns to
	// build the observer + executor from the injected governance beans.
	return &Cache{BigCache: client, name: name, mgr: mgr, inj: inj}, nil
}
