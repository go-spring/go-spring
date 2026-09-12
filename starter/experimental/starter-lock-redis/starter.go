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

// Package StarterLockRedis contributes Redis-backed [lock.Locker] beans to a
// Go-Spring application. Blank-importing this package registers one Locker per
// entry under spring.lock.instances.<name>; each Locker reuses the *redis.Client bean
// named by its `client` field (provided by starter-go-redis under
// spring.go-redis.instances.<client>).
//
// This is a Contributor-archetype starter (see starter/DESIGN.md §2.3): it
// exports no port and holds no connection of its own, it merely contributes
// a bean type behind the framework-neutral lock.Locker seam. Switching the
// lock backend from Redis to etcd/consul is therefore a blank-import swap —
// no business code changes.
//
// The Locker bean is registered under its config name and exported as
// lock.Locker, so callers inject it by interface. The bean is wrapped with the
// observe-lock adapter by default (trace span + metric + access log); set
// spring.lock.instances.<name>.observe.enabled=false for the bare locker:
//
//	type Service struct {
//	    Lock lock.Locker `autowire:"jobs"`
//	}
//
// Leader election is available for free via lock.NewElection over the same
// Locker (see [go-spring.org/spring/lock.Election]).
package StarterLockRedis

import (
	"go-spring.org/cloud/lock"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

func init() {
	gs.Module(gs.OnProperty("spring.lock.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.lock.instances}", func(name string, c Config) error {
			// Fail fast: silently defaulting to some arbitrary *redis.Client
			// would hide a misconfiguration that only surfaces on first
			// Acquire, potentially in production. Refusing to boot is safer.
			if c.Client == "" {
				return errutil.Explain(nil, "lock-redis: instance %q missing required property %q",
					name, "spring.lock.instances."+name+".client")
			}
			// TagArg injects the *redis.Client bean by name — this is the
			// seam that ties the Locker to a specific redis instance. The bean
			// is wrapped with the observe-lock adapter by default (see
			// newLocker); observe.enabled=false opts out. There is no separate
			// "<name>-observed" bean — the primary name is already observed.
			r.Provide(newLocker, gs.ValueArg(c), gs.TagArg(c.Client)).
				Name(name).
				Export(gs.As[lock.Locker]()).
				Destroy(destroyLocker).
				Caller(1)
			return nil
		})
	})
}

// destroyLocker stops background renew goroutines. It never touches the
// injected *redis.Client — starter-go-redis owns that lifecycle. The bean is a
// lock.Locker (possibly observe-wrapped); Close passes through the wrapper.
func destroyLocker(l lock.Locker) error {
	return l.Close()
}
