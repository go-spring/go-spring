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
	"time"

	"go-spring.org/stdlib/errutil"
)

// Assert is a pluggable verdict on a finished run. The runner calls each
// registered Assert after driving completes and the [Result] is populated, then
// records its outcome in [Result.Assertions]. A nil error means pass; a non-nil
// error means fail and its message is preserved. Register one via
// [Runner.Assert]. The ctx is the original run context (not the
// duration-bounded one), safe for follow-up lookups.
//
// Built-in assertions cover the common "held up" checks: [AssertMinQPS],
// [AssertErrorRateBelow], [AssertP99Below], [AssertGCPauseAvgBelow]. Write your
// own to assert anything [Result] or runtime exposes — e.g. "breaker opened
// within N seconds" (Result.Circuit > 0), "the metric registry gained the
// expected sample count", "no goroutine leak".
type Assert func(ctx context.Context, r *Result) error

// AssertionResult is the outcome of one [Assert], stored on [Result].
type AssertionResult struct {
	Name string
	Pass bool
	Msg  string // failure detail; empty on pass
}

// AssertMinQPS passes when the observed QPS is at least min. Use it to guarantee
// the harness actually drove the intended pressure (catches a misconfigured
// driver or an op that returns instantly without doing work).
func AssertMinQPS(min float64) Assert {
	return func(_ context.Context, r *Result) error {
		if r.QPS < min {
			return errutil.Explain(nil, "qps %.0f below floor %.0f", r.QPS, min)
		}
		return nil
	}
}

// AssertErrorRateBelow passes when the error fraction (errors / ops) is under
// max (0..1). With zero ops it passes (nothing failed because nothing ran).
func AssertErrorRateBelow(max float64) Assert {
	return func(_ context.Context, r *Result) error {
		if r.Ops == 0 {
			return nil
		}
		rate := float64(r.Errors()) / float64(r.Ops)
		if rate > max {
			return errutil.Explain(nil, "error rate %.2f%% exceeds ceiling %.2f%%", rate*100, max*100)
		}
		return nil
	}
}

// AssertP99Below passes when the p99 latency is under max. No-samples passes.
func AssertP99Below(max time.Duration) Assert {
	return func(_ context.Context, r *Result) error {
		p := r.Percentile(0.99)
		if p > max {
			return errutil.Explain(nil, "p99 %s exceeds ceiling %s", p, max)
		}
		return nil
	}
}

// AssertGCPauseAvgBelow passes when the average per-cycle GC pause over the run
// is under max. Requires [Runner.CaptureGC] to have been enabled, else NumGC is
// zero and the assertion passes trivially (no GC data to judge on).
func AssertGCPauseAvgBelow(max time.Duration) Assert {
	return func(_ context.Context, r *Result) error {
		if r.NumGC == 0 {
			return nil
		}
		if r.GCPauseAvg > max {
			return errutil.Explain(nil, "avg gc pause %s exceeds ceiling %s (over %d cycles)", r.GCPauseAvg, max, r.NumGC)
		}
		return nil
	}
}
