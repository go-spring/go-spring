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

package loadtest

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// TestRun_SuccessCountsOps: a fast no-op op run for a short duration reports
// ops > 0 and zero errors.
func TestRun_SuccessCountsOps(t *testing.T) {
	r := Run(context.Background(), Config{Concurrency: 4, Duration: 30 * time.Millisecond},
		func(context.Context) error { return nil })
	assert.That(t, r.Ops > 0).True()
	assert.That(t, r.Errors()).Equal(int64(0))
	assert.That(t, len(r.Latencies) == int(r.Ops)).True()
}

// TestRun_ClassifiesErrors buckets each error kind into its own class plus other.
func TestRun_ClassifiesErrors(t *testing.T) {
	var i int32
	r := Run(context.Background(), Config{Concurrency: 1, Duration: 50 * time.Millisecond},
		func(context.Context) error {
			n := atomic.AddInt32(&i, 1) % 5
			switch n {
			case 0:
				return resilience.ErrCircuitOpen
			case 1:
				return resilience.ErrRateLimited
			case 2:
				return resilience.ErrBulkheadFull
			case 3:
				return fault.ErrInjected
			default:
				return errors.New("boom")
			}
		})
	assert.That(t, r.Buckets[BucketCircuit] > 0).True()
	assert.That(t, r.Buckets[BucketRateLimited] > 0).True()
	assert.That(t, r.Buckets[BucketBulkhead] > 0).True()
	assert.That(t, r.Buckets[BucketInjected] > 0).True()
	assert.That(t, r.Buckets[BucketOther] > 0).True()
	assert.That(t, r.Errors()).Equal(r.Ops)
}

// TestRun_StopsOnContextCancel: cancelling the context stops the workers even
// before the duration elapses.
func TestRun_StopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	r := Run(ctx, Config{Concurrency: 2, Duration: 10 * time.Second},
		func(context.Context) error { return nil })
	// Must return well under the 10s duration — cancellation took effect.
	assert.That(t, time.Since(start) < 2*time.Second).True()
	assert.That(t, r.Ops > 0).True()
}

// TestPercentile_EmptyAndSorted covers the edge + ordering.
func TestPercentile_EmptyAndSorted(t *testing.T) {
	r := &Result{}
	assert.That(t, r.Percentile(0.99)).Equal(time.Duration(0))
	r.Latencies = []time.Duration{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	assert.That(t, r.Percentile(0.50) >= 5).True() // index 4..5 region
	assert.That(t, r.Percentile(0.99) == 10 || r.Percentile(0.99) == 9).True()
}

// TestRun_RespectsMinConcurrency guards the < 1 normalization.
func TestRun_RespectsMinConcurrency(t *testing.T) {
	r := Run(context.Background(), Config{Concurrency: 0, Duration: 20 * time.Millisecond},
		func(context.Context) error { return nil })
	assert.That(t, r.Ops > 0).True() // ran with at least 1 worker
}

// TestRun_TagsOpContextLoadTest verifies the harness tags the ctx handed to op
// as load-test traffic, so downstream clients can recognise synthetic load.
func TestRun_TagsOpContextLoadTest(t *testing.T) {
	var seen atomic.Bool
	Run(context.Background(), Config{Concurrency: 2, Duration: 20 * time.Millisecond},
		func(ctx context.Context) error {
			if traffic.IsLoadTest(ctx) {
				seen.Store(true)
			}
			return nil
		})
	assert.That(t, seen.Load()).True()
}
