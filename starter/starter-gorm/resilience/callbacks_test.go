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

package gormresilience

import (
	"context"
	"errors"
	"testing"
	"time"

	"go-spring.org/cloud/governance/capability/matcher"
	"go-spring.org/cloud/governance/capability/ratelimit"
	"go-spring.org/cloud/governance/dataplane/outbound/chain"

	"go-spring.org/cloud/governance/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
	"gorm.io/gorm"
)

func newExec(t *testing.T, p any) chain.Executor {
	t.Helper()
	fact, err := ratelimit.NewFactory([]ratelimit.Provider{ratelimit.NewLocalProvider()})
	if err != nil {
		t.Fatal(err)
	}
	m, err := resilience.NewManager(fact)
	if err != nil {
		t.Fatal(err)
	}
	switch v := p.(type) {
	case nil:
	case resilience.TimeoutRetry:
		assert.Error(t, m.ApplyTimeoutRetry(resilience.TimeoutRetrySettings{Services: []resilience.TimeoutRetryService{{
			Service: "svc",
			Rules: []resilience.TimeoutRetryRule{{
				ActionMatch:  matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
				TimeoutRetry: v,
			}},
		}}})).Nil()
	case resilience.BreakerPolicy:
		assert.Error(t, m.ApplyBreaker(resilience.BreakerSettings{Services: []resilience.BreakerService{{
			Service: "svc",
			Rules: []resilience.BreakerRule{{
				ActionMatch:   matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
				BreakerPolicy: v,
			}},
		}}})).Nil()
	case ratelimit.Policy:
		assert.Error(t, m.ApplyRateLimit(resilience.RateLimitSettings{Services: []resilience.RateLimitService{{
			Service: "svc",
			Rules: []resilience.RateLimitRule{{
				ActionMatch: matcher.ActionMatch{Exact: matcher.ServiceScopeAction},
				Policy:      v,
			}},
		}}})).Nil()
	}
	return m.Wrap("svc", nil)
}

// TestRunGuardRecordNotFoundNeverTripsBreaker is the gorm-specific nuance:
// gorm.ErrRecordNotFound is a normal outcome, not a failure, so no amount of
// misses may open the circuit.
func TestRunGuardRecordNotFoundNeverTripsBreaker(t *testing.T) {
	exec := newExec(t, resilience.BreakerPolicy{ErrorThreshold: 2})
	defer func() { _ = exec.Close() }()

	for range 10 {
		err := runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, nil, func(context.Context) error {
			return gorm.ErrRecordNotFound
		})
		assert.Error(t, err).Is(gorm.ErrRecordNotFound)
	}
	// A subsequent real op still runs (breaker never opened).
	err := runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, nil, func(context.Context) error { return nil })
	assert.Error(t, err).Nil()
}

// fakeTranslator stands in for a dialect's ErrorTranslator: it maps one driver
// error to gorm's constraint sentinel, exactly as mysql/postgres/sqlite do.
type fakeTranslator struct {
	driverErr error
	verdict   error
}

func (f fakeTranslator) Translate(err error) error {
	if errors.Is(err, f.driverErr) {
		return f.verdict
	}
	return err
}

// TestRunGuardConstraintVerdictNeverTripsBreaker proves a constraint rejection
// is classified through the DIALECT's translator and counts as a normal
// outcome — while the caller still receives the driver's own error.
func TestRunGuardConstraintNeverTripsBreaker(t *testing.T) {
	dupKey := errors.New("Error 1062: Duplicate entry")
	exec := newExec(t, resilience.BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour})
	defer func() { _ = exec.Close() }()
	translator := fakeTranslator{driverErr: dupKey, verdict: gorm.ErrDuplicatedKey}

	calls := 0
	call := func() error {
		calls++
		return runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, translator, func(context.Context) error {
			return dupKey
		})
	}
	// The driver error is returned verbatim, not the sentinel.
	assert.Error(t, call()).Is(dupKey)
	assert.Error(t, call()).Is(dupKey)
	assert.Number(t, calls).Equal(2)
}

func TestIsConstraintVerdict(t *testing.T) {
	driverErr := errors.New("constraint")
	for _, verdict := range []error{
		gorm.ErrDuplicatedKey, gorm.ErrForeignKeyViolated, gorm.ErrCheckConstraintViolated,
	} {
		tr := fakeTranslator{driverErr: driverErr, verdict: verdict}
		assert.That(t, isConstraintVerdict(tr, driverErr)).True()
	}
	other := errors.New("connection reset")
	assert.That(t, isConstraintVerdict(fakeTranslator{driverErr: driverErr, verdict: gorm.ErrDuplicatedKey}, other)).False()
}

// TestRunGuardRealErrorsTripBreaker confirms genuine failures open the circuit
// and the rejection is surfaced to the caller.
func TestRunGuardRealErrorsTripBreaker(t *testing.T) {
	exec := newExec(t, resilience.BreakerPolicy{ErrorThreshold: 2})
	defer func() { _ = exec.Close() }()

	boom := errors.New("connection reset")
	assert.Error(t, runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, nil, func(context.Context) error { return boom })).Is(boom)
	assert.Error(t, runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, nil, func(context.Context) error { return boom })).Is(boom)
	// Breaker now open: the next call is short-circuited before the stub runs.
	assert.Error(t, runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, nil, func(context.Context) error { return nil })).Is(chain.ErrCircuitOpen)
}

// TestRunGuardRateLimitRejects confirms the flow-control path. Following
// Guava, the stockpile starts empty and a charge never waits for its own cost:
// the first operation is granted at the head of the pipeline and owes one pace
// of wait, so the next one is refused (MaxWait 0 admits only a free pipeline)
// without invoking the stub.
func TestRunGuardRateLimitRejects(t *testing.T) {
	exec := newExec(t, ratelimit.Policy{Limiter: ratelimit.TokenBucketAlgorithm, RateLimit: 1, Burst: 2})
	defer func() { _ = exec.Close() }()

	var ran int
	stub := func(context.Context) error { ran++; return nil }
	assert.Error(t, runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, nil, stub)).Nil()
	err := runGuard(context.Background(), exec, chain.Operation{Service: "svc"}, nil, stub)
	assert.Error(t, err).Is(chain.ErrRateLimited)
	assert.That(t, ran).Equal(1) // the rejected call never reached the stub
}
