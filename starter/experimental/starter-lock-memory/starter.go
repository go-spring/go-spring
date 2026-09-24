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

// Package StarterLockMemory wires [lock.MemoryLocker] into Go-Spring under the
// shared "${spring.lock}" prefix, so an application can be wired exactly like a
// production backend (config keys, bean injection, observe wrapping) while
// running with zero external dependencies — local development, demos, and
// tests of lock-consuming code.
//
// It is deliberately NOT for production multi-replica coordination: the lock's
// scope is one process. Importing it is a statement that this deployment runs
// a single instance; for real coordination blank-import a backed starter
// (starter-lock-redis / -etcd / -consul / -k8s) instead. Like every lock
// backend it registers on "${spring.lock.instances.memory}", so only one lock backend
// starter is imported at a time.
package StarterLockMemory

import (
	"go-spring.org/cloud/lock"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

func init() {
	gs.Module(gs.OnProperty("spring.lock.instances.memory"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.lock.instances.memory}", func(name string, c Config) error {
			r.Provide(newLocker, gs.ValueArg(c)).
				Name("memory." + name).
				Destroy(destroyLocker).
				Caller(1)
			return nil
		})
	})
}

// newLocker builds one MemoryLocker-backed Locker. Wrapped with the shared
// observe adapter by default; observe.enabled=false opts out. Timing knobs
// (TTL, renew, retry) are not configured here — they are per-acquire
// [lock.Option] values, identical across every backend.
func newLocker(c Config) (lock.Locker, error) {
	inner := lock.NewMemoryLocker()
	if !c.ObserveEnabled {
		return inner, nil
	}
	return lock.Observe(inner, "memory"), nil
}

// destroyLocker closes the Locker (passes through the observe wrapper).
func destroyLocker(l lock.Locker) error {
	return l.Close()
}
