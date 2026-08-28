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
	// Loud validation instead of silent misbehaviour: this driver maps a policy
	// onto one atomic Lua token-bucket script, so a SlidingWindow policy (or any
	// unknown algorithm) would be silently downgraded to a token bucket with a
	// completely different burst profile. Refuse at wiring time instead.
	if p.Algorithm != "" && p.Algorithm != resilience.TokenBucket {
		return nil, fmt.Errorf("ratelimit-redis: driver %q implements %q only, policy asks for %q (set Algorithm to %q or leave it empty)",
			d.Name, resilience.TokenBucket, p.Algorithm, resilience.TokenBucket)
	}
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

// driverOwners records which instance claimed which driver name, so two
// instances configured with the same name (explicitly via driver=, or by
// colliding with another instance's defaulted name) fail at startup with an
// error naming both instances instead of silently sharing one limiter.
// Re-wiring the same instance (a second gs.RunTest pass) re-claims its own
// name and stays allowed.
var (
	driverOwnerMu sync.Mutex
	driverOwners  = map[string]string{} // driver name -> instance name
)

// claimDriverName reserves driver for instance, or returns a friendly startup
// error when another instance already claimed it.
func claimDriverName(driver, instance string) error {
	driverOwnerMu.Lock()
	defer driverOwnerMu.Unlock()
	if owner, ok := driverOwners[driver]; ok && owner != instance {
		return fmt.Errorf("ratelimit-redis: limiter driver name %q claimed by both instance %q and instance %q — set distinct spring.ratelimit.redis.<name>.driver values",
			driver, owner, instance)
	}
	driverOwners[driver] = instance
	return nil
}

// driverFor returns the Driver registered under name, creating and registering
// it with [resilience.RegisterLimiter] on first use, then binding client.
// A driver name already registered by someone else (e.g. the built-in
// "default" driver, or another module's limiter) is a clear startup error
// instead of the registry's duplicate panic.
func driverFor(name string, client *goredis.Client) (*Driver, error) {
	v, ok := drivers.Load(name)
	if ok {
		d := v.(*Driver)
		d.bind(client.UniversalClient)
		return d, nil
	}
	if _, err := resilience.GetLimiter(name); err == nil {
		return nil, fmt.Errorf("ratelimit-redis: limiter driver name %q is already registered by another module", name)
	}
	d := &Driver{Name: name}
	actual, loaded := drivers.LoadOrStore(name, d)
	d = actual.(*Driver)
	d.bind(client.UniversalClient)
	if !loaded {
		resilience.RegisterLimiter(name, d)
	}
	return d, nil
}
