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

// Package StarterLockEtcd wires an etcd-backed distributed lock into
// Go-Spring. Blank-importing the package registers one [lock.Locker] bean per
// entry under "${spring.lock}", each owning its own *clientv3.Client. Every
// acquired lock runs on a fresh concurrency.Session so its lease and Lost()
// channel are independent from other holds.
//
// This starter is the Contributor archetype for the lock abstraction: switching
// the backend to Redis or Consul is a blank-import swap in the application's
// main package. Business code depends only on the lock.Locker interface and
// never on this package.
package StarterLockEtcd

import (
	"runtime"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/lock"
)

func init() {
	// Register one etcd-backed Locker per entry under "${spring.lock}". A
	// hand-rolled gs.Module (rather than gs.Group) is required so each bean
	// can Export the lock.Locker interface — consumers inject that interface
	// and never see the concrete *etcdLocker type, which is what makes the
	// blank-import swap possible.
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.lock"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.lock}"); err != nil {
			return err
		}
		for name, c := range m {
			if len(c.Endpoints) == 0 {
				return errutil.Explain(nil, "lock-etcd: endpoints is required for instance %q", name)
			}
			b := r.Provide(newEtcdLocker, gs.ValueArg(c)).
				Name(name).
				Export(gs.As[lock.Locker]()).
				Destroy(destroyEtcdLocker)
			b.SetFileLine(file, line)
		}
		return nil
	})
}

// destroyEtcdLocker releases the shared etcd client. Locks handed out before
// shutdown own their own sessions and are unaffected; their leases expire
// naturally once the process exits.
func destroyEtcdLocker(l *etcdLocker) error {
	return l.Close()
}
