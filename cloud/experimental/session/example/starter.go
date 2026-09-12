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

package main

import (
	"time"

	"go-spring.org/cloud/experimental/session"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

// memoryConfig configures one in-process session store bound under
// spring.session.memory.instances.<name>. The store lives in this process, so
// there is no address or key space to configure — only how expired sessions are
// reclaimed.
type memoryConfig struct {
	// CleanInterval is how often a background sweep drops expired sessions. 0
	// (the default) leaves expiry lazy: an expired session is reclaimed the next
	// time a request presents its id, so one that expires and is never read
	// again stays in memory.
	CleanInterval time.Duration `value:"${clean-interval:=0}"`
}

// init is a local contributor starter: it contributes one store per entry under
// spring.session.memory.instances.<name>, each exported as a
// session.SessionStore that an application injects by interface and hands to a
// session.Manager.
//
// The wiring below is what a published starter contains — compare
// starter-session-redis, whose store binds under the sibling key
// spring.session.redis.instances.<name> from starter-go-redis's client. Both
// contribute the same interface under the same family, so moving from this
// in-process backend to a distributed one is a blank import swap and a config
// key change; no handler changes.
func init() {
	gs.Module(gs.OnProperty("spring.session.memory.instances"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.session.memory.instances}", func(name string, c memoryConfig) error {
			r.Provide(func(c memoryConfig) (*session.Memory, error) {
				m := session.NewMemory()
				// The sweeper belongs to the store and is released by the bean's
				// destroy hook, so its goroutine lives exactly as long as the
				// bean does.
				m.StartCleanup(c.CleanInterval)
				return m, nil
			}, gs.ValueArg(c)).
				Name(name).
				Export(gs.As[session.SessionStore]()).
				Destroy((*session.Memory).Stop).
				Caller(1)
			return nil
		})
	})
}
