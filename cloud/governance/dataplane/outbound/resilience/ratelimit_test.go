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

package resilience_test

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// newExternalExecutor builds one executor over a private manager armed with p.
// It is the stand-in for the constructor call sites outside the package use.
func newExternalExecutor(t *testing.T, d resilience.Driver, label string, p ratelimit.Policy) chain.Executor {
	t.Helper()
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m := resilience.NewManager(d, factories)
	assert.That(t, m.ApplyRateLimit(resilience.RateLimitSettings{Default: p})).Nil()
	return m.ExecutorFor(label)
}

// testStore is a minimal shared-store factory for seam-level tests: it counts
// units per scope and rejects once a scope's count reaches the spec's burst. It
// exists only to observe HOW the executor charges a store, not to test
// algorithms.
type testStore struct {
	spent map[string]int
}

func (c *testStore) Algorithms() []string {
	return []string{ratelimit.TokenBucketAlgorithm}
}

func (c *testStore) New(_ string, scope string) (ratelimit.Limiter, error) {
	return &testScoped{store: c, scope: scope}, nil
}

type testScoped struct {
	store *testStore
	scope string
}

func (s *testScoped) Allow(_ context.Context, spec ratelimit.Policy, n int) (bool, error) {
	if s.store.spent == nil {
		s.store.spent = map[string]int{}
	}
	if s.store.spent[s.scope]+n > spec.Burst {
		return false, nil
	}
	s.store.spent[s.scope] += n
	return true, nil
}

// TestApplyRateLimitRejectsUnknownLimiterKind pins the apply-time check: a rule
// naming a kind no factory was registered for is refused, not silently counted
// as another algorithm.
func TestApplyRateLimitRejectsUnknownLimiterKind(t *testing.T) {
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{
		ratelimit.NewLocalProvider(),
	})
	assert.That(t, err).Nil()
	m := resilience.NewManager(resilience.NewDefaultDriver(), factories)

	sliding := ratelimit.Policy{RateLimit: 1, Limiter: "redis:sliding-window"}
	assert.Error(t, m.ApplyRateLimit(resilience.RateLimitSettings{Default: sliding})).NotNil()

	bucket := ratelimit.Policy{RateLimit: 1, Limiter: "token-bucket"}
	assert.Error(t, m.ApplyRateLimit(resilience.RateLimitSettings{Default: bucket})).Nil()
}

// TestDefaultDriverCountsPerExecutor pins the default the rate-limit stage
// follows every other stage's: its state belongs to the executor. Two executors
// built by one driver do NOT share a budget, because each counts in a private
// store — and that is still one budget per service through the manager, which
// builds one executor per label and hands it to every caller of that label.
func TestDefaultDriverCountsPerExecutor(t *testing.T) {
	d := resilience.NewDefaultDriver()
	p := ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1}
	ctx := context.Background()
	e1 := newExternalExecutor(t, d, "svc", p)
	e2 := newExternalExecutor(t, d, "svc", p)
	run := func(e chain.Executor) error {
		return e.Execute(ctx, chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil })
	}
	assert.That(t, run(e1)).Nil() // e1 spends its own budget...
	assert.That(t, run(e2)).Nil() // ...so e2 still has one of its own
	assert.That(t, errors.Is(run(e1), chain.ErrRateLimited)).True()
}

// TestDefaultDriverSharesSuppliedStore is the other half of that default: hand
// the driver a store and every executor it builds counts in that one, charged
// under the service each executor is bound to. This is what puts a single limit
// beyond the process (to every replica) when the store is over a shared
// backend.
func TestDefaultDriverSharesSuppliedStore(t *testing.T) {
	store := &testStore{}
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{store})
	assert.That(t, err).Nil()
	m := resilience.NewManager(resilience.NewDefaultDriver(), factories)
	p := ratelimit.Policy{RateLimit: 1, Burst: 1, Limiter: "token-bucket"}
	assert.That(t, m.ApplyRateLimit(resilience.RateLimitSettings{Default: p})).Nil()
	e1 := m.ExecutorFor("svc")
	e2 := m.ExecutorFor("svc")
	ctx := context.Background()
	assert.That(t, e1.Execute(ctx, chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil })).Nil()
	assert.That(t, errors.Is(e2.Execute(ctx, chain.Operation{}, new(chain.Recorder), func(context.Context) error { return nil }), chain.ErrRateLimited)).True()
	// The charge landed under the service name, not some private key.
	assert.That(t, store.spent["svc"]).Equal(1)
}
