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

package StarterGormMySql

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/spring/resilience"
	"go-spring.org/stdlib/testing/assert"
	"gorm.io/gorm"
)

func newExec(t *testing.T, p resilience.Policy) resilience.Executor {
	d, err := resilience.MustGetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return exec
}

// TestRecordNotFoundNeverTripsBreaker is the gorm-specific nuance the adapter
// must get right: gorm.ErrRecordNotFound is a normal outcome, not a failure, so
// no amount of misses may open the circuit.
func TestRecordNotFoundNeverTripsBreaker(t *testing.T) {
	exec := newExec(t, resilience.Policy{ErrorThreshold: 2})
	defer func() { _ = exec.Close() }()

	for range 10 {
		err := runGuard(context.Background(), exec, "gorm:mysql:test", func() error {
			return gorm.ErrRecordNotFound
		})
		assert.Error(t, err).Is(gorm.ErrRecordNotFound)
	}
	// A subsequent real op still runs (breaker never opened).
	err := runGuard(context.Background(), exec, "gorm:mysql:test", func() error { return nil })
	assert.Error(t, err).Nil()
}

// TestRealErrorsTripBreaker confirms genuine failures still open the circuit
// and the rejection is surfaced to the caller.
func TestRealErrorsTripBreaker(t *testing.T) {
	exec := newExec(t, resilience.Policy{ErrorThreshold: 2})
	defer func() { _ = exec.Close() }()

	boom := errors.New("connection reset")
	err := runGuard(context.Background(), exec, "gorm:mysql:test", func() error { return boom })
	assert.Error(t, err).Is(boom)
	err = runGuard(context.Background(), exec, "gorm:mysql:test", func() error { return boom })
	assert.Error(t, err).Is(boom)

	// Breaker now open: the next call is short-circuited before the stub runs.
	err = runGuard(context.Background(), exec, "gorm:mysql:test", func() error { return nil })
	assert.Error(t, err).Is(resilience.ErrCircuitOpen)
}

// TestRateLimitRejects confirms the flow-control path: once the burst is spent,
// further operations are rejected as rate-limited without invoking the stub.
func TestRateLimitRejects(t *testing.T) {
	exec := newExec(t, resilience.Policy{RateLimit: 1, Burst: 2})
	defer func() { _ = exec.Close() }()

	var ran int
	stub := func() error { ran++; return nil }

	assert.Error(t, runGuard(context.Background(), exec, "gorm:mysql:test", stub)).Nil()
	assert.Error(t, runGuard(context.Background(), exec, "gorm:mysql:test", stub)).Nil()
	err := runGuard(context.Background(), exec, "gorm:mysql:test", stub)
	assert.Error(t, err).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(2) // the rejected call never reached the stub
}
