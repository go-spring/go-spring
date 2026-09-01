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
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	StarterCache "go-spring.org/starter-cache"
	"go-spring.org/starter-memcached/bytecache"
	health2 "go-spring.org/starter-memcached/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Memcached clients as a group, one per entry under
	// "${spring.memcached}". A gs.Module (rather than gs.Group) is used so each
	// instance's client bean can be paired with a health.Indicator registered
	// under the same name — and to attach the file:line of this registration to
	// the bean for diagnostics.
	//
	// The memcache client keeps a lazy connection pool and exposes no Close
	// method, so the destroy callback only stops any discovery Resolver watch
	// behind the client (added when ServiceName is set).
	gs.Module(gs.OnProperty("spring.memcached"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.memcached}", func(name string, c Config) error {
			// IndexArg(1, ...) binds c to index 1, leaving index 0 (*gs.ContextProvider)
			// to be autowired — the documented pattern for a ctor whose first param
			// is ContextProvider (a bare ValueArg would bind to index 0 instead).
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
			).Name(name).Init((*Client).Init).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(c *Client) health.Indicator { return health2.NewClientHealth(name, c.Client) }, gs.TagArg(name)).Name("memcache:" + name).Export(gs.As[health.Indicator]()).Caller(1)
			return nil
		})
	})
}

// init registers the "memcached" cache driver so a *memcache.Client registered
// under ${spring.memcached} can be exposed as a cache.Cache via:
//
//	spring.cache.<name>.driver = memcached:<memcached-instance-name>
//
// The beanID selects which memcache client bean to wrap; the implementation
// lives in starter-memcached/bytecache.
func init() {
	StarterCache.RegisterDriver("memcached", func(beanID string) gs.ModuleFunc {
		return func(r gs.BeanProvider, p flatten.Storage) error {
			r.Provide(func(c *Client) *cache.Cache {
				return cache.New(bytecache.NewByteCache(c.Client))
			}, gs.TagArg(beanID)).Name(beanID)
			return nil
		}
	})
}

// newClient creates a new Memcached client based on the provided configuration,
// wrapped so every operation flows through the observe kit (trace+metric+log).
func newClient(ctx *gs.ContextProvider, name string, c Config) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating memcached client, servers=%v service-name=%s", c.Servers, c.ServiceName)

	if len(c.Servers) == 0 && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "memcached: one of servers or service-name must be set")
	}
	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx.Context, log.TagAppDef, "memcached driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "memcached driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "memcached: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create memcached client")
	}
	// Fail fast: probe every configured server with a PING at startup so a
	// misconfigured or unreachable server surfaces during boot rather than on
	// the first request.
	if err := client.Ping(); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "memcached: startup ping failed: %v", err)
		return nil, errutil.Explain(err, "memcached: startup ping failed")
	}
	log.Infof(ctx.Context, log.TagAppDef, "memcached client initialized, servers=%v", c.Servers)
	// Return the wrapper; gs field-injects Resilience (gs.Dync, hot-reloadable)
	// + Observability after this returns, then calls Init (InitMethod)
	// to build the observer + executor. Close (Destroy) stops any discovery
	// Resolver watch and closes the executor.
	return &Client{Client: client, name: name}, nil
}
