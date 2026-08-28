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

// Package StarterRatelimitRedis contributes Redis-backed limiter drivers to a
// Go-Spring application: one resilience.LimiterDriver per entry under
// spring.ratelimit.redis.<name>, each reusing the *redis.Client bean named by
// its `client` field (provided by starter-go-redis under
// spring.go-redis.<client>).
//
// This is a Contributor-archetype starter: it exports no port of its own, it
// adapts the existing Lua token-bucket implementation
// (starter-go-redis/experimental) into the [resilience.LimiterDriver] registry
// so consumers that select limiters by driver name — starter-gateway's
// rateLimit filter, [resilience.GetLimiter], app code — switch from per-replica
// to cross-replica limiting purely by changing that name:
//
//	# before: each replica limits on its own counters
//	spring.gateway.route... = rateLimit(rate=100)
//	# after: one global budget shared by every replica
//	spring.ratelimit.redis.web.client = cache
//	spring.ratelimit.redis.web.driver = redis
//	spring.gateway.route... = rateLimit(rate=100,driver=redis)
//
// The resilience executor driver is untouched: breaker/retry/timeout keep the
// "default" (or sentinel) driver while limiting alone moves to Redis, because
// the limiter registry is independent of the executor registry by design.
package StarterRatelimitRedis

import (
	"context"

	goredis "go-spring.org/starter-go-redis"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	gs.Module(gs.OnProperty("spring.ratelimit.redis"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.ratelimit.redis}", func(name string, c Config) error {
			// Fail fast: an empty client would otherwise surface only at the
			// first Allow call — as an unlimited pass-through.
			if c.Client == "" {
				return errutil.Explain(nil, "ratelimit-redis: instance %q missing required property %q",
					name, "spring.ratelimit.redis."+name+".client")
			}
			driver := c.Driver
			if driver == "" {
				driver = name
			}
			// Fail fast on a driver name claimed by another instance of THIS
			// starter: two instances sharing one name would silently share a
			// single limiter (the registry only panics on cross-module
			// duplicates), hiding the misconfiguration until rate limits bleed
			// across instances in production.
			if err := claimDriverName(driver, name); err != nil {
				return err
			}
			log.Debugf(context.Background(), log.TagAppDef, "creating redis limiter driver instance=%s driver=%s client=%s", name, driver, c.Client)
			// TagArg injects the *redis.Client bean by name — the seam that ties
			// this driver to a specific redis instance. The ctor registers the
			// driver in the resilience limiter registry under `driver`; Export
			// makes the bean root-reachable so the registration always runs.
			r.Provide(func(client *goredis.Client) (*Driver, error) {
				return driverFor(driver, client)
			}, gs.TagArg(c.Client)).
				Name(name).
				Export(gs.As[resilience.LimiterDriver]()).
				Caller(1)
			return nil
		})
	})
}
