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

package StarterGoRedis

import (
	"context"
	"errors"
	"testing"

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

func newHook(t *testing.T, p resilience.Policy) *resilienceHook {
	d, err := resilience.MustGetDriver("default")
	assert.Error(t, err).Nil()
	exec, err := d.NewExecutor(p)
	assert.Error(t, err).Nil()
	return &resilienceHook{exec: exec, resource: "redis:test"}
}

// call drives one command through the hook with a stubbed next that returns
// nextErr. It returns both the error the caller sees and the error recorded on
// the command.
func call(h *resilienceHook, nextErr error) (seen, onCmd error) {
	cmd := redis.NewStringCmd(context.Background(), "get", "k")
	ph := h.ProcessHook(func(ctx context.Context, c redis.Cmder) error {
		c.SetErr(nextErr)
		return nextErr
	})
	seen = ph(context.Background(), cmd)
	return seen, cmd.Err()
}

// TestRedisNilNeverTripsBreaker is the redis-specific nuance the adapter must
// get right: a cache miss (redis.Nil) is a normal outcome, not a failure, so no
// amount of misses may open the circuit.
func TestRedisNilNeverTripsBreaker(t *testing.T) {
	h := newHook(t, resilience.Policy{ErrorThreshold: 2})
	for range 10 {
		seen, onCmd := call(h, redis.Nil)
		assert.Error(t, seen).Is(redis.Nil)
		assert.Error(t, onCmd).Is(redis.Nil)
	}
	// A subsequent real command still runs (breaker never opened).
	seen, _ := call(h, nil)
	assert.Error(t, seen).Nil()
}

// TestRealErrorsTripBreaker confirms genuine failures still open the circuit and
// the rejection is surfaced both to the caller and onto the command.
func TestRealErrorsTripBreaker(t *testing.T) {
	h := newHook(t, resilience.Policy{ErrorThreshold: 2})
	boom := errors.New("connection reset")

	seen, _ := call(h, boom)
	assert.Error(t, seen).Is(boom)
	seen, _ = call(h, boom)
	assert.Error(t, seen).Is(boom)

	// Breaker now open: the command is short-circuited before next runs.
	seen, onCmd := call(h, nil)
	assert.Error(t, seen).Is(resilience.ErrCircuitOpen)
	assert.Error(t, onCmd).Is(resilience.ErrCircuitOpen)
}

// TestRateLimitRejects confirms the flow-control path: once the burst is spent,
// further commands are rejected as rate-limited without invoking next.
func TestRateLimitRejects(t *testing.T) {
	h := newHook(t, resilience.Policy{RateLimit: 1, Burst: 2})
	var ran int
	stub := h.ProcessHook(func(ctx context.Context, c redis.Cmder) error {
		ran++
		return nil
	})
	cmd := func() redis.Cmder { return redis.NewStringCmd(context.Background(), "ping") }

	assert.Error(t, stub(context.Background(), cmd())).Nil()
	assert.Error(t, stub(context.Background(), cmd())).Nil()
	rejected := cmd()
	assert.Error(t, stub(context.Background(), rejected)).Is(resilience.ErrRateLimited)
	assert.Error(t, rejected.Err()).Is(resilience.ErrRateLimited)
	assert.That(t, ran).Equal(2) // the rejected command never reached next
}
