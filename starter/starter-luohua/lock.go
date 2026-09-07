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

package luohua

import (
	"go-spring.org/cloud/lock"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/flatten"
)

// lockDefault provides luohua's in-process [lock.Locker] baseline so a luohua
// service that uses lock but wires no backend starter (starter-lock-redis /
// starter-lock-etcd) still gets a working single-process lock for local runs.
// It steps aside via OnMissingBean the moment a real backend bean is present —
// a service pointing at redis/etcd for its lock is never silently downgraded
// to the in-process one.
func lockDefault() *lock.MemoryLocker { return lock.NewMemoryLocker() }

func init() {
	// Armed by spring.luohua.lock=true (OnProperty is a prefix check), gated by
	// the master Enabled. The default rides the same OnMissingBean step-aside as
	// identity/i18n: consumers autowire lock.Locker and get luohua's baseline
	// only while no backend starter provides its own bean.
	gs.Module(gs.OnProperty("spring.luohua.lock"), func(r gs.BeanProvider, p flatten.Storage) error {
		if off, err := disabled(p); err != nil {
			return err
		} else if off {
			return nil // whole baseline off; do not assemble the keyed bean capability
		}
		r.Provide(lockDefault).
			Condition(gs.OnMissingBean[lock.Locker]()).
			Export(gs.As[lock.Locker]()).Caller(1)
		return nil
	})
}
