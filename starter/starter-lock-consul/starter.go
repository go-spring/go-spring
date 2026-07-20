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

// Package StarterLockConsul integrates Consul as a distributed-lock backend
// for the [lock.Locker] abstraction in stdlib/lock.
//
// Blank-importing this starter registers one [lock.Locker] bean per entry
// under "spring.lock.<name>", each backed by its own *api.Client. Because the
// bean type is the neutral [lock.Locker] interface, switching between Redis,
// etcd and Consul backends is a blank-import swap with no application-code
// change — this is the Contributor archetype from starter/DESIGN.md §2.3.
// Leader election on top of any Locker is available via [lock.NewElection].
package StarterLockConsul

import (
	"runtime"

	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/spring/cloud/lock"
)

func init() {
	// Register one Locker bean per entry under "${spring.lock}". We bind the
	// map ourselves rather than use gs.Group so a missing Address (§3 fail-fast
	// rule in DESIGN.md) surfaces during bean provisioning with a message that
	// names the offending instance.
	_, file, line, _ := runtime.Caller(0)
	gs.Module(gs.OnProperty("spring.lock"), func(r gs.BeanProvider, p flatten.Storage) error {
		var m map[string]Config
		if err := conf.Bind(p, &m, "${spring.lock}"); err != nil {
			return err
		}
		for name, c := range m {
			if c.Address == "" {
				return errutil.Explain(nil, "lock-consul: spring.lock.%s.address is required", name)
			}
			b := r.Provide(newConsulLocker, gs.ValueArg(c)).
				Name(name).
				Export(gs.As[lock.Locker]()).
				Destroy(destroyLocker)
			b.SetFileLine(file, line)
		}
		return nil
	})
}

// destroyLocker is the per-bean destructor. The api.Client itself has no
// Close, so Close on the locker only guards the sync.Once contract; the actual
// cleanup work lives in each Lock handle's Unlock.
func destroyLocker(l *consulLocker) error {
	return l.Close()
}
