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

// Package loadtest is a load-test harness shared by the go-spring client
// starters' example-load binaries. It drives an [Op] under a pluggable
// scheduling [Driver] for a bounded duration, records per-op latency and a
// classified error breakdown, then optionally runs health [Assert]ions so a run
// can reach a verdict — "the protection stack held and signals were not lost" —
// rather than merely producing numbers.
//
// Three extension seams make the harness customizable without forking it:
//
//   - [Driver] decides WHEN ops fire: closed-loop (N workers, fire-as-fast-as-
//     op-returns), open-loop (fixed arrival rate independent of latency), or a
//     varying schedule (ramp / staircase / arbitrary). Supply your own to model
//     any traffic shape.
//   - [Assert] decides what "held up" means: check QPS floor, error rate, p99,
//     GC pause, or anything you can read off [Result] / the runtime. The
//     [Runner] collects a run's verdict in [Result.Assertions].
//   - [Classify] (a func(error) string) decides how errors map to buckets when
//     the default resilience+fault taxonomy is not enough; custom labels land in
//     [Result.Buckets].
//
// The legacy [Run] entry point stays a one-liner for closed-loop loads; the
// [Runner] builder is the full-featured path. It is example-grade tooling, not
// production code: each example-load main wires one starter-specific [Op] and
// hands it to [Run] or [New]().Driver(...).Run(...).
package loadtest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"time"

	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// Op is one operation a worker fires per dispatch. It returns nil on success.
// The context passed to it is the run's context: tagged as load-test traffic,
// bounded by the run duration, and cancelled when the run stops.
type Op func(ctx context.Context) error

// Config controls a legacy closed-loop [Run].
type Config struct {
	Concurrency int           // number of worker goroutines
	Duration    time.Duration // how long to drive load before stopping
}

// Error-bucket labels produced by [DefaultClassify]. A custom [Classify] may
// return these or any other label; all labels land in [Result.Buckets] and
// [Result.Print] outputs these five in the listed order before custom ones.
const (
	BucketCircuit     = "circuit"
	BucketRateLimited = "rate-limited"
	BucketBulkhead    = "bulkhead"
	BucketInjected    = "fault-injected"
	BucketOther       = "other"
)

// builtinBuckets fixes the display order of the built-in labels in
// [Result.Print] so reports stay diffable regardless of map iteration order.
var builtinBuckets = [...]string{BucketCircuit, BucketRateLimited, BucketBulkhead, BucketInjected, BucketOther}

// Result is the aggregate of a run. Latencies is sorted ascending so
// [Result.Percentile] can index directly.
type Result struct {
	Ops       int64
	Elapsed   time.Duration
	QPS       float64
	Latencies []time.Duration

	// Buckets maps each error label (see the Bucket* constants and any custom
	// [Classify] labels) to its count. [Result.Errors] sums it.
	Buckets map[string]int64

	// Driver names the scheduling [Driver] that produced this result
	// ("closed-loop", "scheduled", or "%T" of a custom driver).
	Driver string

	// GC summary, populated when [Runner.CaptureGC] was enabled; zero otherwise.
	// AssertGCPauseAvgBelow reads GCPauseAvg.
	NumGC      int64
	GCPauseAvg time.Duration

	// Assertions holds the outcome of each [Assert] registered on the runner.
	// [Result.Passed] reports whether every assertion passed.
	Assertions []AssertionResult
}

// Errors reports the total across all error buckets, custom included.
func (r *Result) Errors() int64 {
	var total int64
	for _, v := range r.Buckets {
		total += v
	}
	return total
}

// Percentile returns the p-quantile of the latency sample, or 0 when the run
// produced no samples. p is clamped to [0, 1].
func (r *Result) Percentile(p float64) time.Duration {
	if len(r.Latencies) == 0 {
		return 0
	}
	p = min(max(p, 0), 1)
	return r.Latencies[int(float64(len(r.Latencies)-1)*p)]
}

// Passed reports whether every registered assertion passed (and is true when no
// assertions were registered — a run with no verdict criteria has nothing to
// fail).
func (r *Result) Passed() bool {
	for _, a := range r.Assertions {
		if !a.Pass {
			return false
		}
	}
	return true
}

// DefaultClassify maps an error to a bucket label using the resilience sentinels
// and [fault.IsInjected]. It is the default [Classify]; a custom one may return
// these labels or any other.
func DefaultClassify(err error) string {
	switch {
	case errors.Is(err, resilience.ErrCircuitOpen):
		return BucketCircuit
	case errors.Is(err, resilience.ErrRateLimited):
		return BucketRateLimited
	case errors.Is(err, resilience.ErrBulkheadFull):
		return BucketBulkhead
	case fault.IsInjected(err):
		return BucketInjected
	default:
		return BucketOther
	}
}

// Run drives op as a closed-loop load across cfg.Concurrency workers for
// cfg.Duration, then returns the aggregate [Result]. It is the legacy
// one-liner; for open-loop, ramp, assertions or a custom driver use [New]().
// Each op's context is tagged as load-test traffic (see [traffic.WithLoadTest])
// so downstream clients can recognise the synthetic load.
func Run(ctx context.Context, cfg Config, op Op) *Result {
	return New().
		Driver(ClosedLoop{Concurrency: cfg.Concurrency}).
		Duration(cfg.Duration).
		Run(ctx, op)
}

// Print writes a human-readable report (driver, throughput, latency
// percentiles, error breakdown, GC summary, assertion verdict) to w.
func (r *Result) Print(w io.Writer) {
	errs := r.Errors()
	errPct := 0.0
	if r.Ops > 0 {
		errPct = float64(errs) / float64(r.Ops) * 100
	}
	fmt.Fprintf(w, "================ load report ================\n")
	fmt.Fprintf(w, "driver=%s  ops=%d  qps=%.0f  elapsed=%s  errors=%d (%.2f%%)\n",
		driverLabel(r.Driver), r.Ops, r.QPS, r.Elapsed.Truncate(time.Millisecond), errs, errPct)
	if len(r.Latencies) > 0 {
		fmt.Fprintf(w, "latency  p50=%s  p90=%s  p99=%s  p999=%s  max=%s\n",
			r.Percentile(0.50), r.Percentile(0.90), r.Percentile(0.99),
			r.Percentile(0.999), r.Latencies[len(r.Latencies)-1])
	}
	fmt.Fprintf(w, "errors by class:")
	if len(r.Buckets) == 0 {
		fmt.Fprintf(w, "  none\n")
	} else {
		for _, b := range builtinBuckets {
			fmt.Fprintf(w, "  %s=%d", b, r.Buckets[b])
		}
		// Custom labels follow the built-ins, in a stable (sorted) order so
		// reports stay diffable across runs.
		var custom []string
		for k := range r.Buckets {
			if !slices.Contains(builtinBuckets[:], k) {
				custom = append(custom, k)
			}
		}
		slices.Sort(custom)
		for _, k := range custom {
			fmt.Fprintf(w, "  %s=%d", k, r.Buckets[k])
		}
		fmt.Fprintf(w, "\n")
	}
	if r.NumGC > 0 {
		fmt.Fprintf(w, "gc  cycles=%d  avg-pause=%s\n", r.NumGC, r.GCPauseAvg.Truncate(time.Microsecond))
	}
	if len(r.Assertions) > 0 {
		fmt.Fprintf(w, "verdict: %s\n", verdictLabel(r.Passed()))
		for _, a := range r.Assertions {
			mark := "PASS"
			if !a.Pass {
				mark = "FAIL"
			}
			if a.Msg != "" {
				fmt.Fprintf(w, "  [%s] %s: %s\n", mark, a.Name, a.Msg)
			} else {
				fmt.Fprintf(w, "  [%s] %s\n", mark, a.Name)
			}
		}
	}
	fmt.Fprintf(w, "=============================================\n")
}

func driverLabel(s string) string {
	if s == "" {
		return "?"
	}
	return s
}

func verdictLabel(pass bool) string {
	if pass {
		return "PASSED"
	}
	return "FAILED"
}
