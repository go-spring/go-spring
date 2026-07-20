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

package StarterNats

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/spring/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// newConnWithPolicy builds a Conn whose guard is wired to a real executor from
// the default resilience driver, but whose embedded *nats.Conn is nil. The
// tests never invoke methods that touch the embedded conn — they drive guard
// directly with a stubbed call — so no live nats server is needed.
func newConnWithPolicy(t *testing.T, p resilience.Policy) *Conn {
	d, err := resilience.MustGetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return &Conn{exec: exec, resource: "nats:test"}
}

// TestGuardPassThrough proves the zero-config opt-in: a Conn with no executor
// attached runs the call inline and returns its result unchanged, matching the
// contract of the redis/http adapters that return the base unchanged when
// resilience is disabled.
func TestGuardPassThrough(t *testing.T) {
	h := &Conn{}
	boom := errors.New("boom")

	assert.Error(t, h.guard(context.Background(), func(context.Context) error { return nil })).Nil()
	assert.Error(t, h.guard(context.Background(), func(context.Context) error { return boom })).Is(boom)
}

// TestGuardRateLimit confirms the flow-control path: once the burst is spent,
// further calls are rejected without invoking the stub.
func TestGuardRateLimit(t *testing.T) {
	h := newConnWithPolicy(t, resilience.Policy{RateLimit: 1, Burst: 2})
	var ran int
	stub := func(context.Context) error {
		ran++
		return nil
	}

	assert.Error(t, h.guard(context.Background(), stub)).Nil()
	assert.Error(t, h.guard(context.Background(), stub)).Nil()
	assert.Error(t, h.guard(context.Background(), stub)).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(2) // the rejected call never reached the stub
}

// TestGuardCircuitOpen confirms genuine failures still open the circuit and
// the rejection short-circuits the next call before the stub runs.
func TestGuardCircuitOpen(t *testing.T) {
	h := newConnWithPolicy(t, resilience.Policy{ErrorThreshold: 2})
	boom := errors.New("connection reset")

	assert.Error(t, h.guard(context.Background(), func(context.Context) error { return boom })).Is(boom)
	assert.Error(t, h.guard(context.Background(), func(context.Context) error { return boom })).Is(boom)

	// Breaker is now open: a call whose stub would succeed is rejected upfront.
	var ran int
	err := h.guard(context.Background(), func(context.Context) error {
		ran++
		return nil
	})
	assert.Error(t, err).Is(resilience.ErrCircuitOpen)
	assert.That(t, ran).Equal(0)
}
