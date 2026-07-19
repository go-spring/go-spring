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

package StarterBatchRedis

import "time"

// Config configures one Redis-backed [go-spring.org/stdlib/batch.JobRepository]
// instance bound under spring.batch-repository.<name>. It intentionally does
// not carry Redis connection details: the repository reuses an existing
// *redis.Client bean registered by starter-go-redis, so switching between
// shared / dedicated Redis topologies is a config-only change on the redis
// side.
//
// The prefix is deliberately spring.batch-repository.<name>, not
// spring.batch.<name>: the batch runner owns the spring.batch namespace for
// job / step / chunk configuration, and repository backends are a distinct
// capability that the runner references by name via spring.batch.repository.
type Config struct {
	// Client is the name of the *redis.Client bean that backs this repository.
	// The bean must be provided by starter-go-redis under
	// spring.go-redis.<Client>. Empty is a fail-fast configuration error —
	// the starter refuses to boot rather than silently falling back to some
	// default instance.
	Client string `value:"${client}"`

	// KeyPrefix is prepended to every key before it hits Redis. Multiple
	// applications sharing a Redis instance use different prefixes to keep
	// their key spaces disjoint. Empty means keys are used as-is.
	KeyPrefix string `value:"${key-prefix:=}"`

	// TTL, when > 0, is applied via EXPIRE to every JobExecution / step-hash
	// key touched by the repository. Zero (the default) leaves execution
	// records in Redis forever, which is what you want when restart windows
	// are open-ended; set a value to garbage-collect finished-and-forgotten
	// runs and cap Redis growth.
	TTL time.Duration `value:"${ttl:=0}"`
}
