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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/capability/loadbalance"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/capability/source"
	admissionplane "go-spring.org/cloud/governance/controlplane/admission"
	loadbalanceplane "go-spring.org/cloud/governance/controlplane/loadbalance"
	"go-spring.org/cloud/governance/dataplane/inbound/admission"
	serverfault "go-spring.org/cloud/governance/dataplane/inbound/fault"
	clientfault "go-spring.org/cloud/governance/dataplane/outbound/fault"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/cloud/traffic"
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

// store is a test handle that binds a scope to the factory the starter
// contributes, so the tests keep a scope-shaped Allow.
type store struct{ f ratelimit.Provider }

func (s store) Allow(ctx context.Context, scope string, spec ratelimit.Policy, n int) (bool, error) {
	lim, err := s.f.New(experimental.TokenBucketLimiter, scope)
	if err != nil {
		return false, err
	}
	return lim.Allow(ctx, spec, n)
}

// newStore builds the factory the starter contributes, over cli. Two stores over
// one miniredis (and two clients) stand in for two replicas: each counts
// nothing locally, both spend the same Redis budget.
func newStore(t *testing.T, cli redis.UniversalClient) store {
	t.Helper()
	f, err := experimental.NewProvider(cli)
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	return store{f: f}
}

// TestSharedBudgetAcrossReplicas pins the point of the starter: burst 5 with a
// rate too small to refill during the test is granted exactly five times in
// total, however the calls interleave between two independent stores.
func TestSharedBudgetAcrossReplicas(t *testing.T) {
	_, cli := newMiniRedis(t)
	a := newStore(t, cli)
	b := newStore(t, cli)
	ctx := context.Background()
	p := ratelimit.Policy{RateLimit: 0.001, Burst: 5}

	allowed := 0
	for i := 0; i < 5; i++ {
		s := a
		if i%2 == 1 {
			s = b
		}
		if _, err := s.Allow(ctx, "api", p, 1); err != nil {
			t.Fatalf("allow %d: %v", i, err)
		}
		allowed++
	}
	if allowed != 5 {
		t.Fatalf("interleaved allows = %d, want 5 (shared budget)", allowed)
	}
	// Both replicas must now be refused: the bucket lives in Redis, not in either.
	for name, s := range map[string]store{"a": a, "b": b} {
		if admitted, err := s.Allow(ctx, "api", p, 1); err != nil || admitted {
			t.Fatalf("replica %s after drain: admitted=%v err=%v, want refusal", name, admitted, err)
		}
	}
}

// TestScopeIsolation proves one store keeps each scope's budget apart: draining
// one scope leaves another scope's budget untouched.
func TestScopeIsolation(t *testing.T) {
	_, cli := newMiniRedis(t)
	c := newStore(t, cli)
	ctx := context.Background()
	p := ratelimit.Policy{RateLimit: 0.001, Burst: 2}

	for i := 0; i < 2; i++ {
		if _, err := c.Allow(ctx, "tenant-a", p, 1); err != nil {
			t.Fatalf("tenant-a allow %d: %v", i, err)
		}
	}
	if admitted, err := c.Allow(ctx, "tenant-a", p, 1); err != nil || admitted {
		t.Fatalf("tenant-a budget should be exhausted, admitted=%v err=%v", admitted, err)
	}
	// A different scope has an independent budget.
	if _, err := c.Allow(ctx, "tenant-b", p, 1); err != nil {
		t.Fatalf("tenant-b: %v", err)
	}
}

// TestBatchAllowAllOrNothing pins the atomic all-or-nothing batch: a request for
// more tokens than are left consumes nothing, and n <= 0 is a no-op allow.
func TestBatchAllowAllOrNothing(t *testing.T) {
	_, cli := newMiniRedis(t)
	c := newStore(t, cli)
	ctx := context.Background()
	p := ratelimit.Policy{RateLimit: 0.001, Burst: 5}

	if _, err := c.Allow(ctx, "api", p, 3); err != nil {
		t.Fatalf("Allow(3) on full bucket: %v", err)
	}
	// Only 2 left: asking for 3 must consume nothing.
	if admitted, err := c.Allow(ctx, "api", p, 3); err != nil || admitted {
		t.Fatalf("Allow(3) with 2 left: admitted=%v err=%v, want refusal", admitted, err)
	}
	// The 2 survivors are still there.
	if _, err := c.Allow(ctx, "api", p, 2); err != nil {
		t.Fatalf("Allow(2) after refused batch: %v", err)
	}
	// n <= 0 is a no-op allow.
	if _, err := c.Allow(ctx, "api", p, 0); err != nil {
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
		if _, err := c.Allow(ctx, "api", ratelimit.Policy{}, 1); err != nil {
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
	p := ratelimit.Policy{RateLimit: 0.001, Burst: 10}

	var granted atomic.Int64
	var w sync.WaitGroup
	for i := 0; i < 50; i++ {
		w.Add(1)
		go func() {
			defer w.Done()
			admitted, err := c.Allow(ctx, "api", p, 1)
			switch {
			case err != nil:
				t.Errorf("Allow: %v", err)
			case admitted:
				granted.Add(1)
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
	if _, err := c.Allow(context.Background(), "api", ratelimit.Policy{RateLimit: 2, Burst: 4}, 1); err != nil {
		t.Fatalf("Allow: %v", err)
	}
	// ceil(burst/rate)+1 = 3s for the policy above.
	ttl := srv.TTL("ratelimit:api")
	if ttl < 2*time.Second || ttl > 4*time.Second {
		t.Fatalf("ttl = %v, want ~3s", ttl)
	}
}

// TestWiring_MissingClientNameFailsStartup pins the fail-fast contract: a
// present block with an empty client, and an absent block, both refuse to wire.
// The empty case is caught by the field's expr (which names Config.Client, not
// the full property path); the absent case is caught by the required tag, which
// does name the property. setup returns before touching the BeanProvider on
// that path, so passing nil is safe.
func TestWiring_MissingClientNameFailsStartup(t *testing.T) {
	p := flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{
		"spring.ratelimit.redis.client": "",
	}))
	err := setup(nil, p)
	if err == nil {
		t.Fatal("an empty client name must fail wiring")
	}
	if !strings.Contains(err.Error(), "Config.Client") {
		t.Fatalf("error %q does not name the client field", err.Error())
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
// with the block configured, the container resolves the limiter factory for
// Redis store. That bean is what the bundled driver injects, which
// is how every executor ends up spending the shared budget.
func TestWiring_ContributesRedisStore(t *testing.T) {
	srv, cli := newMiniRedis(t)

	// The *goredis.Client bean starter-go-redis would publish, built the same
	// way that starter builds one (identity + observation, no governance).
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	fact2, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	adm, err := admission.NewManager(fact2)
	if err != nil {
		t.Fatal(err)
	}
	center := &governance.Runtime{
		Resilience:  res,
		LoadBalance: loadbalance.NewManager(loadbalance.NewFactory([]loadbalance.Provider{loadbalance.NewBuiltinProvider()})),
	Admission:   adm,
		ClientFault: clientfault.NewManager(),
		ServerFault: serverfault.NewManager(),
		Directory:   discovery.NewManager(nil),
		Traffic:     traffic.NewDefaultPropagator(traffic.DefaultBinding()),
	}
	if _, err := loadbalanceplane.NewCenter(context.Background(), center.LoadBalance, source.NewPushSource(loadbalance.Settings{})); err != nil {
		t.Fatal(err)
	}
	if _, err := admissionplane.NewCenter(context.Background(), center.Admission, source.NewPushSource(admission.Settings{})); err != nil {
		t.Fatal(err)
	}
	w, err := goredis.NewClient(cli, goredis.Config{}, nil, center)
	if err != nil {
		t.Fatalf("goredis.NewClient: %v", err)
	}

	gs.Web(false).Configure(func(app gs.App) {
		app.Property("spring.ratelimit.redis.client", "cache")
		app.Provide(func() *goredis.Client { return w }).Name("cache")
	}).RunTest(t, func(ts *struct {
		Factories *ratelimit.Factory `autowire:"?"`
	}) {
		if ts.Factories == nil {
			t.Fatal("the container must contribute the collected limiter kinds")
		}
		if _, err := ts.Factories.NewLimiter(experimental.TokenBucketLimiter, "api"); err != nil {
			t.Fatal("the configured starter must contribute its limiter kind")
		}
		// Only the Redis store can write the bucket key, which is what makes
		// this an assertion about WHICH kind was contributed.
		lim, err := ts.Factories.NewLimiter(experimental.TokenBucketLimiter, "api")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		_, err = lim.Allow(context.Background(), ratelimit.Policy{RateLimit: 100, Burst: 5}, 1)
		if err != nil {
			t.Fatalf("Allow: %v", err)
		}
		if !srv.Exists("ratelimit:api") {
			t.Fatal("the contributed store did not reach Redis")
		}
	})
}

// TestWiring_NoBlockContributesNoStore pins the opt-in: an imported but
// unconfigured starter contributes no Redis kind. The process-local kinds are
// still there (ratelimit's own starter registers them), so a rule that names
// none of this starter's kinds stays local.
func TestWiring_NoBlockContributesNoStore(t *testing.T) {
	gs.Web(false).Configure(func(app gs.App) {}).RunTest(t, func(ts *struct {
		Factories *ratelimit.Factory `autowire:"?"`
	}) {
		if ts.Factories == nil {
			t.Fatal("the container must contribute the collected limiter kinds")
		}
		if _, err := ts.Factories.NewLimiter(experimental.TokenBucketLimiter, "api"); err == nil {
			t.Fatal("without spring.ratelimit.redis the redis kind may not be contributed")
		}
		if _, err := ts.Factories.NewLimiter(ratelimit.TokenBucketAlgorithm, "api"); err != nil {
			t.Fatal("the built-in process-local kind must still be contributed")
		}
	})
}
