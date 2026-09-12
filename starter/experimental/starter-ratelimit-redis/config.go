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

// Config configures one Redis-backed limiter driver instance bound under
// spring.ratelimit.redis.instances.<name>. Like the other redis-consuming starters it
// carries no Redis connection details: the driver reuses a *redis.Client bean
// registered by starter-go-redis, so topology changes (shared cluster vs
// dedicated cluster) stay config-only on the redis side.
type Config struct {
	// Client is the name of the *redis.Client bean backing this limiter driver.
	// The bean must be provided by starter-go-redis under spring.go-redis.instances.<Client>.
	// Empty is a fail-fast configuration error — silently defaulting would hide
	// a misconfiguration until the first (unlimited!) Allow call in production.
	Client string `value:"${client}"`

	// Driver is the name this limiter registers under in the resilience limiter
	// registry, i.e. the string consumers pass to [resilience.GetLimiter] (and
	// gateway's rateLimit `driver=` argument). Defaults to the instance name, so
	// multi-instance setups get one driver name each without extra config.
	Driver string `value:"${driver:=}"`
}
