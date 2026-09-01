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
// observability (trace/metric/log via the shared observe kit) and resilience
// (rate-limit / circuit-breaker / retry / timeout via the resilience executor)
// onto every command. A "redigo" cache driver is also registered so a pool can
// be exposed as a cache.Cache.
package StarterRedigo

import (
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	observe "go-spring.org/cloud/observe"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	StarterCache "go-spring.org/starter-cache"
	"go-spring.org/starter-redigo/bytecache"
	poolhealth "go-spring.org/starter-redigo/health"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	// Register multiple Redis clients as a group, one per entry under
	// "${spring.redigo}". A gs.Module (rather than gs.Group) is used so each
	// instance's pool bean can be paired with a health.Indicator registered under
	// the same name — and to attach the file:line of this registration to the
	// bean for diagnostics.
	gs.Module(gs.OnProperty("spring.redigo"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.redigo}", func(name string, c Config) error {

			// The adapter is the gs↔NewPool bridge: it converts the
			// *gs.ContextProvider into a plain context.Context, so NewPool itself
			// never touches gs types and is usable standalone. Users wanting
			// custom assembly skip this bean entirely and call NewPool (or
			// NewConn) themselves.
			//
			// IndexArg(2) binds the GLOBAL observability policy (the
			// observability.* keys shared across all client starters) at provide
			// time — the same TagArg-binds-a-struct pattern starter-gin uses for
			// its Config. It is deliberately NOT part of per-instance Config
			// (whose ObserveEnabled is only the kill switch).
			r.Provide(
				createPool,
				gs.IndexArg(1, gs.ValueArg(c)),
				gs.IndexArg(2, gs.TagArg("${observability:=}")),
			).Name(name).Destroy(destroyPool)

			// Contribute a health indicator for this instance unless the user
			// disabled it (health.enabled=false), injecting the pool just
			// registered above by name.
			if c.HealthEnabled {
				r.Provide(func(w *Pool) health.Indicator {
					return poolhealth.NewPoolHealth(name, w.Pool)
				}, gs.TagArg(name)).Name("redigo:" + name).Export(gs.As[health.Indicator]())
			}
			return nil
		})
	})

	// init registers the "redigo" cache driver so a *redis.Pool registered under
	// ${spring.redigo} can be exposed as a cache.Cache via:
	//
	//	spring.cache.<name>.driver = redigo:<redigo-instance-name>
	//
	// The beanID selects which pool bean to wrap; the implementation lives in
	// starter-redigo/bytecache.
	StarterCache.RegisterDriver("redigo", func(beanID string) gs.ModuleFunc {
		return func(r gs.BeanProvider, p flatten.Storage) error {
			r.Provide(func(w *Pool) *cache.Cache {
				return cache.New(bytecache.NewByteCache(w.Pool))
			}, gs.TagArg(beanID)).Name(beanID)
			return nil
		}
	})
}

// createPool is the gs entry: it dispatches to the configured Driver —
// which owns the full pool assembly (the bundled DefaultDriver delegates to
// [NewPool]) — and authoritatively re-attaches cfg (custom out-of-package
// drivers cannot set unexported fields). The Driver's returned Pool is fully
// armed; see [NewPool] and the Driver interface doc for the assembly contract
// and the two customization shapes.
//
// obs is the GLOBAL observability policy shared across all client starters
// (the observability.* keys); it is deliberately not part of per-instance
// Config, whose ObserveEnabled is only the per-instance kill switch. The gs
// entry binds it and passes it in; a programmatic caller passes
// observe.ObserveConfig{} (or whatever policy it wants).
func createPool(ctx *gs.ContextProvider, c Config, obs observe.ObserveConfig) (*Pool, error) {

	log.Debugf(ctx.Context, log.TagAppDef, "creating redigo client, addr=%s service-name=%s", c.Addr, c.ServiceName)

	if err := errutil.RequireAny("redis",
		errutil.Field{Name: "addr", Value: c.Addr},
		errutil.Field{Name: "service-name", Value: c.ServiceName},
	); err != nil {
		return nil, err
	}

	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx.Context, log.TagAppDef, "redigo driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "redis driver not found: %s", c.Driver)
	}

	// The driver returns the wrapped Pool (NOT the raw *redis.Pool): it may
	// customize the wrapper itself, and downstream consumers uniformly deal in
	// the project's type.
	w, err := d.CreateClient(ctx.Context, c, obs)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "redigo: create client failed: %v", err)
		return nil, errutil.Explain(err, "failed to create redis client")
	}
	if w == nil || w.Pool == nil {
		return nil, errutil.Explain(nil, "redis driver %s returned a nil pool", c.Driver)
	}
	w.cfg = c

	// Fail fast (opt-in): the redigo pool dials lazily, so when StartupPing is
	// set, dial one connection directly and PING it at startup. A misconfigured
	// address or unreachable server then surfaces during boot rather than on
	// the first request. The pool is already assembled at this point, so the
	// ping runs through the command chain (span et al.) — a harmless, even
	// useful, first blip. See startupPing for why it dials directly instead of
	// via pool.Get.
	if c.StartupPing {
		if err := startupPing(ctx.Context, w.Pool); err != nil {
			_ = w.Close() // stop resolver watch + close pool
			return nil, err
		}
	}

	log.Infof(ctx.Context, log.TagAppDef, "redigo client initialized, addr=%s", c.Addr)
	return w, nil
}

func destroyPool(p *Pool) error {
	return p.Close()
}
