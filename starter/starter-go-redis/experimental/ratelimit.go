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
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/errutil"
)

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

// redisCounters is a Redis-backed [resilience.Counters]: one token budget per
// scope, shared by every replica, in contrast to the bundled in-memory store,
// which counts each replica on its own. The counters live in Redis and the
// refill/consume runs as one atomic script, so replicas cannot overspend a
// budget between them.
//
// Two limits are deliberate. Sliding-window scopes are counted as token buckets
// (the atomic script is what makes a shared budget correct, and a window does not
// map onto one script cheaply). Queueing ([resilience.ClientPolicy.RateLimitMaxWait])
// is not offered either: waiting for a token would mean polling Redis, so a redis
// store rejects an over-limit unit immediately — keep queueing for the in-process
// store.
type redisCounters struct {
	client redis.UniversalClient
	prefix string
}

var _ resilience.Counters = (*redisCounters)(nil)

// NewCounters returns a Redis-backed [resilience.Counters] over client.
// Contribute it to the container and the resilience driver injects it, so every
// executor in the process — and every replica running this starter — spends one
// budget per scope, instead of each executor counting privately:
//
//	gs.Provide(func() (resilience.Counters, error) {
//	    return experimental.NewCounters(client)
//	})
//
// It returns the interface, so the bean is indexed under [resilience.Counters]
// and needs no Export. A nil client fails here with an error rather than
// surfacing as a counter error on the first protected call.
func NewCounters(client redis.UniversalClient) (resilience.Counters, error) {
	if client == nil {
		return nil, errutil.Explain(nil, "starter-go-redis: nil redis counters client")
	}
	return &redisCounters{client: client, prefix: "ratelimit:"}, nil
}

// Allow charges n units of scope's budget to Redis. Keys are namespaced under
// "ratelimit:" so per-scope counters never collide with application data.
func (c *redisCounters) Allow(ctx context.Context, scope string, p resilience.ClientPolicy, n int) error {
	if p.RateLimit <= 0 || n <= 0 { // no budget configured
		return nil
	}
	burst := p.Burst
	if burst <= 0 {
		if burst = int(p.RateLimit); burst < 1 {
			burst = 1
		}
	}
	res, err := redisTokenBucket.Run(ctx, c.client,
		[]string{c.prefix + scope},
		p.RateLimit, float64(burst), time.Now().UnixMilli(), n,
	).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return resilience.ErrRateLimited
	}
	return nil
}
