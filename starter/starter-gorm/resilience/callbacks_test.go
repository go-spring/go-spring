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

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
	"gorm.io/gorm"
)

func newExec(t *testing.T, p resilience.Policy) resilience.Executor {
	t.Helper()
	d := resilience.NewDefaultDriver()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return exec
}

// TestRunGuardRecordNotFoundNeverTripsBreaker is the gorm-specific nuance:
// gorm.ErrRecordNotFound is a normal outcome, not a failure, so no amount of
// misses may open the circuit.
func TestRunGuardRecordNotFoundNeverTripsBreaker(t *testing.T) {
	exec := newExec(t, resilience.Policy{ErrorThreshold: 2})
	defer func() { _ = exec.Close() }()

	for range 10 {
		err := runGuard(context.Background(), exec, "gorm:test", func() error {
			return gorm.ErrRecordNotFound
		})
		assert.Error(t, err).Is(gorm.ErrRecordNotFound)
	}
	// A subsequent real op still runs (breaker never opened).
	err := runGuard(context.Background(), exec, "gorm:test", func() error { return nil })
	assert.Error(t, err).Nil()
}

// TestRunGuardRealErrorsTripBreaker confirms genuine failures open the circuit
// and the rejection is surfaced to the caller.
func TestRunGuardRealErrorsTripBreaker(t *testing.T) {
	exec := newExec(t, resilience.Policy{ErrorThreshold: 2})
	defer func() { _ = exec.Close() }()

	boom := errors.New("connection reset")
	assert.Error(t, runGuard(context.Background(), exec, "gorm:test", func() error { return boom })).Is(boom)
	assert.Error(t, runGuard(context.Background(), exec, "gorm:test", func() error { return boom })).Is(boom)
	// Breaker now open: the next call is short-circuited before the stub runs.
	assert.Error(t, runGuard(context.Background(), exec, "gorm:test", func() error { return nil })).Is(resilience.ErrCircuitOpen)
}

// TestRunGuardRateLimitRejects confirms the flow-control path: once the burst
// is spent, further operations are rejected as rate-limited without invoking
// the stub.
func TestRunGuardRateLimitRejects(t *testing.T) {
	exec := newExec(t, resilience.Policy{RateLimit: 1, Burst: 2})
	defer func() { _ = exec.Close() }()

	var ran int
	stub := func() error { ran++; return nil }
	assert.Error(t, runGuard(context.Background(), exec, "gorm:test", stub)).Nil()
	assert.Error(t, runGuard(context.Background(), exec, "gorm:test", stub)).Nil()
	err := runGuard(context.Background(), exec, "gorm:test", stub)
	assert.Error(t, err).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(2) // the rejected call never reached the stub
}
