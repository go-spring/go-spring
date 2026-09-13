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
	"testing"
	"time"

	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/testing/assert"
)

// recordingListener captures breaker state transitions for test assertions.
type recordingListener struct {
	events []struct {
		resource string
		from, to BreakerState
	}
}

func (l *recordingListener) OnBreakerStateChange(resource string, from, to BreakerState) {
	l.events = append(l.events, struct {
		resource string
		from, to BreakerState
	}{resource, from, to})
}

// TestBreakerEvents verifies the default driver emits closed→open (trip),
// open→half-open (probe) and half-open→closed (recovery) transitions to a
// listener attached via SetBreakerEventListener.
func TestBreakerEvents(t *testing.T) {
	d := NewDefaultDriver()
	exec, err := d.NewExecutor(Policy{ErrorThreshold: 2, OpenDuration: 5 * time.Millisecond})
	assert.Error(t, err).Nil()
	defer func() { _ = exec.Close() }()

	rec := &recordingListener{}
	exec.(BreakerEventListenerSetter).SetBreakerEventListener(rec)

	boom := errutil.Explain(nil, "boom")
	ctx := context.Background()
	// Two consecutive failures trip the breaker (closed → open).
	_ = exec.Execute(ctx, "svc", func(context.Context) error { return boom })
	_ = exec.Execute(ctx, "svc", func(context.Context) error { return boom })
	assert.That(t, len(rec.events)).Equal(1)
	assert.That(t, rec.events[0].from).Equal(BreakerClosed)
	assert.That(t, rec.events[0].to).Equal(BreakerOpen)

	// The breaker is open: this call is rejected before fn runs, no transition.
	_ = exec.Execute(ctx, "svc", func(context.Context) error { return nil })
	assert.That(t, len(rec.events)).Equal(1)

	// Wait out the cool-down so the next call is admitted as a half-open trial.
	time.Sleep(20 * time.Millisecond)
	// Trial succeeds: open → half-open (probe admitted), then half-open → closed.
	_ = exec.Execute(ctx, "svc", func(context.Context) error { return nil })
	assert.That(t, len(rec.events)).Equal(3)
	assert.That(t, rec.events[1].from).Equal(BreakerOpen)
	assert.That(t, rec.events[1].to).Equal(BreakerHalfOpen)
	assert.That(t, rec.events[2].from).Equal(BreakerHalfOpen)
	assert.That(t, rec.events[2].to).Equal(BreakerClosed)
}

// TestBreakerEventsNoListener confirms the breaker still works (no panic) when
// no listener is attached.
func TestBreakerEventsNoListener(t *testing.T) {
	d := NewDefaultDriver()
	exec, err := d.NewExecutor(Policy{ErrorThreshold: 1, OpenDuration: time.Second})
	assert.Error(t, err).Nil()
	defer func() { _ = exec.Close() }()

	err = exec.Execute(context.Background(), "svc", func(context.Context) error { return errutil.Explain(nil, "boom") })
	assert.Error(t, err).NotNil() // tripped, no panic
}

// TestExecutorRefresh verifies Refresh adopts the new policy and resets
// per-resource state: a breaker tripped under the old (tight) policy is gone
// after Refresh raises the threshold, so calls run again.
func TestExecutorRefresh(t *testing.T) {
	d := NewDefaultDriver()
	exec, err := d.NewExecutor(Policy{ErrorThreshold: 1, OpenDuration: time.Hour})
	assert.Error(t, err).Nil()
	defer func() { _ = exec.Close() }()

	ctx := context.Background()
	boom := errutil.Explain(nil, "boom")
	_ = exec.Execute(ctx, "svc", func(context.Context) error { return boom }) // trips (threshold 1)
	err = exec.Execute(ctx, "svc", func(context.Context) error { return nil })
	assert.Error(t, err).Is(ErrCircuitOpen) // rejected, breaker open

	// Raise the threshold well above the failure count; Refresh resets state.
	assert.Error(t, exec.Refresh(Policy{ErrorThreshold: 100, OpenDuration: time.Hour})).Nil()
	err = exec.Execute(ctx, "svc", func(context.Context) error { return nil })
	assert.Error(t, err).Nil() // fresh breaker under new policy, call runs

	// Reject negative rate limit.
	err = exec.Refresh(Policy{RateLimit: -1})
	assert.Error(t, err).NotNil()
}
