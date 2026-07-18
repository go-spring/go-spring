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

package StarterLockRedis

import "time"

// Config configures one Redis-backed [go-spring.org/stdlib/lock.Locker] instance
// bound under spring.lock.<name>. It intentionally does not carry Redis
// connection details: locking reuses an existing *redis.Client bean registered
// by starter-go-redis, so switching between share-a-cluster / dedicated-cluster
// topologies is a config-only change on the redis side.
type Config struct {
	// Client is the name of the *redis.Client bean that backs this locker.
	// The bean must be provided by starter-go-redis under spring.go-redis.<Client>.
	// Empty is a fail-fast configuration error — the starter refuses to boot
	// rather than silently falling back to a default instance.
	Client string `value:"${client}"`

	// TTL is the default lease duration handed to the lock package when a
	// caller does not override it via lock.WithTTL. The lock package clamps
	// zero/negative values to its own 30s default.
	TTL time.Duration `value:"${ttl:=30s}"`

	// RenewInterval is the default lease-refresh interval. Zero means "use
	// TTL/3" (the lock package default). A negative value disables auto-renew
	// so the lock expires strictly after TTL regardless of work duration.
	RenewInterval time.Duration `value:"${renew-interval:=0}"`

	// RetryInterval is how often Acquire polls the backend while contended.
	RetryInterval time.Duration `value:"${retry-interval:=100ms}"`

	// KeyPrefix is prepended to every key before it hits Redis. Multiple
	// applications sharing a Redis instance use different prefixes to keep
	// their key spaces disjoint. Empty means keys are used as-is.
	KeyPrefix string `value:"${key-prefix:=}"`
}
