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
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/capability/source"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// svcRule arms a whole-service rate rule for label.
func svcRule(label string, p ratelimit.Policy) resilience.RateLimitSettings {
	return resilience.RateLimitSettings{Services: []resilience.RateLimitService{{
		Service: label,
		Rules: []resilience.RateLimitRule{{
			ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			Policy:      p,
		}},
	}}}
}

// newExternalExecutor builds one executor over a private manager armed with p.
// It is the stand-in for the constructor call sites outside the package use.
func newExternalExecutor(t *testing.T, label string, p ratelimit.Policy) chain.Executor {
	t.Helper()
	factories, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	assert.That(t, err).Nil()
	m, err := resilience.NewManager(factories)
	assert.That(t, err).Nil()
	assert.That(t, m.ApplyRateLimit(svcRule(label, p))).Nil()
	return m.Wrap(label, nil)
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
	m, err := resilience.NewManager(factories)
	assert.That(t, err).Nil()

	sliding := ratelimit.Policy{RateLimit: 1, Limiter: "redis:sliding-window"}
	assert.Error(t, m.ApplyRateLimit(svcRule("svc", sliding))).NotNil()

	bucket := ratelimit.Policy{RateLimit: 1, Limiter: "token-bucket"}
	assert.Error(t, m.ApplyRateLimit(svcRule("svc", bucket))).Nil()
}

// TestRateCounterCountsPerManager pins that a rate budget belongs to the engine
// that counts it. Two private managers each own their counter, so they do not
// share a budget — while one manager still keeps a single budget per label.
func TestRateCounterCountsPerManager(t *testing.T) {
	p := ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1}
	ctx := context.Background()
	e1 := newExternalExecutor(t, "svc", p)
	e2 := newExternalExecutor(t, "svc", p)
	run := func(e chain.Executor) error {
		return e.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })
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
	m, err := resilience.NewManager(factories)
	assert.That(t, err).Nil()
	p := ratelimit.Policy{RateLimit: 1, Burst: 1, Limiter: "token-bucket"}
	assert.That(t, m.ApplyRateLimit(svcRule("svc", p))).Nil()
	e1 := m.Wrap("svc", nil)
	e2 := m.Wrap("svc", nil)
	ctx := context.Background()
	assert.That(t, e1.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil })).Nil()
	assert.That(t, errors.Is(e2.Execute(ctx, chain.Operation{Service: "svc"}, new(chain.Recorder), func(context.Context, *chain.Round) error { return nil }), chain.ErrRateLimited)).True()
	// The charge landed under the service name, not some private key.
	assert.That(t, store.spent["svc"]).Equal(1)
}

// TestRateLimitServiceRequiredAtBind pins the single-field rule that lives in
// the expr tag rather than in Validate.
func TestRateLimitServiceRequiredAtBind(t *testing.T) {
	_, err := source.Parse[resilience.RateLimitSettings]("ratelimit.properties", []byte(`
services[0].rules[0].action.exact=GET /x
services[0].rules[0].rate-limit=1
`), "")
	assert.Error(t, err).NotNil()

	_, err = source.Parse[resilience.RateLimitSettings]("ratelimit.properties", []byte(`
services[0].service=svc
services[0].rules[0].action.exact=GET /x
services[0].rules[0].rate-limit=1
`), "")
	assert.Error(t, err).Nil()
}

// TestRateLimitIsZeroCoversNewKnobs pins that IsZero sees the newer knobs: a
// policy setting ONLY one of them is no longer "no protection".
func TestRateLimitIsZeroCoversNewKnobs(t *testing.T) {
	assert.That(t, (ratelimit.Policy{WarmUp: time.Second}).IsZero()).False()
	assert.That(t, (ratelimit.Policy{ColdFactor: 3}).IsZero()).False()
}
