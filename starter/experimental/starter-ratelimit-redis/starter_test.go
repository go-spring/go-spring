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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"go-spring.org/cloud/governance/resilience"
	goredis "go-spring.org/starter-go-redis"
)

// newTestDriver starts an in-process Redis and returns a Driver bound to it,
// mirroring what the starter's bean ctor produces. Multiple drivers over the
// same server model multiple replicas sharing one Redis.
func newTestDriver(t *testing.T, name string) *Driver {
	t.Helper()
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	wrapped := &goredis.Client{UniversalClient: client}
	return driverFor(name, wrapped)
}

// newLimiter builds a limiter through the full driver path
// (Driver.NewRateLimiter → experimental Lua token bucket).
func newLimiter(t *testing.T, d *Driver, rate float64, burst int) resilience.RateLimiter {
	t.Helper()
	l, err := d.NewRateLimiter(resilience.LimitPolicy{Rate: rate, Burst: burst})
	if err != nil {
		t.Fatalf("NewRateLimiter: %v", err)
	}
	return l
}

func TestDriverRegisteredInLimiterRegistry(t *testing.T) {
	d := newTestDriver(t, "testreg")
	got, err := resilience.GetLimiter("testreg")
	if err != nil {
		t.Fatalf("GetLimiter: %v", err)
	}
	if got != resilience.LimiterDriver(d) {
		t.Fatal("registry returned a different driver instance")
	}
	// Re-wiring (a second container pass in the same process) must rebind the
	// client, not panic on duplicate registration.
	newTestDriver(t, "testreg")
}

func TestSharedBudgetAcrossInstances(t *testing.T) {
	// Two "replicas" (independent Driver + limiter objects) over one Redis: the
	// budget is global. burst=5, rate tiny so nothing refills during the test.
	d := newTestDriver(t, "shared")
	a := newLimiter(t, d, 0.001, 5)
	b := newLimiter(t, d, 0.001, 5)
	ctx := context.Background()

	allowed := 0
	for i := 0; i < 5; i++ {
		if i%2 == 0 {
			ok, err := a.Allow(ctx, "api")
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				allowed++
			}
		} else {
			ok, err := b.Allow(ctx, "api")
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				allowed++
			}
		}
	}
	if allowed != 5 {
		t.Fatalf("interleaved allows = %d, want 5 (shared budget)", allowed)
	}
	// Both replicas must now be refused: the bucket lives in Redis, not in either.
	if ok, err := a.Allow(ctx, "api"); err != nil || ok {
		t.Fatalf("replica A after drain: ok=%v err=%v", ok, err)
	}
	if ok, err := b.Allow(ctx, "api"); err != nil || ok {
		t.Fatalf("replica B after drain: ok=%v err=%v", ok, err)
	}
}

func TestKeyIsolation(t *testing.T) {
	d := newTestDriver(t, "iso")
	l := newLimiter(t, d, 0.001, 2)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow(ctx, "tenant-a"); !ok {
			t.Fatal("tenant-a should have its own budget")
		}
	}
	if ok, _ := l.Allow(ctx, "tenant-a"); ok {
		t.Fatal("tenant-a budget exhausted")
	}
	// A different key has an independent budget.
	if ok, err := l.Allow(ctx, "tenant-b"); err != nil || !ok {
		t.Fatalf("tenant-b: ok=%v err=%v", ok, err)
	}
}

func TestAllowNAllOrNothing(t *testing.T) {
	d := newTestDriver(t, "allown")
	l := newLimiter(t, d, 0.001, 5)
	ctx := context.Background()

	if ok, err := l.AllowN(ctx, "api", 3); err != nil || !ok {
		t.Fatalf("AllowN(3) on full bucket: ok=%v err=%v", ok, err)
	}
	// Only 2 left: asking for 3 must consume nothing.
	if ok, err := l.AllowN(ctx, "api", 3); err != nil || ok {
		t.Fatalf("AllowN(3) with 2 left: ok=%v err=%v", ok, err)
	}
	// The 2 survivors are still there.
	if ok, err := l.AllowN(ctx, "api", 2); err != nil || !ok {
		t.Fatalf("AllowN(2) after refused batch: ok=%v err=%v", ok, err)
	}
	// n<=0 is a no-op allow.
	if ok, err := l.AllowN(ctx, "api", 0); err != nil || !ok {
		t.Fatalf("AllowN(0): ok=%v err=%v", ok, err)
	}
}

func TestContinuousRefill(t *testing.T) {
	// High rate + short sleep so the test stays fast while remaining robust:
	// rate=100/s, burst=3, sleep ~100ms → ~10 new tokens.
	d := newTestDriver(t, "refill")
	l := newLimiter(t, d, 100, 3)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if ok, _ := l.Allow(ctx, "api"); !ok {
			t.Fatalf("initial allow %d denied", i)
		}
	}
	if ok, _ := l.Allow(ctx, "api"); ok {
		t.Fatal("bucket should be empty")
	}
	time.Sleep(150 * time.Millisecond)
	// Refilled by roughly rate*0.15 = 15 tokens (capped at burst).
	for i := 0; i < 3; i++ {
		if ok, err := l.Allow(ctx, "api"); err != nil || !ok {
			t.Fatalf("allow after refill %d: ok=%v err=%v", i, ok, err)
		}
	}
}

func TestConcurrency(t *testing.T) {
	d := newTestDriver(t, "conc")
	l := newLimiter(t, d, 0.001, 10)
	ctx := context.Background()

	var granted atomic.Int64
	var w sync.WaitGroup
	for i := 0; i < 50; i++ {
		w.Add(1)
		go func() {
			defer w.Done()
			ok, err := l.Allow(ctx, "api")
			if err != nil {
				t.Errorf("Allow: %v", err)
				return
			}
			if ok {
				granted.Add(1)
			}
		}()
	}
	w.Wait()
	if got := granted.Load(); got != 10 {
		t.Fatalf("concurrent grants = %d, want exactly the burst of 10", got)
	}
}

func TestUnlimitedPolicy(t *testing.T) {
	d := newTestDriver(t, "unlimited")
	l := newLimiter(t, d, 0, 0) // zero rate = pass-through, no redis round-trip
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		if ok, err := l.Allow(ctx, "api"); err != nil || !ok {
			t.Fatalf("unlimited allow %d: ok=%v err=%v", i, ok, err)
		}
	}
}

func TestKeyTTLPreventsColdKeyPileup(t *testing.T) {
	srv := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: srv.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	d := driverFor("ttl", &goredis.Client{UniversalClient: client})
	l := newLimiter(t, d, 2, 4)
	if _, err := l.Allow(context.Background(), "api"); err != nil {
		t.Fatal(err)
	}
	// The bucket hash carries an expiry so abandoned keys age out.
	ttl := srv.TTL("ratelimit:api")
	if ttl <= 0 {
		t.Fatalf("bucket key has no TTL: %v", ttl)
	}
	// ceil(burst/rate)+1 = 3s for the policy above.
	if ttl < 2*time.Second || ttl > 4*time.Second {
		t.Fatalf("ttl = %v, want ~3s", ttl)
	}
}
