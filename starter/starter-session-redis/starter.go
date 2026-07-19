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

// Package StarterSessionRedis contributes Redis-backed
// [session.SessionStore] beans to a Go-Spring application. Blank-importing this
// package registers one Store per entry under spring.session.redis.<name>; each
// Store reuses the *redis.Client bean named by its `client` field (provided by
// starter-go-redis under spring.go-redis.<client>).
//
// This is a Contributor-archetype starter (see starter/DESIGN.md §2.3): it
// exports no port and holds no connection of its own, it merely contributes a
// bean type behind the framework-neutral session.SessionStore seam. Switching
// the session backend from Redis to any other distributed store is therefore a
// blank-import swap — no business code changes.
//
// The Store bean is registered under its config name and exported as
// session.SessionStore, so an application injects it by interface and hands it
// to a session.Manager to serve shared, cross-replica HTTP sessions:
//
//	gs.Provide(func(store session.SessionStore) *gs.HttpServeMux {
//	    mgr := session.NewManager(store, session.Options{})
//	    return &gs.HttpServeMux{Handler: mgr.Middleware(mux)}
//	}, gs.TagArg("web"))
package StarterSessionRedis

import (
	"runtime"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/session"
)

func init() {
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.session.redis"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.session.redis}"); err != nil {
			return err
		}
		for name, c := range m {
			// Fail fast: silently defaulting to some arbitrary *redis.Client
			// would hide a misconfiguration that only surfaces on the first
			// session read/write, potentially in production.
			if c.Client == "" {
				return errutil.Explain(nil, "session-redis: instance %q missing required property %q",
					name, "spring.session.redis."+name+".client")
			}
			// TagArg injects the *redis.Client bean by name — this is the seam
			// that ties the store to a specific redis instance.
			b := r.Provide(newStore, gs.ValueArg(c), gs.TagArg(c.Client)).
				Name(name).
				Export(gs.As[session.SessionStore]())
			b.SetFileLine(file, line)
		}
		return nil
	})
}
