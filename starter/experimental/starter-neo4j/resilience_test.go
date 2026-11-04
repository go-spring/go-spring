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

package StarterNeo4j

import (
	"context"
	"testing"

	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// fakeDriver is a nil-behavior DriverWithContext so a *Client wrapper can be
// exercised without a live Neo4j server. The guard tests never reach the
// driver: they drive RunWithResilience with a stubbed fn and assert the executor
// short-circuits before fn runs.
type fakeDriver struct {
	neo4j.DriverWithContext
}

// newGuardedNeo4jClient builds a Client whose executor comes from the default
// resilience driver. The embedded driver is nil.
func newGuardedNeo4jClient(t *testing.T, p ratelimit.Policy) *Client {
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m := resilience.NewManager(resilience.NewDefaultDriver(), fact)
	assert.Error(t, m.ApplyRateLimit(resilience.RateLimitSettings{Default: p})).Nil()
	return &Client{DriverWithContext: fakeDriver{}, exec: m.ExecutorFor("svc")}
}

// TestRunWithResiliencePassThrough proves the zero-config stance: a Client with
// no executor attached (governance off) runs fn inline unchanged.
func TestRunWithResiliencePassThrough(t *testing.T) {
	c := &Client{DriverWithContext: fakeDriver{}}
	assert.Error(t, RunWithResilience(context.Background(), c, chain.Operation{}, func(context.Context) error { return nil })).Nil()
}

// TestRunWithResilienceRateLimit confirms the flow-control path: once the
// burst is spent, fn is rejected without running.
func TestRunWithResilienceRateLimit(t *testing.T) {
	c := newGuardedNeo4jClient(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 1})
	var ran int
	fn := func(context.Context) error {
		ran++
		return nil
	}
	assert.Error(t, RunWithResilience(context.Background(), c, chain.Operation{}, fn)).Nil()
	assert.Error(t, RunWithResilience(context.Background(), c, chain.Operation{}, fn)).Is(chain.ErrRateLimited)
	assert.That(t, ran).Equal(1) // the rejected call never ran
}
