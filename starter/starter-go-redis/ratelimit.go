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

package StarterGoRedis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/cloud/resilience"
)

// redisTokenBucket is the atomic token-bucket refill/consume, evaluated entirely
// inside Redis so concurrent replicas share one budget. State lives in a hash
// (tokens + last-refill ms); the key auto-expires once idle long enough to
// refill fully, so abandoned keys never leak. It returns 1 when the requested
// tokens were granted, 0 otherwise.
var redisTokenBucket = redis.NewScript(`
local rate = tonumber(ARGV[1])
local burst = tonumber(ARGV[2])
local now = tonumber(ARGV[3])
local requested = tonumber(ARGV[4])
local data = redis.call('HMGET', KEYS[1], 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts = tonumber(data[2])
if tokens == nil then
  tokens = burst
  ts = now
end
local delta = math.max(0, now - ts) / 1000.0
tokens = math.min(burst, tokens + delta * rate)
local allowed = 0
if tokens >= requested then
  tokens = tokens - requested
  allowed = 1
end
redis.call('HSET', KEYS[1], 'tokens', tokens, 'ts', now)
local ttl = math.ceil(burst / rate) + 1
redis.call('EXPIRE', KEYS[1], ttl)
return allowed
`)

// redisRateLimiter is a Redis-backed [resilience.RateLimiter]: it enforces a
// single global token-bucket budget shared across every replica, in contrast to
// the builtin per-replica limiter. It is the distributed limiting seam — same
// interface, global enforcement. The sliding-window algorithm is not offered
// here; the token bucket is what maps cleanly onto an atomic Lua script.
type redisRateLimiter struct {
	client redis.UniversalClient
	prefix string
	rate   float64
	burst  float64
}

var _ resilience.RateLimiter = (*redisRateLimiter)(nil)

// NewRateLimiter builds a global [resilience.RateLimiter] over client from a
// [resilience.LimitPolicy]. A zero Rate yields an unlimited pass-through (no
// Redis round-trip). Burst defaults to a small multiple of Rate when unset. Keys
// are namespaced under "ratelimit:" so per-key budgets never collide with
// application data.
func NewRateLimiter(client redis.UniversalClient, p resilience.LimitPolicy) resilience.RateLimiter {
	burst := p.Burst
	if burst <= 0 {
		if burst = int(p.Rate); burst < 1 {
			burst = 1
		}
	}
	return &redisRateLimiter{
		client: client,
		prefix: "ratelimit:",
		rate:   p.Rate,
		burst:  float64(burst),
	}
}

func (l *redisRateLimiter) Allow(ctx context.Context, key string) (bool, error) {
	return l.AllowN(ctx, key, 1)
}

func (l *redisRateLimiter) AllowN(ctx context.Context, key string, n int) (bool, error) {
	if l.rate == 0 || n <= 0 { // unlimited / no-op
		return true, nil
	}
	now := time.Now().UnixMilli()
	res, err := redisTokenBucket.Run(ctx, l.client,
		[]string{l.prefix + key},
		l.rate, l.burst, now, n,
	).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

// Close is a no-op: the limiter borrows the shared client, whose lifecycle the
// owning bean's destructor manages.
func (l *redisRateLimiter) Close() error { return nil }
