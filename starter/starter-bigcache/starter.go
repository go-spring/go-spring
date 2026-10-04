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
	"go-spring.org/cloud/cache"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
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
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.bigcache.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.bigcache.instances", "spring.bigcache.default")
		return conf.BindEach(p, "${spring.bigcache.instances}", func(name string, c Config) error {
			// IndexArg places name (index 1) and c (index 2) explicitly, leaving
			// index 0 (*gs.ContextProvider) to be autowired - the documented
			// pattern for a ctor whose first param is ContextProvider. The
			// Driver param (index 3) is selected by the entry's ${driver} key:
			// unset → "?" (nullable by-type — injects the single Driver bean
			// when a company provides one, nil otherwise, and newClient falls
			// back to DefaultDriver); set → that bean name, and naming a bean
			// that does not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.bigcache.instances."+name+".driver:=${spring.bigcache.default.driver:=?}}")),
				// No governance beans are injected: bigcache is in-process, so there
				// is no policy a rule could apply and no emitter to declare to. See
				// [NewCache].
			).Name(name).Destroy((*Cache).Close).Caller(1)

			// Expose this instance as a cache.Cache (the adapter lives in
			// this package's bytecache.go). Named "bigcache:<name>" — cache.Cache
			// is a shared type across backend starters, so the prefix keeps the
			// (name, type) key unique. Un-injected, the bean never instantiates.
			r.Provide(func(c *Cache) *cache.Cache {
				return cache.New(NewByteCache(c))
			}, gs.TagArg(name)).Name("bigcache:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new BigCache instance from the provided configuration. The
// Driver returns it complete — identity fixed and observability installed, by
// [NewCache] — so nothing here patches it afterwards.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver) (*Cache, error) {
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	// The error is returned as the driver produced it: attribution belongs to
	// the layer that ran the failing step (see [Driver]), and the container
	// adds the bean name and this registration's file:line on top.
	client, err := d.CreateClient(ctx.Context, name, c)
	if err != nil {
		return nil, err
	}
	// The Driver returned the cache complete — identity and observation both
	// applied while it was built. There is no Init hook and nothing runs after
	// this: the bean is finished when the ctor returns.
	return client, nil
}
