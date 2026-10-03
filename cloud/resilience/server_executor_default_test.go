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

package resilience

import (
	"context"
	"errors"
	"go-spring.org/cloud/chain"
	"testing"
	"time"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

// admit builds the bundled inbound engine for one route and returns it.
func admit(t *testing.T, service string, p ServerPolicy) chain.Executor {
	t.Helper()
	e, err := NewDefaultDriver(nil).NewServerExecutor(service, p)
	assert.Error(t, err).Nil()
	return e
}

// TestServerZeroPolicyIsPassThrough pins the baseline: an unconfigured route
// admits the request untouched, and the handler runs exactly once — inbound has
// no retry stage, so a failing handler is never replayed.
func TestServerZeroPolicyIsPassThrough(t *testing.T) {
	exec := admit(t, "gin::8080", ServerPolicy{})
	ctx := context.Background()

	calls := 0
	boom := errutil.Explain(nil, "handler failed")
	err := exec.Execute(ctx, func(context.Context) error {
		calls++
		return boom
	})
	assert.Error(t, err).Is(boom)
	assert.Number(t, calls).Equal(1) // not replayed

	calls = 0
	assert.Error(t, exec.Execute(ctx, func(context.Context) error { calls++; return nil })).Nil()
	assert.Number(t, calls).Equal(1)
}

// TestServerRejects pins the three rejection outcomes the inbound seam can
// produce before a handler runs.
func TestServerRejects(t *testing.T) {
	ctx := context.Background()
	ok := func(context.Context) error { return nil }

	t.Run("rate limited", func(t *testing.T) {
		exec := admit(t, "gin::8080", ServerPolicy{RateLimit: 0.001, Burst: 1})
		assert.Error(t, exec.Execute(ctx, ok)).Nil()
		assert.That(t, errors.Is(exec.Execute(ctx, ok), chain.ErrRateLimited)).True()
	})

	t.Run("bulkhead full", func(t *testing.T) {
		exec := admit(t, "gin::8080", ServerPolicy{MaxConcurrent: 1})
		release := make(chan struct{})
		entered := make(chan struct{})
		go func() {
			_ = exec.Execute(ctx, func(context.Context) error {
				close(entered)
				<-release
				return nil
			})
		}()
		<-entered
		assert.That(t, errors.Is(exec.Execute(ctx, ok), chain.ErrBulkheadFull)).True()
		close(release)
	})

	t.Run("circuit open", func(t *testing.T) {
		exec := admit(t, "gin::8080", ServerPolicy{ErrorThreshold: 1, OpenDuration: time.Hour})
		_ = exec.Execute(ctx, func(context.Context) error { return errutil.Explain(nil, "boom") })
		assert.That(t, errors.Is(exec.Execute(ctx, ok), chain.ErrCircuitOpen)).True()
	})
}

// TestServerHandlingBudget pins that AttemptTimeout bounds the call itself — a
// request has one attempt, so the budget is the whole call's time rather than a
// slice of it — and that it reaches the handler through the context.
func TestServerHandlingBudget(t *testing.T) {
	exec := admit(t, "gin::8080", ServerPolicy{AttemptTimeout: 20 * time.Millisecond})

	var deadline time.Time
	var ok bool
	assert.Error(t, exec.Execute(context.Background(), func(ctx context.Context) error {
		deadline, ok = ctx.Deadline()
		return nil
	})).Nil()
	assert.That(t, ok).True("the handling budget must reach the handler as a deadline")
	assert.That(t, time.Until(deadline) > 0 && time.Until(deadline) <= 20*time.Millisecond).True()
}

// TestServerRejectsNegativeRateLimit pins that a policy the driver cannot honor
// is refused at construction rather than served as an ungoverned route.
func TestServerRejectsNegativeRateLimit(t *testing.T) {
	_, err := NewDefaultDriver(nil).NewServerExecutor("gin::8080", ServerPolicy{RateLimit: -1})
	assert.Error(t, err).NotNil()
}
