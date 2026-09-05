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

package StarterLockConsul

import (
	"go-spring.org/cloud/lock"
	"go-spring.org/spring/gs"
)

// newLocker is the bean constructor behind the instance name: it builds the
// consul locker and — by default — wraps it with the shared observe-lock
// adapter (trace span + duration/in-flight metric + access log;
// lock.system="consul"), so the primary bean is transparently observed.
// Instances that opt out via observe.enabled=false get the bare locker. When
// starter-otel is not imported the global OTel providers are no-ops, so the
// wrapper adds negligible overhead.
func newLocker(ctx *gs.ContextProvider, c Config) (lock.Locker, error) {
	inner, err := newConsulLocker(ctx, c)
	if err != nil {
		return nil, err
	}
	return wrapIfObserved(c, inner), nil
}

// wrapIfObserved is the transparent-default decision: bound configs default
// observe.enabled=true, so the primary bean is the wrapped locker; an explicit
// opt-out returns the bare inner locker unchanged.
func wrapIfObserved(c Config, inner lock.Locker) lock.Locker {
	if !c.ObserveEnabled {
		return inner
	}
	return lock.WrapLocker("consul", inner)
}
