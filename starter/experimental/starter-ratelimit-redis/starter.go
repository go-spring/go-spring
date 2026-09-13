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
// spring.ratelimit.redis.instances.<name>, each reusing the *redis.Client bean named by
// its `client` field (provided by starter-go-redis under
// spring.go-redis.instances.<client>).
//
// This is a Contributor-archetype starter: it exports no port of its own, it
// adapts the existing Lua token-bucket implementation
// (starter-go-redis/experimental) into a bean named after the driver, exported
// as [resilience.LimiterDriver] — the container is the limiter directory — so
// consumers that select limiters by driver name (starter-gateway's rateLimit
// filter, app code) switch from per-replica to cross-replica limiting purely by
// changing that name:
//
//	# before: each replica limits on its own counters
//	spring.gateway.route... = rateLimit(rate=100)
//	# after: one global budget shared by every replica
//	spring.ratelimit.redis.instances.web.client = cache
//	spring.ratelimit.redis.instances.web.driver = redis
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
	gs.Module(gs.OnProperty("spring.ratelimit.redis.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.ratelimit.redis.instances}", func(name string, c Config) error {
			// Fail fast: an empty client would otherwise surface only at the
			// first Allow call — as an unlimited pass-through.
			if c.Client == "" {
				return errutil.Explain(nil, "ratelimit-redis: instance %q missing required property %q",
					name, "spring.ratelimit.redis.instances."+name+".client")
			}
			driver := c.Driver
			if driver == "" {
				driver = name
			}
			// Fail fast on a driver name claimed by another instance of THIS
			// starter: two instances sharing one name would otherwise surface as
			// a duplicate-bean error, which names the bean but not the two
			// instance keys that collided.
			if err := claimDriverName(driver, name); err != nil {
				return err
			}
			log.Debugf(context.Background(), log.TagAppDef, "creating redis limiter driver instance=%s driver=%s client=%s", name, driver, c.Client)
			// TagArg injects the *redis.Client bean by name — the seam that ties
			// this driver to a specific redis instance. The bean is NAMED after
			// the driver, not the instance, because the container's limiter
			// directory is keyed by the driver name the rateLimit filter's
			// driver= argument addresses; Export makes it visible to name-keyed
			// LimiterDriver injection.
			r.Provide(func(client *goredis.Client) *Driver {
				return driverFor(driver, client)
			}, gs.TagArg(c.Client)).
				Name(driver).
				Export(gs.As[resilience.LimiterDriver]()).
				Caller(1)
			return nil
		})
	})
}
