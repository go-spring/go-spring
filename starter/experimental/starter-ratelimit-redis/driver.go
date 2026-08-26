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

import (
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"go-spring.org/cloud/governance/resilience"
	goredis "go-spring.org/starter-go-redis"
	experimental "go-spring.org/starter-go-redis/experimental"
)

// Driver is the bean this starter exports per instance. It adapts a named
// *redis.Client into a [resilience.LimiterDriver] registered under the
// configured driver name, so consumers that resolve limiters by name —
// starter-gateway's rateLimit filter (`driver=`), [resilience.GetLimiter], any
// app code — get cross-replica limiting with zero code change.
//
// The token-bucket algorithm itself (atomic refill/consume Lua script, key
// namespacing, burst defaulting) lives in starter-go-redis/experimental; this
// module only contributes the config-driven wiring, mirroring how
// starter-resilience wires sentinel into the resilience Driver registry.
type Driver struct {
	// Name is the limiter driver name this instance is registered under in the
	// resilience limiter registry ([resilience.GetLimiter]).
	Name string

	mu     sync.RWMutex
	client redis.UniversalClient
}

var _ resilience.LimiterDriver = (*Driver)(nil)

// NewRateLimiter builds a Redis-backed [resilience.RateLimiter] from p. The
// underlying client is captured at call time, so a re-wired container (tests)
// swaps clients without rebuilding registered limiters.
func (d *Driver) NewRateLimiter(p resilience.LimitPolicy) (resilience.RateLimiter, error) {
	d.mu.RLock()
	c := d.client
	d.mu.RUnlock()
	if c == nil {
		return nil, fmt.Errorf("ratelimit-redis: driver %q has no redis client bound", d.Name)
	}
	return experimental.NewRateLimiter(c, p), nil
}

// bind attaches c as the client this driver hands to new limiters.
func (d *Driver) bind(c redis.UniversalClient) {
	d.mu.Lock()
	d.client = c
	d.mu.Unlock()
}

// Client exposes the bound redis client, so the wiring bean (and tests) can
// reach the concrete *goredis.Client wrapper without re-injecting by name.
func (d *Driver) Client() redis.UniversalClient {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.client
}

// drivers holds one Driver singleton per driver name. Registration with the
// resilience limiter registry is once-per-process (the registry panics on
// duplicates), but the client binding is refreshed on every wiring pass —
// a second gs.RunTest in the same test binary rebinds instead of panicking.
var drivers sync.Map // name (string) -> *Driver

// driverFor returns the Driver registered under name, creating and registering
// it with [resilience.RegisterLimiter] on first use, then binding client.
func driverFor(name string, client *goredis.Client) *Driver {
	v, ok := drivers.Load(name)
	if ok {
		d := v.(*Driver)
		d.bind(client.UniversalClient)
		return d
	}
	d := &Driver{Name: name}
	actual, loaded := drivers.LoadOrStore(name, d)
	d = actual.(*Driver)
	d.bind(client.UniversalClient)
	if !loaded {
		resilience.RegisterLimiter(name, d)
	}
	return d
}
