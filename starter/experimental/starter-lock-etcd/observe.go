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

package StarterLockEtcd

import (
	"go-spring.org/cloud/experimental/lock"
	lockobserve "go-spring.org/cloud/observe/lock"
)

// wrapLockerBean wraps the instance's locker with the shared observe-lock
// adapter (trace span + duration/in-flight metric + access log;
// lock.system="etcd"). cfg flows the instance's observer.observability
// config into the adapter. When starter-otel is not imported the global
// OTel providers are no-ops, so the wrapper adds negligible overhead.
func wrapLockerBean(c Config, inner lock.Locker) lock.Locker {
	return lockobserve.WrapLocker("etcd", c.Observer.Observability, inner)
}
