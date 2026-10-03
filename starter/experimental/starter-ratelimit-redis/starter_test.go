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
	"context"
	"errors"
	"go-spring.org/cloud/chain"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	"go-spring.org/spring/gs"
	goredis "go-spring.org/starter-go-redis"
	experimental "go-spring.org/starter-go-redis/experimental"
	"go-spring.org/stdlib/flatten"
)

// newMiniRedis starts an in-process Redis and returns it with a client over it.
// Several clients over one server model several replicas sharing one Redis.
func newMiniRedis(t *testing.T) (*miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	srv := miniredis.RunT(t)
	cli := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = cli.Close() })
	return srv, cli
}

// newStore builds the store the starter contributes, over cli. Two stores over
// one miniredis (and two clients) stand in for two replicas: each counts
// nothing locally, both spend the same Redis budget.
func newStore(t *testing.T, cli redis.UniversalClient) resilience.Counters {
	t.Helper()
	c, err := experimental.NewCounters(cli)
	if err != nil {
		t.Fatalf("NewCounters: %v", err)
	}
	return c
}

// TestSharedBudgetAcrossReplicas pins the point of the starter: burst 5 with a
// rate too small to refill during the test is granted exactly five times in
// total, however the calls interleave between two independent stores.
func TestSharedBudgetAcrossReplicas(t *testing.T) {
	_, cli := newMiniRedis(t)
	a := newStore(t, cli)
	b := newStore(t, cli)
	ctx := context.Background()
	p := resilience.RateSpec{RateLimit: 0.001, Burst: 5}

	allowed := 0
	for i := 0; i < 5; i++ {
		s := a
		if i%2 == 1 {
			s = b
		}
		if err := s.Allow(ctx, "api", p, 1); err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		allowed++
	}
	if allowed != 5 {
		t.Fatalf("interleaved allows = %d, want 5 (shared budget)", allowed)
	}
	// Both replicas must now be refused: the bucket lives in Redis, not in either.
	for name, s := range map[string]resilience.Counters{"a": a, "b": b} {
		if err := s.Allow(ctx, "api", p, 1); !errors.Is(err, chain.ErrRateLimited) {
			t.Fatalf("replica %s after drain: err=%v, want ErrRateLimited", name, err)
		}
	}
}

// TestScopeIsolation proves one store keeps each scope's budget apart: draining
// one scope leaves another scope's budget untouched.
func TestScopeIsolation(t *testing.T) {
	_, cli := newMiniRedis(t)
	c := newStore(t, cli)
	ctx := context.Background()
	p := resilience.RateSpec{RateLimit: 0.001, Burst: 2}

	for i := 0; i < 2; i++ {
		if err := c.Allow(ctx, "tenant-a", p, 1); err != nil {
			t.Fatalf("tenant-a allow %d: %v", i, err)
		}
	}
	if err := c.Allow(ctx, "tenant-a", p, 1); !errors.Is(err, chain.ErrRateLimited) {
		t.Fatalf("tenant-a budget should be exhausted, err=%v", err)
	}
	// A different scope has an independent budget.
	if err := c.Allow(ctx, "tenant-b", p, 1); err != nil {
		t.Fatalf("tenant-b: %v", err)
	}
}

// TestBatchAllowAllOrNothing pins the atomic all-or-nothing batch: a request for
// more tokens than are left consumes nothing, and n <= 0 is a no-op allow.
func TestBatchAllowAllOrNothing(t *testing.T) {
	_, cli := newMiniRedis(t)
	c := newStore(t, cli)
	ctx := context.Background()
	p := resilience.RateSpec{RateLimit: 0.001, Burst: 5}

	if err := c.Allow(ctx, "api", p, 3); err != nil {
		t.Fatalf("Allow(3) on full bucket: %v", err)
	}
	// Only 2 left: asking for 3 must consume nothing.
	if err := c.Allow(ctx, "api", p, 3); !errors.Is(err, chain.ErrRateLimited) {
		t.Fatalf("Allow(3) with 2 left: err=%v, want ErrRateLimited", err)
	}
	// The 2 survivors are still there.
	if err := c.Allow(ctx, "api", p, 2); err != nil {
		t.Fatalf("Allow(2) after refused batch: %v", err)
	}
	// n <= 0 is a no-op allow.
	if err := c.Allow(ctx, "api", p, 0); err != nil {
		t.Fatalf("Allow(0): %v", err)
	}
}

// TestUnlimitedPolicy proves a zero RateLimit is an unlimited pass-through with
// no Redis round-trip at all.
func TestUnlimitedPolicy(t *testing.T) {
	srv, cli := newMiniRedis(t)
	c := newStore(t, cli)
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		if err := c.Allow(ctx, "api", resilience.RateSpec{}, 1); err != nil {
			t.Fatalf("unlimited allow %d: %v", i, err)
		}
	}
	if srv.Exists("ratelimit:api") {
		t.Fatal("an unlimited policy must not touch Redis")
	}
}

// TestConcurrency proves the shared budget holds under racing callers: 50
// goroutines on one store grant exactly the burst of 10.
func TestConcurrency(t *testing.T) {
	_, cli := newMiniRedis(t)
	c := newStore(t, cli)
	ctx := context.Background()
	p := resilience.RateSpec{RateLimit: 0.001, Burst: 10}

	var granted atomic.Int64
	var w sync.WaitGroup
	for i := 0; i < 50; i++ {
		w.Add(1)
		go func() {
			defer w.Done()
			err := c.Allow(ctx, "api", p, 1)
			switch {
			case err == nil:
				granted.Add(1)
			case errors.Is(err, chain.ErrRateLimited):
			default:
				t.Errorf("Allow: %v", err)
			}
		}()
	}
	w.Wait()
	if got := granted.Load(); got != 10 {
		t.Fatalf("concurrent grants = %d, want exactly the burst of 10", got)
	}
}

// TestKeyTTLPreventsColdKeyPileup pins the expiry the store leaves on every
// scope key, so abandoned budgets age out instead of leaking.
func TestKeyTTLPreventsColdKeyPileup(t *testing.T) {
	srv, cli := newMiniRedis(t)
	c := newStore(t, cli)
	if err := c.Allow(context.Background(), "api", resilience.RateSpec{RateLimit: 2, Burst: 4}, 1); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	// ceil(burst/rate)+1 = 3s for the policy above.
	ttl := srv.TTL("ratelimit:api")
	if ttl < 2*time.Second || ttl > 4*time.Second {
		t.Fatalf("ttl = %v, want ~3s", ttl)
	}
}

// TestWiring_MissingClientNameFailsStartup pins the fail-fast contract: a
// present block with an empty client, and an absent block, both refuse to wire
// and both name the property. setup returns before touching the BeanProvider on
// that path, so passing nil is safe.
func TestWiring_MissingClientNameFailsStartup(t *testing.T) {
	p := flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{
		"spring.ratelimit.redis.client": "",
	}))
	err := setup(nil, p)
	if err == nil {
		t.Fatal("an empty client name must fail wiring")
	}
	if !strings.Contains(err.Error(), "spring.ratelimit.redis.client") {
		t.Fatalf("error %q does not name the missing property", err.Error())
	}

	// No key at all: the required value tag rejects it, and the message still
	// names the property.
	p = flatten.NewPropertiesStorage(flatten.NewProperties(nil))
	err = setup(nil, p)
	if err == nil {
		t.Fatal("a missing client name must fail wiring")
	}
	if !strings.Contains(err.Error(), "spring.ratelimit.redis.client") {
		t.Fatalf("error %q does not name the missing property", err.Error())
	}
}

// TestWiring_ContributesRedisStore proves the contribution contract end to end:
// with the block configured, the container resolves resilience.Counters to the
// Redis store. That bean is what the bundled driver injects, which
// is how every executor ends up spending the shared budget.
func TestWiring_ContributesRedisStore(t *testing.T) {
	srv, cli := newMiniRedis(t)

	// The *goredis.Client bean starter-go-redis would publish, built the same
	// way that starter builds one (identity + observation, no governance).
	w, err := goredis.NewClient(cli, goredis.Config{}, nil, cloud.ClientParams{})
	if err != nil {
		t.Fatalf("goredis.NewClient: %v", err)
	}

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.ratelimit.redis.client", "cache")
		app.Provide(func() *goredis.Client { return w }).Name("cache")
	}).RunTest(t, func(ts *struct {
		Counters resilience.Counters `autowire:"?"`
	}) {
		if ts.Counters == nil {
			t.Fatal("the configured starter must contribute a resilience.Counters bean")
		}
		// Only the Redis store can write the bucket key, which is what makes
		// this an assertion about WHICH store was injected.
		err := ts.Counters.Allow(context.Background(), "api",
			resilience.RateSpec{RateLimit: 100, Burst: 5}, 1)
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !srv.Exists("ratelimit:api") {
			t.Fatal("the injected store did not reach Redis")
		}
	})
}

// TestWiring_NoBlockContributesNoStore pins the opt-in: an imported but
// unconfigured starter contributes nothing, and nothing else does either — the
// container holds no counter store at all. That is the default the resilience
// driver is built for (its store argument is optional): each executor counts in
// a private store of its own, which still means one budget per service label.
func TestWiring_NoBlockContributesNoStore(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {}).RunTest(t, func(ts *struct {
		Counters resilience.Counters `autowire:"?"`
	}) {
		if ts.Counters != nil {
			t.Fatal("without spring.ratelimit.redis no counter store may be contributed")
		}
	})
}
