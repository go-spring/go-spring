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

// Package StarterRedigo registers the redigo (gomodule/redigo) Redis client as a
// go-spring starter. Each entry under "${spring.redigo}" becomes one pooled
// client bean; the pool is wrapped by Pool, which layers
// observability (trace/metric/access log, see observe.go) and resilience
// (rate-limit / circuit-breaker / retry / timeout via the resilience executor)
// onto every command. Each pool is also exposed as a cache.Cache bean named
// "redigo:<name>".
package StarterRedigo

import (
	"go-spring.org/cloud"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	"go-spring.org/cloud/governance"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Redis clients as a group, one per entry under
	// "${spring.redigo}". A gs.Module (rather than gs.Group) is used so each
	// instance's pool bean can be paired with a health.Indicator registered under
	// the same name — and to attach the file:line of this registration to the
	// bean for diagnostics.
	gs.Module(gs.OnProperty("spring.redigo.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		// Any key an instance does not define falls back to the family-wide
		// "default" bucket: spring.redigo.default.<k> is the value every
		// instance inherits unless it sets its own.
		p = flatten.WithFallback(p, "spring.redigo.instances", "spring.redigo.default")
		return conf.BindEach(p, "${spring.redigo.instances}", func(name string, c Config) error {

			// The adapter is the gs↔NewPool bridge: it converts the
			// *gs.ContextProvider into a plain context.Context, so NewPool itself
			// never touches gs types and is usable standalone. Users wanting
			// custom assembly skip this bean entirely and call NewPool (or
			// NewConn) themselves. The Driver param (index 2) is selected by the
			// entry's ${driver} key: unset → "?" (nullable by-type — injects the
			// single Driver bean when a company provides one, nil otherwise, and
			// createPool falls back to DefaultDriver); set → that bean name, and
			// naming a bean that does not exist fails loud.
			r.Provide(
				createPool,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${spring.redigo.instances."+name+".driver:=${spring.redigo.default.driver:=?}}")),
				gs.IndexArg(3, gs.TagArg("${spring.redigo.instances."+name+".discovery:=${spring.redigo.default.discovery:=none}}")),
				// The governance center is the family's sole injection point: it
				// hands out the resilience/fault/loadbalance authorities.
			).Name(name).Destroy(destroyPool)

			// Contribute a health indicator for this instance unless the user
			// disabled it (health=false), injecting the pool just
			// registered above by name.
			if c.Health {
				r.Provide(func(w *Pool) *health.Indicator {
					return NewClientHealth(name, w)
				}, gs.TagArg(name)).Name("redigo:" + name)
			}
			// Expose this instance as a cache.Cache (the adapter lives in
			// this package's bytecache.go). Named "redigo:<name>" — cache.Cache is
			// a shared type across backend starters, so the prefix keeps the
			// (name, type) key unique. Un-injected, the bean never instantiates.
			r.Provide(func(w *Pool) *cache.Cache {
				return cache.New(NewByteCache(w.Pool))
			}, gs.TagArg(name)).Name("redigo:" + name)
			return nil
		})
	})
}

// createPool is the gs entry: it dispatches to the injected Driver — the
// optional pool-assembly Driver bean, or the bundled DefaultDriver when a
// company provides none — which owns the full assembly (the bundled
// DefaultDriver delegates to [NewPool]) — and authoritatively re-attaches cfg
// (custom out-of-package drivers cannot set unexported fields). The Driver
// returns the Pool complete: identity and governance are both applied while it
// is built, so nothing runs after this returns. See [NewPool] and the Driver
// interface doc for the assembly contract and the two customization shapes.
//
// disc is the backend the entry's ${discovery} label resolves to in the center's
// discovery directory (nil when the key is unset, sentinel "none", or names
// nothing, and the entry dials a static Addr).
//
// center is the governance center bean — the family's sole spokesperson: the
// resilience/fault/loadbalance authorities are read from it rather than
// injected one by one — the ctor bundles them into the [cloud.ClientParams] it
// hands the driver, which applies it in the constructor. Loadbalance is
// included because this starter binds endpoint selection through the shared
// machinery. The bean is nil in a standalone (non-gs) call, and the zero bundle
// degrades to an observed-only, loudly-unmanaged executor rather than failing.
func createPool(ctx *gs.ContextProvider, c Config, d Driver, discoveryLabel string, center *governance.Center) (*Pool, error) {
	// The pool's identity rides on a context derived here: every line below
	// carries it without repeating it. The provider's own context is left
	// alone — that one is the shared application context, not this
	// constructor's.
	cctx := log.WithFields(ctx.Context, log.String("addr", c.Addr))

	log.Debug(cctx, log.TagAppDef, func() []log.Field {
		return []log.Field{
			log.String("service_name", c.ServiceName),
			log.Msg("creating redigo client"),
		}
	})

	if err := errutil.RequireAny("redis",
		errutil.Field{Name: "addr", Value: c.Addr},
		errutil.Field{Name: "service-name", Value: c.ServiceName},
	); err != nil {
		return nil, err
	}

	disc, _ := center.Discovery().Get(discoveryLabel)

	// Fail loud when the entry routes through discovery but the cited label
	// names no backend — the center's discovery directory is the table.
	if c.ServiceName != "" && disc == nil {
		if c.Discovery == "" {
			return nil, errutil.Explain(nil, "redis: instance routes by service-name but sets no discovery backend (set ${spring.redigo.instances.<name>.discovery} to the name of a discovery backend bean)")
		}
		return nil, errutil.Explain(nil, "redis: instance cites discovery backend %q but no such bean exists (register a discovery backend bean under that name)", c.Discovery)
	}
	// No company Driver bean → fall back to the bundled default assembly.
	if d == nil {
		d = DefaultDriver{}
	}

	// d owns pool assembly. It returns the wrapped Pool (NOT the raw
	// *redis.Pool): it may customize the wrapper itself, and downstream
	// consumers uniformly deal in the project's type. Governance rides the
	// Governance bundle into the driver, so the pool is assembled complete — a
	// custom Driver's pool is governed too, without the Driver having to know
	// where the beans came from.
	w, err := d.CreateClient(ctx.Context, c,
		cloud.ClientParams{
			Resilience:  center.Resilience(),
			Fault:       center.Fault(),
			Loadbalance: center.Loadbalance(),
			Discovery:   disc,
		})
	if err != nil {
		log.Error(ctx.Context, log.TagAppDef, err, log.Msg("redigo create client failed"))
		return nil, errutil.Explain(err, "failed to create redis client")
	}
	if w == nil || w.Pool == nil {
		return nil, errutil.Explain(nil, "redis driver returned a nil pool")
	}
	w.cfg = c

	// Fail fast (opt-in, e.g. ping=true): the redigo pool dials lazily, so when
	// Ping is set, probe the target once at startup. A misconfigured address or
	// unreachable server then surfaces during boot rather than on the first
	// request. The probe is [HealthCheck] — the same single health
	// implementation the Actuator indicator uses — which dials one connection
	// directly (see its doc for why it dials instead of using pool.Get). The
	// pool is already assembled at this point, so the ping runs through the
	// command chain (span et al.) — a harmless, even useful, first blip.
	if c.Ping {
		if err := HealthCheck(ctx.Context, w); err != nil {
			log.Error(cctx, log.TagAppDef, err, log.Msg("redigo: startup ping failed"))
			_ = w.Close() // stop resolver watch + close pool
			return nil, errutil.Explain(err, "redis: startup ping failed")
		}
	}

	log.Info(cctx, log.TagAppDef, log.Msg("create redigo client success"))
	return w, nil
}

func destroyPool(p *Pool) error {
	return p.Close()
}
