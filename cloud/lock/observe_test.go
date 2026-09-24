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

package lock

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go-spring.org/stdlib/errutil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// fakeLock is a held-lock handle the test drives directly. lose() stands in for
// the backend dropping the lease; Unlock closes the same channel, as every real
// backend does. A nil lost channel means "the lease can never be lost", which
// is what a backend without lease loss returns.
type fakeLock struct {
	key       string
	lost      chan struct{}
	lostOnce  sync.Once
	unlockErr error
}

func newFakeLock(key string) *fakeLock {
	return &fakeLock{key: key, lost: make(chan struct{})}
}

func (f *fakeLock) Key() string           { return f.key }
func (f *fakeLock) Token() string         { return "token" }
func (f *fakeLock) Lost() <-chan struct{} { return f.lost }

func (f *fakeLock) lose() {
	if f.lost != nil {
		f.lostOnce.Do(func() { close(f.lost) })
	}
}

func (f *fakeLock) Unlock(ctx context.Context) error {
	// The hold ends either way: a real backend signals the end of the hold on
	// a successful release and on one it rejects as already taken over.
	f.lose()
	return f.unlockErr
}

// fakeLocker scripts Acquire/TryAcquire results so a test can drive the
// success, miss and error paths without a real backend. handle, when set, is
// the held lock the test gets back; otherwise a fresh one is built per call.
type fakeLocker struct {
	tryOK  bool
	err    error
	handle *fakeLock
}

func (f fakeLocker) held(key string) *fakeLock {
	if f.handle != nil {
		return f.handle
	}
	return newFakeLock(key)
}

func (f fakeLocker) Acquire(ctx context.Context, key string, opts ...Option) (Lock, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.held(key), nil
}

func (f fakeLocker) TryAcquire(ctx context.Context, key string, opts ...Option) (Lock, bool, error) {
	if f.err != nil {
		return nil, false, f.err
	}
	if !f.tryOK {
		return nil, false, nil
	}
	return f.held(key), true, nil
}

func (fakeLocker) Close() error { return nil }

// installGlobals wires in-memory OTel providers so a test can assert spans and
// metrics actually fire.
func installGlobals(t *testing.T) (*tracetest.InMemoryExporter, sdkmetric.Reader, func()) {
	t.Helper()
	prevTP := otel.GetTracerProvider()
	prevMP := otel.GetMeterProvider()

	spanExp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExp))
	rdr := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(rdr))

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	return spanExp, rdr, func() {
		otel.SetTracerProvider(prevTP)
		otel.SetMeterProvider(prevMP)
		_ = tp.Shutdown(context.Background())
		_ = mp.Shutdown(context.Background())
	}
}

// attrValue reads one attribute off a span stub or metric datapoint.
func attrValue(attrs []attribute.KeyValue, key attribute.Key) (attribute.Value, bool) {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value, true
		}
	}
	return attribute.Value{}, false
}

// histPoints collects the duration histogram datapoints for the named metric.
func histPoints(t *testing.T, rdr sdkmetric.Reader, name string) []metricdata.HistogramDataPoint[float64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != name {
				continue
			}
			if h, ok := m.Data.(metricdata.Histogram[float64]); ok {
				return h.DataPoints
			}
		}
	}
	return nil
}

// lostCount reads the lock.lost.total counter for one backend system. Counting
// per system keeps the scenarios below independent: they all run under the same
// provider install, so the reader accumulates their records.
//
// It takes no *testing.T on purpose: the callers poll it from assert.Eventually
// / assert.Never, which run the condition on their own goroutine, where
// require would be illegal.
func lostCount(rdr sdkmetric.Reader, system string) int64 {
	var rm metricdata.ResourceMetrics
	if err := rdr.Collect(context.Background(), &rm); err != nil {
		return 0
	}

	var total int64
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "lock.lost.total" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, p := range sum.DataPoints {
				if s, ok := attrValue(p.Attributes.ToSlice(), "system"); ok && s.AsString() == system {
					total += p.Value
				}
			}
		}
	}
	return total
}

// heldValue reads the current lock.held gauge for one backend system.
func heldValue(t *testing.T, rdr sdkmetric.Reader, system string) int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "lock.held" {
				continue
			}
			g, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, p := range g.DataPoints {
				if s, ok := attrValue(p.Attributes.ToSlice(), "system"); ok && s.AsString() == system {
					return p.Value
				}
			}
		}
	}
	return 0
}

// durationStatuses collects the status attribute of every duration datapoint
// recorded for one operation on one backend.
func durationStatuses(t *testing.T, rdr sdkmetric.Reader, system, op string) []string {
	t.Helper()
	var out []string
	for _, p := range histPoints(t, rdr, "lock.operation.duration") {
		attrs := p.Attributes.ToSlice()
		if s, ok := attrValue(attrs, "system"); !ok || s.AsString() != system {
			continue
		}
		if o, ok := attrValue(attrs, "operation"); !ok || o.AsString() != op {
			continue
		}
		status, ok := attrValue(attrs, "status")
		require.True(t, ok)
		out = append(out, status.AsString())
	}
	return out
}

// TestObserve_AcquireSuccess asserts the Acquire path: a client span named
// "acquire" carrying system + key, and a operation.duration
// metric datapoint tagged status=ok.
func TestObserve_AcquireSuccess(t *testing.T) {
	spanExp, rdr, cleanup := installGlobals(t)
	defer cleanup()

	l := Observe(fakeLocker{}, "redis")
	held, err := l.Acquire(context.Background(), "jobs:1")
	require.NoError(t, err)
	assert.Equal(t, "jobs:1", held.Key())

	spans := spanExp.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "acquire", spans[0].Name)
	assert.Equal(t, codes.Unset, spans[0].Status.Code)

	system, ok := attrValue(spans[0].Attributes, "lock.system")
	require.True(t, ok)
	assert.Equal(t, "redis", system.AsString())
	key, ok := attrValue(spans[0].Attributes, "lock.key")
	require.True(t, ok)
	assert.Equal(t, "jobs:1", key.AsString())

	points := histPoints(t, rdr, "lock.operation.duration")
	require.Len(t, points, 1)
	status, ok := attrValue(points[0].Attributes.ToSlice(), "status")
	require.True(t, ok)
	assert.Equal(t, "ok", status.AsString())

	// The held gauge rose with the wrapped handle; the unlock scenario below
	// brings it back down.
	assert.Equal(t, int64(1), heldValue(t, rdr, "redis"))

	// The remaining scenarios run under the same provider install: the OTel
	// global meter caches by name and only ever delegates to the first provider
	// set, so a second installGlobals in this file would see no records.
	testTryAcquireMiss(t, spanExp, rdr)
	testAcquireError(t, spanExp, rdr)
	testUnlockOK(t, spanExp, rdr)
	testUnlockNotHeld(t, spanExp, rdr)
	testLostReported(t, rdr)
	testNilLostChannel(t, rdr)
	testResignedNotLost(t, rdr)
	testOperationTotals(t, rdr)
}

// operationTotals collects lock.operation.total into a "operation.status" ->
// count map.
func operationTotals(t *testing.T, rdr sdkmetric.Reader) map[string]int64 {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, rdr.Collect(context.Background(), &rm))
	out := map[string]int64{}
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "lock.operation.total" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				op, status := "", ""
				for _, kv := range dp.Attributes.ToSlice() {
					if kv.Key == "operation" {
						op = kv.Value.AsString()
					}
					if kv.Key == "status" {
						status = kv.Value.AsString()
					}
				}
				require.NotEmpty(t, status)
				out[op+"."+status] += dp.Value
			}
		}
	}
	return out
}

// testOperationTotals asserts the counter side of the scenarios above: every
// status the file exercises has at least one exclusive datapoint on
// lock.operation.total, so summed over status the counter equals the
// operations executed.
func testOperationTotals(t *testing.T, rdr sdkmetric.Reader) {
	t.Helper()
	got := operationTotals(t, rdr)
	assert.GreaterOrEqual(t, got["acquire.ok"], int64(1))
	assert.GreaterOrEqual(t, got["acquire.error"], int64(1))
	assert.GreaterOrEqual(t, got["try_acquire.missed"], int64(1))
	assert.GreaterOrEqual(t, got["unlock.ok"], int64(1))
	assert.GreaterOrEqual(t, got["unlock.not_held"], int64(1))
}

// TestObserve_TryAcquireMiss asserts the miss path: a span named
// "try_acquire" whose duration datapoint carries acquired=false, so a
// dashboard can separate misses from wins without parsing errors.
func testTryAcquireMiss(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {

	l := Observe(fakeLocker{tryOK: false}, "etcd")
	_, ok, err := l.TryAcquire(context.Background(), "leader")
	require.NoError(t, err)
	assert.False(t, ok)

	// The exporter and reader already hold the acquire scenario above, so
	// assert on the latest span and on the datapoint tagged status=missed.
	spans := spanExp.GetSpans()
	last := spans[len(spans)-1]
	assert.Equal(t, "try_acquire", last.Name)
	assert.Equal(t, codes.Unset, last.Status.Code)

	var missed bool
	for _, p := range histPoints(t, rdr, "lock.operation.duration") {
		if status, ok := attrValue(p.Attributes.ToSlice(), "status"); ok && status.AsString() == "missed" {
			missed = true
		}
	}
	assert.True(t, missed)
}

// testAcquireError asserts the error path: a backend failure must mark the span
// failed and land in the duration metric as status=error. Before this test the
// error path was tagged "ok" — the metric claimed a 100% success rate while the
// span and the log both said otherwise.
func testAcquireError(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {
	backendErr := errutil.Explain(nil, "backend down")

	l := Observe(fakeLocker{err: backendErr}, "consul")
	_, err := l.Acquire(context.Background(), "jobs:2")
	require.ErrorIs(t, err, backendErr)

	spans := spanExp.GetSpans()
	last := spans[len(spans)-1]
	assert.Equal(t, "acquire", last.Name)
	assert.Equal(t, codes.Error, last.Status.Code)

	// "consul" appears only in this scenario, so its datapoints are exactly the
	// records this failed call produced: one, tagged status=error.
	var got int
	for _, p := range histPoints(t, rdr, "lock.operation.duration") {
		attrs := p.Attributes.ToSlice()
		if system, ok := attrValue(attrs, "system"); !ok || system.AsString() != "consul" {
			continue
		}
		got++
		status, ok := attrValue(attrs, "status")
		require.True(t, ok)
		assert.Equal(t, "error", status.AsString())
	}
	assert.Equal(t, 1, got, "the failed acquire must be recorded exactly once")
}

// testUnlockOK asserts that releasing a lock is observed like acquiring it: an
// "unlock" client span and a duration datapoint, so a dashboard can see how
// often releasing fails as well as how long locks are held.
func testUnlockOK(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {
	l := Observe(fakeLocker{}, "k8s")
	held, err := l.Acquire(context.Background(), "jobs:3")
	require.NoError(t, err)
	require.NoError(t, held.Unlock(context.Background()))

	spans := spanExp.GetSpans()
	last := spans[len(spans)-1]
	assert.Equal(t, "unlock", last.Name)
	assert.Equal(t, codes.Unset, last.Status.Code)

	assert.Equal(t, []string{"ok"}, durationStatuses(t, rdr, "k8s", "unlock"))
}

// testUnlockNotHeld asserts the split-brain signal: releasing a lock that was
// taken over while we held it is its own status, neither a success nor a
// generic backend error, because that is the event a distributed lock exists to
// prevent.
func testUnlockNotHeld(t *testing.T, spanExp *tracetest.InMemoryExporter, rdr sdkmetric.Reader) {
	handle := newFakeLock("jobs:4")
	handle.unlockErr = ErrNotHeld

	l := Observe(fakeLocker{handle: handle}, "memory")
	held, err := l.Acquire(context.Background(), "jobs:4")
	require.NoError(t, err)
	require.ErrorIs(t, held.Unlock(context.Background()), ErrNotHeld)

	spans := spanExp.GetSpans()
	last := spans[len(spans)-1]
	assert.Equal(t, "unlock", last.Name)
	assert.Equal(t, codes.Error, last.Status.Code)

	assert.Equal(t, []string{"not_held"}, durationStatuses(t, rdr, "memory", "unlock"))
}

// testLostReported asserts the signal the handle wrapper exists for: when the
// backend drops the lease while the caller still holds the handle, the loss is
// counted rather than passing unnoticed until the work duplicates itself.
func testLostReported(t *testing.T, rdr sdkmetric.Reader) {
	handle := newFakeLock("jobs:5")

	l := Observe(fakeLocker{handle: handle}, "k8s")
	_, err := l.Acquire(context.Background(), "jobs:5")
	require.NoError(t, err)

	handle.lose()

	assert.Eventually(t, func() bool {
		return lostCount(rdr, "k8s") == 1
	}, time.Second, 5*time.Millisecond, "the loss must be counted exactly once")
}

// testNilLostChannel asserts a backend whose lease can never be lost (Lost
// returns nil) behaves after wrapping exactly as it did before: Lost() hands
// back nil rather than a live channel, so a caller selecting on it blocks as it
// always did, and no goroutine is started to wait on a channel that will never
// close.
func testNilLostChannel(t *testing.T, rdr sdkmetric.Reader) {
	handle := &fakeLock{key: "jobs:7"} // no lost channel: nothing can be lost

	l := Observe(fakeLocker{handle: handle}, "memory")
	held, err := l.Acquire(context.Background(), "jobs:7")
	require.NoError(t, err)
	assert.Nil(t, held.Lost(), "the wrapper must not invent a channel the backend never had")

	// The handle is still usable, and its release is still observed.
	require.NoError(t, held.Unlock(context.Background()))
	assert.Contains(t, durationStatuses(t, rdr, "memory", "unlock"), "ok")
}

// testResignedNotLost asserts the counter stays quiet for an ordinary release.
// Every backend closes the lost channel on Unlock too, so counting that close
// would report one "loss" per election term and drown the real ones.
func testResignedNotLost(t *testing.T, rdr sdkmetric.Reader) {
	handle := newFakeLock("jobs:6")

	l := Observe(fakeLocker{handle: handle}, "etcd")
	held, err := l.Acquire(context.Background(), "jobs:6")
	require.NoError(t, err)
	require.NoError(t, held.Unlock(context.Background()))

	// Unlock sets the released flag before it closes the channel, so the
	// reporting goroutine cannot mistake this close for a loss: the condition
	// is false for good, not merely for now.
	assert.Never(t, func() bool {
		return lostCount(rdr, "etcd") != 0
	}, 200*time.Millisecond, 5*time.Millisecond, "a voluntary release is not a loss")
}
