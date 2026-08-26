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
	"maps"
	"runtime"
	"slices"
	"sync"
	"time"

	"go-spring.org/cloud/governance/traffic"
)

// Classify maps an op's error to a bucket label. The default ([DefaultClassify])
// uses the resilience + fault taxonomy; supply your own via [Runner.Classify]
// to add custom buckets (they land in [Result.Buckets]) — for example to split
// "other" into per-handler or per-status breakdowns. A classifier that returns
// "" (or any label equal to [BucketOther]) is counted under "other".
type Classify func(error) string

// Runner is the full-featured load-test entry: pick a scheduling [Driver], a
// [Duration], any number of [Assert]ions, and optionally a custom [Classify]
// and GC capture, then call [Runner.Run]. The legacy [Run] is a thin wrapper
// around a closed-loop Runner. Build one with [New]; methods chain.
type Runner struct {
	driver   Driver
	duration time.Duration
	classify Classify
	gc       bool
	asserts  []namedAssert
}

type namedAssert struct {
	name string
	fn   Assert
}

// New returns a Runner with sensible defaults: closed-loop driver (1 worker),
// [DefaultClassify], no duration (run until ctx cancels), no asserts.
func New() *Runner {
	return &Runner{
		driver:   ClosedLoop{Concurrency: 1},
		classify: DefaultClassify,
	}
}

// Driver sets the scheduling strategy. See [ClosedLoop], [OpenLoop], [Ramp],
// [Staircase], [Scheduled], or supply a custom [Driver].
func (r *Runner) Driver(d Driver) *Runner {
	r.driver = d
	return r
}

// Duration bounds the run; the runner applies it as a context timeout over the
// caller's ctx. <= 0 means run until the caller's ctx is cancelled.
func (r *Runner) Duration(d time.Duration) *Runner {
	r.duration = d
	return r
}

// Classify sets a custom error-to-bucket mapper, replacing [DefaultClassify].
func (r *Runner) Classify(c Classify) *Runner {
	r.classify = c
	return r
}

// CaptureGC toggles reading runtime.MemStats before/after the run so [Result]
// carries GC cycle count + average pause (for [AssertGCPauseAvgBelow]).
func (r *Runner) CaptureGC(b bool) *Runner {
	r.gc = b
	return r
}

// Assert registers a verdict check run after driving completes. The same Assert
// may be registered under several names; the first failing one still records.
func (r *Runner) Assert(name string, fn Assert) *Runner {
	r.asserts = append(r.asserts, namedAssert{name: name, fn: fn})
	return r
}

// Run drives op under the configured driver/duration, records latency + error
// buckets, captures GC if enabled, runs assertions, and returns the [Result].
// The op's context is tagged as load-test traffic (so downstream clients
// recognise synthetic load) and bounded by Duration.
func (r *Runner) Run(ctx context.Context, op Op) *Result {
	runCtx := ctx
	if r.duration > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, r.duration)
		defer cancel()
	}
	runCtx = traffic.WithLoadTest(runCtx, "loadtest.Run")

	rec := newRecorder(r.classify)

	var pre runtime.MemStats
	if r.gc {
		runtime.ReadMemStats(&pre)
	}

	// invoke wraps op with timing + recording so the Driver only decides timing.
	invoke := func(icx context.Context) error {
		start := time.Now()
		err := op(icx)
		rec.observe(time.Since(start), err)
		return err
	}

	start := time.Now()
	_ = r.driver.Drive(runCtx, invoke)
	elapsed := time.Since(start)

	res := rec.result(elapsed)
	res.Driver = driverName(r.driver)
	if r.gc {
		var post runtime.MemStats
		runtime.ReadMemStats(&post)
		res.NumGC = int64(post.NumGC - pre.NumGC)
		if res.NumGC > 0 {
			res.GCPauseAvg = time.Duration(post.PauseTotalNs-pre.PauseTotalNs) / time.Duration(res.NumGC)
		}
	}

	// Assertions run against the finished Result; the original ctx (not the
	// duration-bounded runCtx) is passed so follow-up lookups aren't cancelled.
	for _, a := range r.asserts {
		err := a.fn(ctx, res)
		ar := AssertionResult{Name: a.name, Pass: err == nil}
		if err != nil {
			ar.Msg = err.Error()
		}
		res.Assertions = append(res.Assertions, ar)
	}
	return res
}

// recorder collects per-op latency and classified error buckets. Both the
// latency slice and the bucket map live under one mutex (the recorder is shared
// across the driver's goroutines); this is example-grade tooling, so a single
// lock instead of per-label atomics is the simpler trade.
type recorder struct {
	classify Classify

	mu        sync.Mutex
	buckets   map[string]int64
	latencies []time.Duration
	ops       int64
}

func newRecorder(classify Classify) *recorder {
	if classify == nil {
		classify = DefaultClassify
	}
	return &recorder{classify: classify, buckets: map[string]int64{}}
}

func (r *recorder) observe(lat time.Duration, err error) {
	r.mu.Lock()
	r.latencies = append(r.latencies, lat)
	r.ops++
	if err != nil {
		label := r.classify(err)
		if label == "" {
			label = BucketOther
		}
		r.buckets[label]++
	}
	r.mu.Unlock()
}

func (r *recorder) result(elapsed time.Duration) *Result {
	r.mu.Lock()
	lats := make([]time.Duration, len(r.latencies))
	copy(lats, r.latencies)
	buckets := make(map[string]int64, len(r.buckets))
	maps.Copy(buckets, r.buckets)
	ops := r.ops
	r.mu.Unlock()
	slices.Sort(lats)

	qps := 0.0
	if elapsed > 0 {
		qps = float64(ops) / elapsed.Seconds()
	}
	return &Result{
		Ops:       ops,
		Elapsed:   elapsed,
		QPS:       qps,
		Latencies: lats,
		Buckets:   buckets,
	}
}
