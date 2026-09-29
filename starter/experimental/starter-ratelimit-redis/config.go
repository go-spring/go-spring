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

package StarterRatelimitRedis

// Config binds ${spring.ratelimit.redis}, the starter's single block. There is
// one counter store per process, so the only thing to configure is the
// connection it counts through: the starter carries no Redis connection details
// of its own, it reuses a *goredis.Client bean registered by starter-go-redis.
//
// The rate-limit knobs (rate-limit/burst/algorithm/window/rate-limit-max-wait)
// are NOT here: they are fields of resilience.ClientPolicy, configured per service on
// the governance rule document, and read by the executor that spends the
// budget.
type Config struct {
	// Client names the *goredis.Client bean whose Redis instance backs the
	// shared counters. The bean is provided by starter-go-redis under
	// spring.go-redis.instances.<Client>. The key is required: an empty value is
	// a fail-fast wiring error, since silently defaulting would hide a
	// misconfiguration until the first protected call.
	Client string `value:"${client}"`
}
