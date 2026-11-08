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

package StarterGovernanceSentinel

import (
	"context"
	"testing"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// TestProviderIsSelectableAlgorithm proves the fusion: a resilience policy that
// names `sentinel` as its limiter runs sentinel's flow control inside the SAME
// pipeline — matching, timeout, retry and the neutral rejection are still the
// engine's, not a second executor's.
func TestProviderIsSelectableAlgorithm(t *testing.T) {
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{
		ratelimit.NewLocalProvider(),
		Provider{},
	})
	assert.Error(t, err).Nil()

	mgr, err := resilience.NewManager(fact)
	assert.That(t, err).Nil()
	apply := func(rate float64) {
		t.Helper()
		assert.Error(t, mgr.ApplyRateLimit(resilience.RateLimitSettings{
			Services: []resilience.RateLimitService{{
				Service: "svc-fusion",
				Rules: []resilience.RateLimitRule{{
					ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
					Policy:      ratelimit.Policy{Limiter: LimiterAlgorithm, RateLimit: rate},
				}},
			}},
		})).Nil()
	}
	apply(1)

	exec := mgr.Wrap("svc-fusion", nil)
	run := func() error {
		return exec.Execute(context.Background(), chain.Operation{Service: "svc-fusion"}, new(chain.Recorder),
			func(context.Context) error { return nil })
	}

	assert.Error(t, run()).Nil()
	assert.Error(t, run()).Is(chain.ErrRateLimited)

	// A pushed policy is a rule swap on the same counter: the next call sees the
	// new rate and the engine's neutral rejection still carries the verdict.
	apply(1000)
	assert.Error(t, run()).Nil()
}

// TestProviderRejectsUnknownAlgorithm pins the factory contract: the provider
// answers exactly one algorithm name.
func TestProviderRejectsUnknownAlgorithm(t *testing.T) {
	_, err := Provider{}.New("token-bucket", "svc")
	assert.Error(t, err).NotNil()
}

// factoryProbe makes gs build the factory so the test can inspect it.
type factoryProbe struct{}

// TestProviderIsRegisteredInTheContainer pins the import-time contribution: the
// process limiter factory indexes "sentinel" once this package is linked.
func TestProviderIsRegisteredInTheContainer(t *testing.T) {
	var f *ratelimit.Factory

	gs.Web(false).Configure(func(app gs.App) {
		app.Provide(func(fact *ratelimit.Factory) *factoryProbe {
			f = fact
			return &factoryProbe{}
		}).Export(gs.As[gs.Rooter]())
	}).RunTest(t, func(_ *struct{}) {
		if f == nil {
			t.Fatal("the limiter factory bean was not injected")
		}
		if _, err := f.NewLimiter(LimiterAlgorithm, "svc"); err != nil {
			t.Fatalf("the sentinel algorithm must be registered: %v", err)
		}
	})
}
