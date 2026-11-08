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
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
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
	m, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	assert.Error(t, m.ApplyRateLimit(resilience.RateLimitSettings{Services: []resilience.RateLimitService{{
		Service: "svc",
		Rules: []resilience.RateLimitRule{{
			ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			Policy:      p,
		}},
	}}})).Nil()
	return &Client{DriverWithContext: fakeDriver{}, exec: m.Wrap("svc", nil), serviceLabel: "svc"}
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

// TestClientErrorIsNormalOutcome pins the classification: a Neo4j ClientError is
// the server's definite answer about the request, not evidence it is unhealthy.
func TestClientErrorIsNormalOutcome(t *testing.T) {
	clientErr := &neo4j.Neo4jError{Code: "Neo.ClientError.Statement.SyntaxError", Msg: "invalid syntax"}
	transientErr := &neo4j.Neo4jError{Code: "Neo.TransientError.General.DatabaseUnavailable", Msg: "unavailable"}
	assert.That(t, isClientError(clientErr)).True()
	assert.That(t, isClientError(transientErr)).False()
	assert.That(t, isClientError(errors.New("connection refused"))).False()
}

// TestRunWithResilienceRetriesTransient proves the driver's transient verdict
// reaches the retry loop while the caller still receives the original error.
func TestRunWithResilienceRetriesTransient(t *testing.T) {
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	assert.Error(t, m.ApplyTimeoutRetry(resilience.TimeoutRetrySettings{Services: []resilience.TimeoutRetryService{{
		Service: "svc",
		Rules: []resilience.TimeoutRetryRule{{
			ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			TimeoutRetry: resilience.TimeoutRetry{Retry: resilience.Retry{MaxRetries: 2}},
		}},
	}}})).Nil()
	c := &Client{DriverWithContext: fakeDriver{}, exec: m.Wrap("svc", nil), serviceLabel: "svc"}

	transient := &neo4j.Neo4jError{Code: "Neo.TransientError.General.DatabaseUnavailable", Msg: "unavailable"}
	var calls int
	err = RunWithResilience(context.Background(), c, chain.Operation{}, func(context.Context) error {
		calls++
		return transient
	})
	assert.Error(t, err).Is(transient)
	assert.Number(t, calls).Equal(3)
}

// TestRunWithResilienceClientErrorDoesNotTripBreaker proves the predicate reaches
// the executor: a client error is returned verbatim but counts as neither a
// breaker failure nor a retry trigger.
func TestRunWithResilienceClientErrorDoesNotTripBreaker(t *testing.T) {
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	assert.Error(t, m.ApplyBreaker(resilience.BreakerSettings{Services: []resilience.BreakerService{{
		Service: "svc",
		Rules: []resilience.BreakerRule{{
			ActionMatch:   matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
			BreakerPolicy: resilience.BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour},
		}},
	}}})).Nil()
	c := &Client{DriverWithContext: fakeDriver{}, exec: m.Wrap("svc", nil), serviceLabel: "svc"}

	clientErr := &neo4j.Neo4jError{Code: "Neo.ClientError.Statement.SyntaxError", Msg: "invalid syntax"}
	call := func() error {
		return RunWithResilience(context.Background(), c, chain.Operation{}, func(context.Context) error {
			return clientErr
		})
	}
	assert.Error(t, call()).Is(clientErr)
	// Had the client error counted, this second call would be short-circuited.
	assert.Error(t, call()).Is(clientErr)
}
