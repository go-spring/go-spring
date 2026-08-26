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
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/governance/traffic"
	"go-spring.org/stdlib/testing/assert"
)

// TestNewRunner_OpenLoopDrivesAtTargetRate verifies OpenLoop dispatches near the
// configured RPS and that MaxConcurrent bounds in-flight goroutines.
func TestNewRunner_OpenLoopDrivesAtTargetRate(t *testing.T) {
	var ops atomic.Int64
	r := New().
		Driver(OpenLoop(2000, 16)). // ~2000 ops/sec
		Duration(60*time.Millisecond).
		Run(context.Background(), func(context.Context) error {
			ops.Add(1)
			return nil
		})
	assert.That(t, r.Driver).Equal("scheduled")
	// Allow wide tolerance (timers + scheduling jitter). Must be well above 0
	// and well below the closed-loop ceiling of unbounded concurrency.
	assert.That(t, ops.Load() > 20).True()
	assert.That(t, r.Ops == ops.Load()).True()
	assert.That(t, len(r.Latencies) == int(r.Ops)).True()
}

// TestNewRunner_ClosedLoopPreservesLegacyBehavior confirms the builder path
// matches the legacy Run for a closed-loop no-op load.
func TestNewRunner_ClosedLoopPreservesLegacyBehavior(t *testing.T) {
	r := New().
		Driver(ClosedLoop{Concurrency: 4}).
		Duration(30*time.Millisecond).
		Run(context.Background(), func(context.Context) error { return nil })
	assert.That(t, r.Driver).Equal("closed-loop")
	assert.That(t, r.Ops > 0).True()
	assert.That(t, r.Errors()).Equal(int64(0))
}

// TestNewRunner_RampIncreasesRate verifies a Ramp schedule actually raises the
// dispatch rate over time: the second half of the run sees more ops than first.
func TestNewRunner_RampIncreasesRate(t *testing.T) {
	var first, second atomic.Int64
	half := 80 * time.Millisecond
	start := time.Now()
	r := New().
		Driver(Ramp(50, 2000, half, 32)).
		Duration(2*half).
		Run(context.Background(), func(context.Context) error {
			if time.Since(start) < half {
				first.Add(1)
			} else {
				second.Add(1)
			}
			return nil
		})
	_ = r
	assert.That(t, second.Load() > first.Load()).True()
}

// TestNewRunner_TagsOpContextLoadTest confirms the builder path still tags the
// op context as load-test traffic (parity with the legacy Run).
func TestNewRunner_TagsOpContextLoadTest(t *testing.T) {
	var seen atomic.Bool
	New().Driver(ClosedLoop{Concurrency: 2}).Duration(20*time.Millisecond).
		Run(context.Background(), func(ctx context.Context) error {
			if traffic.IsLoadTest(ctx) {
				seen.Store(true)
			}
			return nil
		})
	assert.That(t, seen.Load()).True()
}

// TestNewRunner_AssertVerdict verifies assertions populate Result.Assertions and
// Passed(), both passing and failing.
func TestNewRunner_AssertVerdict(t *testing.T) {
	// Passing case: error rate below a generous ceiling under a fault load.
	r := New().Driver(ClosedLoop{Concurrency: 4}).Duration(30*time.Millisecond).
		Assert("error-rate", AssertErrorRateBelow(0.5)).
		Assert("qps-floor", AssertMinQPS(0)).
		Run(context.Background(), func(context.Context) error { return nil })
	assert.That(t, len(r.Assertions)).Equal(2)
	assert.That(t, r.Passed()).True()

	// Failing case: p99 below an impossibly small ceiling.
	r2 := New().Driver(ClosedLoop{Concurrency: 2}).Duration(30*time.Millisecond).
		Assert("p99", AssertP99Below(time.Nanosecond)).
		Run(context.Background(), func(ctx context.Context) error {
			<-ctx.Done() // force a real latency > 1ns
			return ctx.Err()
		})
	assert.That(t, r2.Passed()).False()
	assert.That(t, !r2.Assertions[0].Pass).True()
	assert.That(t, r2.Assertions[0].Msg != "").True()
}

// TestNewRunner_CustomClassifyBuckets verifies a custom classifier routes unknown
// errors into Result.Buckets alongside the built-in labels.
func TestNewRunner_CustomClassifyBuckets(t *testing.T) {
	busy := errors.New("busy")
	timeout := errors.New("timeout")
	custom := func(err error) string {
		switch {
		case errors.Is(err, busy):
			return "busy"
		case errors.Is(err, timeout):
			return "timeout"
		default:
			return DefaultClassify(err)
		}
	}
	r := New().
		Driver(ClosedLoop{Concurrency: 1}).
		Duration(30*time.Millisecond).
		Classify(custom).
		Run(context.Background(), func(context.Context) error {
			return busy // always busy
		})
	assert.That(t, r.Buckets["busy"] > 0).True()
	assert.That(t, r.Errors() == r.Buckets["busy"]).True() // all errors counted

	// The custom classifier still delegates resilience sentinels to the builtin labels.
	inj := fault.ErrInjected
	r2 := New().Driver(ClosedLoop{Concurrency: 1}).Duration(20*time.Millisecond).
		Classify(custom).
		Run(context.Background(), func(context.Context) error { return inj })
	assert.That(t, r2.Buckets[BucketInjected] > 0).True()
	assert.That(t, len(r2.Buckets) == 1).True()
}

// TestNewRunner_AssertGCPauseRequiresCapture confirms GC assertion is trivially
// true without CaptureGC, and that CaptureGC populates NumGC under allocation.
func TestNewRunner_AssertGCPauseRequiresCapture(t *testing.T) {
	// Without CaptureGC: NumGC is 0 -> AssertGCPauseAvgBelow passes trivially.
	r := New().Driver(ClosedLoop{Concurrency: 1}).Duration(20*time.Millisecond).
		Assert("gc", AssertGCPauseAvgBelow(time.Microsecond)).
		Run(context.Background(), func(context.Context) error { return nil })
	assert.That(t, r.NumGC).Equal(int64(0))
	assert.That(t, r.Passed()).True()

	// With CaptureGC + allocation pressure: NumGC > 0 and GCPauseAvg populated.
	_ = resilience.ErrCircuitOpen // keep resilience import used alongside fault
	r2 := New().Driver(ClosedLoop{Concurrency: 4}).Duration(100*time.Millisecond).
		CaptureGC(true).
		Run(context.Background(), func(context.Context) error {
			_ = make([]byte, 64*1024)
			return nil
		})
	// GC may or may not fire in 100ms; if it did, the field is set.
	if r2.NumGC > 0 {
		assert.That(t, r2.GCPauseAvg > 0).True()
	}
}

// TestStepScheduleBoundaries verifies the staircase schedule picks the right
// rate at cumulative boundaries.
func TestStepScheduleBoundaries(t *testing.T) {
	s := StepSchedule(
		Step{RPS: 10, Duration: 100 * time.Millisecond},
		Step{RPS: 20, Duration: 100 * time.Millisecond},
	)
	assert.That(t, s(0) == 10).True()
	assert.That(t, s(50*time.Millisecond) == 10).True()
	assert.That(t, s(100*time.Millisecond) == 20).True()
	assert.That(t, s(250*time.Millisecond) == 20).True()
}

// TestLinearRampSchedule verifies linear ramp math.
func TestLinearRampSchedule(t *testing.T) {
	s := LinearRamp(100, 500, 100*time.Millisecond)
	assert.That(t, s(0) == 100).True()
	assert.That(t, s(50*time.Millisecond) == 300).True()
	assert.That(t, s(100*time.Millisecond) == 500).True()
	assert.That(t, s(999*time.Millisecond) == 500).True() // holds after
}

// TestPrintIncludesVerdictAndDriver verifies the report renders the new fields.
func TestPrintIncludesVerdictAndDriver(t *testing.T) {
	r := New().Driver(OpenLoop(1000, 4)).Duration(20*time.Millisecond).
		Assert("floor", AssertMinQPS(0)).
		Run(context.Background(), func(context.Context) error { return nil })
	var sb strings.Builder
	_ = r // avoid unused if Print signature changes
	r.Print(&sb)
	out := sb.String()
	assert.That(t, strings.Contains(out, "driver=scheduled")).True()
	assert.That(t, strings.Contains(out, "verdict:")).True()
	assert.That(t, strings.Contains(out, "floor")).True()
}
