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
	"time"

	"go-spring.org/cloud/tlsconf"

	observe "go-spring.org/cloud/observe"
)

// Config binds one etcd-backed distributed-lock instance under
// spring.lock.<name>. Endpoints is required; every other field has a sensible
// default so a minimal configuration only needs the cluster address.
type Config struct {
	// Endpoints lists the etcd cluster nodes to dial. Required; an empty list
	// is rejected at startup so a misconfigured instance never boots silently.
	Endpoints []string `value:"${endpoints}"`

	// Username / Password authenticate against etcd when auth is enabled.
	// Leave empty for anonymous clusters.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// DialTimeout bounds the initial connection attempt. It also bounds the
	// startup readiness probe used to fail fast on unreachable clusters.
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// TTL is the lease duration attached to each acquired lock. When the
	// holder crashes the lease expires after roughly TTL and the lock becomes
	// available. etcd sessions use whole-second TTLs, so values below one
	// second are rounded up to one second.
	TTL time.Duration `value:"${ttl:=30s}"`

	// KeyPrefix is prepended to every lock key so multiple applications can
	// share one cluster without collisions. Trailing slashes are preserved.
	KeyPrefix string `value:"${key-prefix:=/lock/}"`

	// TLS configures optional transport-layer security. Off by default. Uses
	// the shared spring/cloud/tlsconf block so every starter exposes the same
	// tls.* keys.
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// ObserveEnabled toggles the observe-lock instrumentation layer (trace
	// span + duration/in-flight metric + access log) around the Locker. On by
	// default: the primary Locker bean under the instance name is already
	// observed, and callers inject it by that name. Set observe.enabled=false
	// to get the bare locker. When starter-otel is not imported the OTel
	// providers are no-ops, so leaving this on costs almost nothing.
	ObserveEnabled bool `value:"${observe.enabled:=true}"`

	// Observability configures the access log emitted by the shared
	// observe-lock adapter (level off/brief/detailed, per-op skips). Blank
	// level means brief.
	Observability observe.ObserveConfig `value:"${observability:=}"`
}
