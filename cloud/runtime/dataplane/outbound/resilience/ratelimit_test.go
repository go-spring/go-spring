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

package resilience_test

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/cloud/runtime/dataplane/outbound/chain"

	"go-spring.org/cloud/runtime/dataplane/kernel/ratelimit"
	"go-spring.org/cloud/runtime/dataplane/outbound/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// testClusterLimiter is a minimal store for seam-level tests: it counts units per
// scope and rejects once a scope's count reaches the spec's burst. It exists
// only to observe HOW the executor charges a store, not to test algorithms.
type testClusterLimiter struct {
	spent map[string]int
}

func (c *testClusterLimiter) Allow(_ context.Context, scope string, spec ratelimit.Spec, n int) (bool, error) {
	if c.spent == nil {
		c.spent = map[string]int{}
	}
	if c.spent[scope]+n > spec.Burst {
		return false, nil
	}
	c.spent[scope] += n
	return true, nil
}

// TestDefaultDriverCountsPerExecutor pins the default the rate-limit stage
// follows every other stage's: its state belongs to the executor. Two executors
// built by one driver do NOT share a budget, because each counts in a private
// store — and that is still one budget per service through the manager, which
// builds one executor per label and hands it to every caller of that label.
func TestDefaultDriverCountsPerExecutor(t *testing.T) {
	d := resilience.NewDefaultDriver(nil)
	p := resilience.Policy{RateLimit: 1, Burst: 1}
	ctx := context.Background()
	e1, err := d.NewExecutor("svc", p)
	assert.That(t, err).Nil()
	e2, err := d.NewExecutor("svc", p)
	assert.That(t, err).Nil()
	run := func(e chain.Executor) error {
		return e.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil })
	}
	assert.That(t, run(e1)).Nil() // e1 spends its own budget...
	assert.That(t, run(e2)).Nil() // ...so e2 still has one of its own
	assert.That(t, errors.Is(run(e1), chain.ErrRateLimited)).True()
}

// TestDefaultDriverSharesSuppliedStore is the other half of that default: hand
// the driver a store and every executor it builds counts in that one, charged
// under the service each executor is bound to. This is what puts a single limit
// beyond the process (to every replica) when the store is over a shared
// backend.
func TestDefaultDriverSharesSuppliedStore(t *testing.T) {
	store := &testClusterLimiter{}
	d := resilience.NewDefaultDriver(store)
	p := resilience.Policy{RateLimit: 1, Burst: 1}
	ctx := context.Background()
	e1, err := d.NewExecutor("svc", p)
	assert.That(t, err).Nil()
	e2, err := d.NewExecutor("svc", p)
	assert.That(t, err).Nil()
	assert.That(t, e1.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil })).Nil()
	assert.That(t, errors.Is(e2.Execute(ctx, chain.Operation{}, nil, func(context.Context) error { return nil }), chain.ErrRateLimited)).True()
	// The charge landed under the service name, not some private key.
	assert.That(t, store.spent["svc"]).Equal(1)
}
