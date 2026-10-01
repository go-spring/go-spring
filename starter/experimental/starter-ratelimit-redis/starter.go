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

// Package StarterRatelimitRedis contributes a [resilience.Counters] backed by
// Redis — the store the rate-limit stage of a [resilience.ClientExecutor] spends — so
// one budget per scope covers every replica instead of each replica counting its
// own.
//
// Nothing to step aside for: without this starter the process contributes NO
// counter store, and each executor counts in a budget of its own (one budget per
// service label, since the manager builds one executor per label).
// Contributing this one is the whole switch — the bundled driver bean
// injects whatever store the container holds, so every executor it builds, the
// bundled "default" as much as any other backend, spends the shared budget.
// Only the WIDTH of a budget changes, never what it covers: the scope is still
// the target's label.
//
// The store reuses a *goredis.Client bean published by starter-go-redis under
// spring.go-redis.instances.<name>, named by the starter's one property:
//
//	import _ "go-spring.org/starter-ratelimit-redis"
//	# spring.ratelimit.redis.client=cache
//
// The rate-limit knobs themselves (rate-limit/burst/algorithm/window/
// rate-limit-max-wait) are resilience.ClientPolicy fields on the governance rule
// document, not starter configuration: this starter only decides WHERE the
// counters live.
//
// The executor/breaker/retry driver is untouched — rejecting or queueing
// over-limit calls stays the executor's job, and no route argument or per-route
// switch is involved.
package StarterRatelimitRedis

import (
	"context"

	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	goredis "go-spring.org/starter-go-redis"
	experimental "go-spring.org/starter-go-redis/experimental"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// configured turns the starter on when its block is present. An imported but
// unconfigured starter contributes nothing, so each executor keeps counting in
// its own budget — limiting stays per-replica until the application asks for a
// shared one.
var configured = gs.OnProperty("spring.ratelimit.redis")

func init() {
	gs.Module(configured, setup)
}

// setup binds ${spring.ratelimit.redis} and contributes the Redis-backed store.
// It is a gs.Module, not a plain bean, because the block's presence is the
// switch: the module's condition gates the contribution, and setup binds the
// block before providing the bean.
func setup(r gs.BeanProvider, p flatten.Storage) error {
	var c Config
	if err := conf.Bind(p, &c, "${spring.ratelimit.redis}"); err != nil {
		return err
	}
	// Fail fast: an empty client would otherwise surface only at the first
	// protected call — as a limiting decision made by the wrong store.
	if c.Client == "" {
		return errutil.Explain(nil, "ratelimit-redis: missing required property %q", "spring.ratelimit.redis.client")
	}
	log.Debugf(context.Background(), log.TagAppDef, "contributing redis rate-limit counters client=%s", c.Client)
	// TagArg injects the *goredis.Client bean by name — the seam that ties the
	// counters to one Redis instance. The ctor returns the interface type, so gs
	// indexes the bean under resilience.Counters: no Export is needed, and that
	// type is what the bundled driver injects.
	r.Provide(func(client *goredis.Client) (resilience.Counters, error) {
		return experimental.NewCounters(client)
	}, gs.TagArg(c.Client)).Caller(1)
	return nil
}
