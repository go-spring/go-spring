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

package StarterSessionRedis

// Config configures one Redis-backed [go-spring.org/spring/session.SessionStore]
// instance bound under spring.session.redis.<name>. Like the lock starter it
// carries no Redis connection details: session storage reuses an existing
// *redis.Client bean registered by starter-go-redis, so switching between
// share-a-cluster / dedicated-cluster topologies is a config-only change on the
// redis side.
type Config struct {
	// Client is the name of the *redis.Client bean that backs this store. The
	// bean must be provided by starter-go-redis under spring.go-redis.<Client>.
	// Empty is a fail-fast configuration error — the starter refuses to boot
	// rather than silently falling back to a default instance.
	Client string `value:"${client}"`

	// KeyPrefix is prepended to every session id before it hits Redis. Multiple
	// applications sharing a Redis instance use different prefixes to keep their
	// session key spaces disjoint. Default "session:".
	KeyPrefix string `value:"${key-prefix:=session:}"`
}
