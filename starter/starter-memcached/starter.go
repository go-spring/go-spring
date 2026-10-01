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
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
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
	gs.Module(gs.OnProperty("spring.memcached.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.memcached.instances}", func(name string, c Config) error {
			// IndexArg(1, ...) binds c to index 1, leaving index 0 (*gs.ContextProvider)
			// to be autowired — the documented pattern for a ctor whose first param
			// is ContextProvider (a bare ValueArg would bind to index 0 instead).
			// The Driver param (index 3) is selected by the entry's ${driver} key:
			// unset → "?" (nullable by-type — injects the single Driver bean when
			// a company provides one, nil otherwise, and newClient falls back to
			// DefaultDriver); set → that bean name, and naming a bean that does
			// not exist fails loud.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
				gs.IndexArg(3, gs.TagArg("${spring.memcached.instances."+name+".driver:=${spring.memcached.default.driver:=?}}")),
				gs.IndexArg(4, gs.TagArg("${spring.memcached.instances."+name+".discovery:=${spring.memcached.default.discovery:=none}}?")),
				// The governance beans are REQUIRED: each is registered by the package that
				// owns it (cloud/resilience, cloud/loadbalance, cloud/fault), which this
				// starter imports — "governance off" is spring.governance.enabled=false, never
				// an absent bean.
				gs.IndexArg(5, gs.TagArg("")),
				gs.IndexArg(6, gs.TagArg("")),
			).Name(name).Destroy((*Client).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(c *Client) *health.Indicator { return NewClientHealth(name, c) }, gs.TagArg(name)).Name("memcache:" + name).Caller(1)
			// Expose this instance as a cache.Cache (the adapter lives in
			// this package). Named "memcached:<name>" — cache.Cache
			// is a shared type across backend starters, so the prefix keeps the
			// (name, type) key unique. Un-injected, the bean never instantiates.
			r.Provide(func(c *Client) *cache.Cache {
				return cache.New(NewByteCache(c))
			}, gs.TagArg(name)).Name("memcached:" + name).Caller(1)
			return nil
		})
	})
}

// newClient creates a new Memcached client based on the provided configuration,
// wrapped so every operation flows through the module-local observe layer (trace+metric+log).
//
// disc is the discovery backend bean cited by the entry's ${discovery} label
// (nil when the key is unset or the entry uses a static server list).
//
// mgr and inj are the governance beans the container injects (both nil in a
// standalone, non-gs call); the ctor bundles them — together with disc — into
// the [cloud.ClientParams] it hands the driver, which passes it to [NewClient]:
// the client is assembled complete in one step, with the zero bundle degrading
// to an observed-only, loudly-unmanaged executor.
func newClient(ctx *gs.ContextProvider, name string, c Config, d Driver, disc discovery.Discovery,
	mgr *resilience.Manager, inj *fault.Injector) (*Client, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating memcached client, servers=%v service-name=%s", c.Servers, c.ServiceName)

	if len(c.Servers) == 0 && c.ServiceName == "" {
		return nil, errutil.Explain(nil, "memcached: one of servers or service-name must be set")
	}
	// Fail loud when the entry routes through discovery but no backend resolved:
	// either the ${discovery} label is unset, or it names no registered bean
	// (the container is the discovery directory).
	if c.ServiceName != "" && disc == nil {
		if c.Discovery == "" {
			return nil, errutil.Explain(nil, "memcached: instance %q routes by service-name but sets no discovery backend (set ${spring.memcached.instances.%s.discovery} to the name of a discovery backend bean)", name, name)
		}
		return nil, errutil.Explain(nil, "memcached: instance %q cites discovery backend %q but no such bean exists (register a discovery backend bean under that name)", name, c.Discovery)
	}
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}
	client, err := d.CreateClient(ctx.Context, name, c,
		cloud.ClientParams{Resilience: mgr, Fault: inj, Discovery: disc})
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "memcached: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create memcached client")
	}
	// The Driver returned the client complete — identity and governance both
	// applied while it was built. There is no Init hook and nothing runs after
	// this: the bean is finished when the ctor returns.
	// Fail fast: probe every configured server with a PING at startup so a
	// misconfigured or unreachable server surfaces during boot rather than on
	// the first request. The probe is [HealthCheck] — the same single health
	// implementation the Actuator indicator uses — which goes straight to the
	// raw client on purpose: it is a connectivity check, not business traffic,
	// so it must not open a span or spend limiter/breaker budget. A failure
	// abandons the client, so release what was just applied.
	if err := HealthCheck(ctx.Context, client); err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "memcached: startup ping failed: %v", err)
		_ = client.Destroy()
		return nil, errutil.Explain(err, "memcached: startup ping failed")
	}
	log.Infof(ctx.Context, log.TagAppDef, "memcached client initialized, servers=%v", c.Servers)
	return client, nil
}
