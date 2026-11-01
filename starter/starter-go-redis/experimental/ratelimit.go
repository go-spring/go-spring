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
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/stdlib/errutil"
)

// TokenBucketLimiter is the limiter-kind key a rule names to charge this store:
// the Redis backend plus the token-bucket algorithm, in one name.
const TokenBucketLimiter string = "redis:token-bucket"

// redisTokenBucket is the atomic token-bucket refill/consume, evaluated entirely
// inside Redis so concurrent replicas share one budget. State lives in a hash
// (tokens + last-refill ms); the key auto-expires once idle long enough to
// refill fully, so abandoned scopes never leak. It returns 1 when the requested
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

// redisProvider is the [ratelimit.Provider] behind [TokenBucketLimiter]: one
// token budget per scope, shared by every replica, in contrast to a process-local
// limiter, which each counter keeps to itself. The counters live in Redis and
// the refill/consume runs as one atomic script, so replicas cannot overspend a
// budget between them.
//
// Two limits are deliberate. The kind is token-bucket only — the atomic script
// is what makes a shared budget correct, and a window does not map onto one
// script cheaply — so a sliding-window rule is refused at apply time because no
// factory is registered for it. Queueing ([ratelimit.Policy.MaxWait]) is not
// offered either: waiting for a token would mean polling Redis, so the store
// rejects an over-limit unit immediately — keep queueing for the process-local
// limiter.
type redisProvider struct {
	client redis.UniversalClient
	prefix string
}

var _ ratelimit.Provider = (*redisProvider)(nil)

// NewProvider returns the Redis-backed [ratelimit.Provider] over client.
// Contribute it to the container under [TokenBucketLimiter] and every rule that
// names that kind spends one budget per scope, across every replica:
//
//	gs.Provide(func(client *redis.Client) (ratelimit.Provider, error) {
//	    return experimental.NewProvider(client)
//	}).Name(string(experimental.TokenBucketLimiter))
//
// A nil client fails here with an error rather than surfacing as a counter error
// on the first protected call.
func NewProvider(client redis.UniversalClient) (ratelimit.Provider, error) {
	if client == nil {
		return nil, errutil.Explain(nil, "starter-go-redis: nil redis rate-limit client")
	}
	return &redisProvider{client: client, prefix: "ratelimit:"}, nil
}

// Algorithms reports the kinds this provider answers: the Redis backend's token
// bucket only.
func (f *redisProvider) Algorithms() []string {
	return []string{TokenBucketLimiter}
}

// New binds a new handle to scope. The handle is cheap; the state stays in Redis.
func (f *redisProvider) New(algorithm string, scope string) (ratelimit.Limiter, error) {
	if algorithm != TokenBucketLimiter {
		return nil, errutil.Explain(nil, "starter-go-redis: redis provider does not support %q", algorithm)
	}
	return &redisLimiter{f: f, scope: scope}, nil
}

// redisLimiter charges one scope's budget to Redis. Keys are namespaced under
// "ratelimit:" so per-scope counters never collide with application data.
type redisLimiter struct {
	f     *redisProvider
	scope string
}

var _ ratelimit.Limiter = (*redisLimiter)(nil)

func (l *redisLimiter) Allow(ctx context.Context, policy ratelimit.Policy, n int) (bool, error) {
	if policy.RateLimit <= 0 || n <= 0 { // no budget configured
		return true, nil
	}
	burst := policy.Burst
	if burst <= 0 {
		if burst = int(policy.RateLimit); burst < 1 {
			burst = 1
		}
	}
	res, err := redisTokenBucket.Run(ctx, l.f.client,
		[]string{l.f.prefix + l.scope},
		policy.RateLimit, float64(burst), time.Now().UnixMilli(), n,
	).Int64()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}
