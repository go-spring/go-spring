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

package StarterGin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// payloadCaptureLimitForTest mirrors the production default (512 KiB) for the
// harness; tests don't assert on it, so any non-zero cap works, but matching the
// real default keeps the harness honest.
const payloadCaptureLimitForTest = 512 * 1024

// --- metric fake ------------------------------------------------------------
//
// captureProvider is a MeterProvider whose meter records Int64Counter adds and
// Int64/Float64Histogram records so tests can assert the SSE event counter and
// per-event size/interval histograms fired. It embeds metricnoop.MeterProvider
// to satisfy metric.MeterProvider's unexported interface method and overrides
// Meter to return the recording meter; every other instrument falls back to a
// no-op via the embedded metricnoop.Meter, so Observe's request-level metrics
// stay inert.

type captureProvider struct {
	metricnoop.MeterProvider
	m *captureMeter
}

func (p *captureProvider) Meter(string, ...metric.MeterOption) metric.Meter { return p.m }

type captureMeter struct {
	metricnoop.Meter
	mu         sync.Mutex
	counters   map[string]*captureCounter
	updowns    map[string]*captureUpDownCounter
	histograms map[string]*histValue
}

func (m *captureMeter) Int64Counter(name string, _ ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.counters == nil {
		m.counters = map[string]*captureCounter{}
	}
	c := &captureCounter{name: name}
	m.counters[name] = c
	return c, nil
}

func (m *captureMeter) get(name string) *captureCounter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.counters[name]
}

func (m *captureMeter) Int64UpDownCounter(name string, _ ...metric.Int64UpDownCounterOption) (metric.Int64UpDownCounter, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.updowns == nil {
		m.updowns = map[string]*captureUpDownCounter{}
	}
	c := &captureUpDownCounter{name: name}
	m.updowns[name] = c
	return c, nil
}

func (m *captureMeter) updown(name string) *captureUpDownCounter {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.updowns[name]
}

// Int64Histogram / Float64Histogram can't share one recorder type: their Record
// methods take different value types (int64 vs float64) and each carries a
// distinct unexported interface marker, so a single type can't satisfy both.
// Two recorder types mirror the no-op they embed; both stash values as float64
// (int sizes are converted at the call site, mirroring a real backend) so a test
// reads them uniformly via values().
func (m *captureMeter) Int64Histogram(name string, _ ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	h := &captureInt64Histogram{}
	h.name = name
	m.storeHist(name, h)
	return h, nil
}

func (m *captureMeter) Float64Histogram(name string, _ ...metric.Float64HistogramOption) (metric.Float64Histogram, error) {
	h := &captureFloat64Histogram{}
	h.name = name
	m.storeHist(name, h)
	return h, nil
}

// histValue holds the values recorded against either histogram kind under a name.
type histValue struct {
	name string
	mu   sync.Mutex
	vals []float64
}

func (h *histValue) values() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]float64, len(h.vals))
	copy(out, h.vals)
	return out
}

type captureInt64Histogram struct {
	metricnoop.Int64Histogram
	histValue
}

func (h *captureInt64Histogram) Record(_ context.Context, v int64, _ ...metric.RecordOption) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vals = append(h.vals, float64(v))
}

type captureFloat64Histogram struct {
	metricnoop.Float64Histogram
	histValue
}

func (h *captureFloat64Histogram) Record(_ context.Context, v float64, _ ...metric.RecordOption) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vals = append(h.vals, v)
}

func (m *captureMeter) storeHist(name string, h interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.histograms == nil {
		m.histograms = map[string]*histValue{}
	}
	// Both recorder types embed histValue; pull the pointer out by type switch
	// so the test can look it up regardless of which kind was created.
	switch x := h.(type) {
	case *captureInt64Histogram:
		m.histograms[name] = &x.histValue
	case *captureFloat64Histogram:
		m.histograms[name] = &x.histValue
	}
}

func (m *captureMeter) histogram(name string) *histValue {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.histograms[name]
}

type captureCounter struct {
	metricnoop.Int64Counter
	name string
	mu   sync.Mutex
	adds int64
}

func (c *captureCounter) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.adds += incr
}

// captureUpDownCounter records the net delta of Add calls so a test can assert
// the in-flight gauge was exercised (and balanced: net 0 after a request). It
// embeds metricnoop.Int64UpDownCounter for the unexported interface marker.
type captureUpDownCounter struct {
	metricnoop.Int64UpDownCounter
	name string
	mu   sync.Mutex
	net  int64
}

func (c *captureUpDownCounter) Add(_ context.Context, incr int64, _ ...metric.AddOption) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.net += incr
}

func (c *captureUpDownCounter) netValue() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.net
}

// captureHistogram records every value recorded against it (plus the count) so
// a test can assert the size/interval distributions were populated per event. It
// satisfies both metric.Int64Histogram and metric.Float64Histogram: the embedded
// metricnoop.Int64Histogram covers the unexported interface marker and Enabled
// (Float64Histogram needs the same Enabled, satisfied by the one promotion), and
// Record is overridden to capture values.
type captureHistogram struct {
	metricnoop.Int64Histogram
	name string
	mu   sync.Mutex
	vals []float64
}

func (h *captureHistogram) Record(_ context.Context, incr float64, _ ...metric.RecordOption) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.vals = append(h.vals, incr)
}

func (h *captureHistogram) values() []float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]float64, len(h.vals))
	copy(out, h.vals)
	return out
}

// --- trace fake -------------------------------------------------------------
//
// captureTracerProvider returns a Tracer whose spans record their name, start
// time, end time, and attributes so tests can assert the SSE per-event child
// span was created with the right interval semantics. It embeds
// tracenoop.TracerProvider to satisfy trace.TracerProvider's unexported method
// and overrides Tracer. Each span it returns has a valid SpanContext (so the
// IsValid() guards pass) and records the lifecycle calls sseLogger.flush and
// Observe make (SetAttributes/End); every other Span method is a no-op via the
// embedded tracenoop.Span.

type captureTracerProvider struct {
	tracenoop.TracerProvider
	t *captureTracer
}

func (p *captureTracerProvider) Tracer(string, ...trace.TracerOption) trace.Tracer { return p.t }

type captureTracer struct {
	tracenoop.Tracer
	mu    sync.Mutex
	spans []*captureSpan
}

func (t *captureTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	cfg := trace.NewSpanStartConfig(opts...)
	s := &captureSpan{name: name, start: cfg.Timestamp()}
	t.mu.Lock()
	t.spans = append(t.spans, s)
	t.mu.Unlock()
	return trace.ContextWithSpan(ctx, s), s
}

// childSpans returns only the child spans created via Start (the SSE per-event
// spans); the request's own server span is never Start-ed through this tracer in
// the unit harness (Observe calls otel.Tracer(tracerName).Start, which resolves
// to this tracer only because runStream installs it globally - so the server
// span is included too; callers filter by name).
func (t *captureTracer) spansByName(want string) []*captureSpan {
	t.mu.Lock()
	defer t.mu.Unlock()
	var out []*captureSpan
	for _, s := range t.spans {
		if s.name == want {
			out = append(out, s)
		}
	}
	return out
}

// validSpanContext is a non-zero, sampled SpanContext so SpanContext().IsValid()
// is true - the condition Observe and sseLogger.flush check before using a span.
var validSpanContext = trace.NewSpanContext(trace.SpanContextConfig{
	TraceID:    trace.TraceID{1},
	SpanID:     trace.SpanID{1},
	TraceFlags: trace.FlagsSampled,
})

type captureSpan struct {
	tracenoop.Span
	mu    sync.Mutex
	name  string
	start time.Time
	end   time.Time
	attrs []attribute.KeyValue
}

func (s *captureSpan) SpanContext() trace.SpanContext { return validSpanContext }
func (s *captureSpan) IsRecording() bool              { return true }

func (s *captureSpan) SetAttributes(kv ...attribute.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.attrs = append(s.attrs, kv...)
}

func (s *captureSpan) End(opts ...trace.SpanEndOption) {
	cfg := trace.NewSpanEndConfig(opts...)
	s.mu.Lock()
	defer s.mu.Unlock()
	if t := cfg.Timestamp(); !t.IsZero() {
		s.end = t
	} else {
		s.end = time.Now()
	}
}

func (s *captureSpan) duration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.end.IsZero() {
		return 0
	}
	return s.end.Sub(s.start)
}

func (s *captureSpan) attrValue(key string) (attribute.Value, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, kv := range s.attrs {
		if string(kv.Key) == key {
			return kv.Value, true
		}
	}
	return attribute.Value{}, false
}

// --- harness ----------------------------------------------------------------

// streamResult bundles the per-stream observability captures a test asserts on:
// the SSE event counter (metrics), the request's server span, and the child
// spans stamped per event (tracing).
type streamResult struct {
	counter       *captureCounter
	activeReqs    *captureUpDownCounter
	eventSize     *histValue
	eventInterval *histValue
	childSpans    []*captureSpan
}

// runStream builds an engine with Observe + responseCapture (the same order
// applyMiddlewares installs them), installs recording meter + tracer fakes, drives
// one GET /stream, and returns the captured counter, active-requests gauge,
// size/interval histograms, and child spans so a test can assert the per-event
// metric, the in-flight toggle, the distribution, and the span.
func runStream(t *testing.T, payloadEnabled, sseDistributions, activeRequests bool, handler gin.HandlerFunc) streamResult {
	t.Helper()

	prevMeter := otel.GetMeterProvider()
	cm := &captureMeter{}
	otel.SetMeterProvider(&captureProvider{m: cm})
	t.Cleanup(func() { otel.SetMeterProvider(prevMeter) })

	ct := &captureTracer{}
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(&captureTracerProvider{t: ct})
	t.Cleanup(func() { otel.SetTracerProvider(prevTracer) })

	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(Observe(AccessLogConfig{
		Payload: PayloadConfig{Enabled: payloadEnabled},
		Metrics: MetricsConfig{SSEDistributions: sseDistributions, ActiveRequests: activeRequests},
	}))
	e.Use(ResponseCapture(payloadEnabled, payloadCaptureLimitForTest, sseDistributions))
	e.GET("/stream", handler)

	req := httptest.NewRequest(http.MethodGet, "/stream", nil)
	e.ServeHTTP(httptest.NewRecorder(), req)

	return streamResult{
		counter:       cm.get("http.server.sse.events"),
		activeReqs:    cm.updown("http.server.active_requests"),
		eventSize:     cm.histogram("http.server.sse.event.size"),
		eventInterval: cm.histogram("http.server.sse.event.interval"),
		childSpans:    ct.spansByName("sse.event"),
	}
}

func writeSSE(c *gin.Context, n int, flush bool) {
	c.Header("Content-Type", "text/event-stream")
	for i := 0; i < n; i++ {
		_, _ = c.Writer.Write([]byte("data: x\n\n"))
		if flush {
			c.Writer.Flush()
		}
	}
}

// --- counter tests ----------------------------------------------------------

// The handler must run exactly once: adding c.Next() to responseCapture must
// not cause the route handler to execute twice.
func TestSSEHandlerRunsOnce(t *testing.T) {
	var runs int
	runStream(t, true, true, false, func(c *gin.Context) {
		runs++
		writeSSE(c, 1, true)
	})
	if runs != 1 {
		t.Fatalf("handler ran %d times, want 1 (c.Next must not double-execute)", runs)
	}
}

func TestSSEEventCounter_CountsFlushedEvents(t *testing.T) {
	r := runStream(t, true, true, false, func(c *gin.Context) { writeSSE(c, 3, true) })
	if r.counter == nil {
		t.Fatal("http.server.sse.events counter not created")
	}
	if r.counter.adds != 3 {
		t.Fatalf("counter adds=%d, want 3 (one per flushed event)", r.counter.adds)
	}
}

// The counter is always on: it must fire even when payload capture is off, so
// SSE event rate survives a production config that disables payload capture.
func TestSSEEventCounter_AlwaysOnWithoutPayloadCapture(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 3, true) })
	if r.counter == nil {
		t.Fatal("http.server.sse.events counter not created")
	}
	if r.counter.adds != 3 {
		t.Fatalf("counter adds=%d, want 3 (always-on even with payload off)", r.counter.adds)
	}
}

// A trailing event written but never flushed must still be counted: the
// responseCapture defer finalizes the stream, which is the whole reason the
// defer (and its c.Next) exists.
func TestSSEEventCounter_TrailingUnflushedCounted(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 1, false) })
	if r.counter == nil {
		t.Fatal("http.server.sse.events counter not created")
	}
	if r.counter.adds != 1 {
		t.Fatalf("counter adds=%d, want 1 (trailing event must be finalized)", r.counter.adds)
	}
}

// Non-SSE responses must not bump the event counter.
func TestSSEEventCounter_NotFiredForNonSSE(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	if r.counter != nil && r.counter.adds != 0 {
		t.Fatalf("counter adds=%d, want 0 for a non-SSE response", r.counter.adds)
	}
}

// --- histogram tests --------------------------------------------------------

// Each flushed event records its (uncompressed) size into
// http.server.sse.event.size, one record per event - the sampling-independent
// source for a "p99 event size" dashboard that the sampled child span can't back.
func TestSSEEventSizeHistogram_OnePerEvent(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 3, true) })
	if r.eventSize == nil {
		t.Fatal("http.server.sse.event.size histogram not created")
	}
	got := r.eventSize.values()
	if len(got) != 3 {
		t.Fatalf("event size records=%d, want 3 (one per event): %v", len(got), got)
	}
	// Each event is "data: x\n\n" = 9 bytes.
	for i, v := range got {
		if v != 9 {
			t.Fatalf("event size[%d]=%v, want 9", i, v)
		}
	}
}

// Each flushed event records the interval since the previous event into
// http.server.sse.event.interval, one record per event. The first event's
// interval spans from stream start (near zero); a later event after a sleep is
// markedly longer - the "event rate / stall" signal, sampling-independent.
func TestSSEEventIntervalHistogram_RecordsInterval(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) {
		writeSSE(c, 1, true)
		time.Sleep(20 * time.Millisecond)
		writeSSE(c, 1, true)
	})
	if r.eventInterval == nil {
		t.Fatal("http.server.sse.event.interval histogram not created")
	}
	got := r.eventInterval.values()
	if len(got) != 2 {
		t.Fatalf("event interval records=%d, want 2", len(got))
	}
	// Second interval (after a 20ms sleep) >> first (from stream start).
	if got[1] <= got[0] {
		t.Fatalf("second interval %v not greater than first %v", got[1], got[0])
	}
}

// The histograms are always on: they record even when payload capture is off,
// so size/interval distributions survive a production config that disables
// payload capture (the whole reason they exist alongside the sampled span).
func TestSSEHistograms_AlwaysOnWithoutPayloadCapture(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 2, true) })
	if len(r.eventSize.values()) != 2 {
		t.Fatalf("event size records=%d, want 2 (always-on)", len(r.eventSize.values()))
	}
	if len(r.eventInterval.values()) != 2 {
		t.Fatalf("event interval records=%d, want 2 (always-on)", len(r.eventInterval.values()))
	}
}

// A trailing event written but never flushed is still recorded into both
// histograms when the responseCapture defer finalizes the stream.
func TestSSEHistograms_TrailingUnflushedRecorded(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 1, false) })
	if len(r.eventSize.values()) != 1 {
		t.Fatalf("event size records=%d, want 1 (trailing finalized)", len(r.eventSize.values()))
	}
	if len(r.eventInterval.values()) != 1 {
		t.Fatalf("event interval records=%d, want 1 (trailing finalized)", len(r.eventInterval.values()))
	}
}

// Non-SSE responses record nothing into the SSE histograms.
func TestSSEHistograms_NotFiredForNonSSE(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	if r.eventSize != nil && len(r.eventSize.values()) != 0 {
		t.Fatalf("event size records=%d, want 0 for non-SSE", len(r.eventSize.values()))
	}
	if r.eventInterval != nil && len(r.eventInterval.values()) != 0 {
		t.Fatalf("event interval records=%d, want 0 for non-SSE", len(r.eventInterval.values()))
	}
}

// With metrics.sseDistributions off, the size/interval histograms are not created
// (the noop fallback records nothing), so no values are captured - while the
// always-on counter still counts events. This is the toggle's contract: turn off
// the diagnostics-tier distributions without blinding the core event counter.
func TestSSEHistograms_DisabledByConfig(t *testing.T) {
	r := runStream(t, false, false, false, func(c *gin.Context) { writeSSE(c, 3, true) })
	// Counter is core, always on regardless of sseDistributions.
	if r.counter == nil {
		t.Fatal("http.server.sse.events counter not created (counter must stay on)")
	}
	if r.counter.adds != 3 {
		t.Fatalf("counter adds=%d, want 3 (counter is independent of the toggle)", r.counter.adds)
	}
	// Distributions are off: no histogram was created, so nothing was recorded.
	if r.eventSize != nil && len(r.eventSize.values()) != 0 {
		t.Fatalf("event size records=%d, want 0 (distributions disabled)", len(r.eventSize.values()))
	}
	if r.eventInterval != nil && len(r.eventInterval.values()) != 0 {
		t.Fatalf("event interval records=%d, want 0 (distributions disabled)", len(r.eventInterval.values()))
	}
}

// --- active-requests tests --------------------------------------------------

// With activeRequests on, the in-flight gauge gets +1 at request entry and -1 at
// finalize, so after a completed request it balances to net 0 - but it was
// exercised (the counter exists and recorded). This is the toggle's on-contract:
// long-lived connections (SSE) opt in to see in-flight count.
func TestActiveRequests_On_BalancesAfterRequest(t *testing.T) {
	r := runStream(t, false, false, true, func(c *gin.Context) { writeSSE(c, 1, true) })
	if r.activeReqs == nil {
		t.Fatal("http.server.active_requests gauge not created (toggle on)")
	}
	// +1 on entry, -1 on finalize -> net 0 for a completed request.
	if net := r.activeReqs.netValue(); net != 0 {
		t.Fatalf("active_requests net=%d, want 0 (+1/-1 balanced after request)", net)
	}
}

// With activeRequests off (the default), the gauge is never created, so no
// up/down counter exists - the +1/-1 calls go to a no-op. This is the
// off-contract: short-request services skip the per-request bookkeeping.
func TestActiveRequests_Off_NotCreated(t *testing.T) {
	r := runStream(t, false, false, false, func(c *gin.Context) { writeSSE(c, 1, true) })
	if r.activeReqs != nil {
		t.Fatalf("active_requests gauge created (%v), want nil (toggle off)", r.activeReqs)
	}
}

// --- child-span tests -------------------------------------------------------

// Each flushed SSE event creates a child span under the request's server span,
// so each event is its own node on the trace rather than an opaque event on a
// single span. One child span per flushed chunk, mirroring the counter.
func TestSSEChildSpan_OnePerFlushedEvent(t *testing.T) {
	r := runStream(t, true, true, false, func(c *gin.Context) { writeSSE(c, 3, true) })
	if got := len(r.childSpans); got != 3 {
		t.Fatalf("child spans=%d, want 3 (one per flushed event)", got)
	}
}

// The child span is always on, like the counter: it is created even when
// payload capture is off, so the event timeline survives a production config
// that disables payload capture.
func TestSSEChildSpan_AlwaysOnWithoutPayloadCapture(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 2, true) })
	if got := len(r.childSpans); got != 2 {
		t.Fatalf("child spans=%d, want 2 (always-on even with payload off)", got)
	}
}

// A trailing event written but never flushed still gets a child span when the
// responseCapture defer finalizes the stream.
func TestSSEChildSpan_TrailingUnflushedStamped(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 1, false) })
	if got := len(r.childSpans); got != 1 {
		t.Fatalf("child spans=%d, want 1 (trailing event finalized)", got)
	}
}

// Non-SSE responses create no sse.event child span.
func TestSSEChildSpan_NotFiredForNonSSE(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	if got := len(r.childSpans); got != 0 {
		t.Fatalf("child spans=%d, want 0 for a non-SSE response", got)
	}
}

// Each child span's duration is the interval since the previous event: the
// first spans from stream start (the sseLogger is constructed at request entry),
// each later one from the previous flush. The chain of timed child spans is the
// event-rate signal on the trace.
func TestSSEChildSpan_DurationIsEventInterval(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) {
		writeSSE(c, 1, true)
		time.Sleep(20 * time.Millisecond)
		writeSSE(c, 1, true)
	})
	if len(r.childSpans) != 2 {
		t.Fatalf("child spans=%d, want 2", len(r.childSpans))
	}
	// Second span (interval after a 20ms sleep) must be markedly longer than the
	// first (interval from stream start, near-instant). Guards against the span
	// duration collapsing to zero or ignoring the backdated start timestamp.
	if r.childSpans[1].duration() <= r.childSpans[0].duration() {
		t.Fatalf("second event span duration %v not greater than first %v (interval semantics)",
			r.childSpans[1].duration(), r.childSpans[0].duration())
	}
}

// Each child span carries event.seq + event.size attributes, so a trace node
// shows which event and how big it was without leaving the trace view.
func TestSSEChildSpan_CarriesSeqAndSize(t *testing.T) {
	r := runStream(t, false, true, false, func(c *gin.Context) { writeSSE(c, 2, true) })
	if len(r.childSpans) != 2 {
		t.Fatalf("child spans=%d, want 2", len(r.childSpans))
	}
	for i, s := range r.childSpans {
		seq, ok := s.attrValue("event.seq")
		if !ok {
			t.Fatalf("span[%d] missing event.seq attr", i)
		}
		if seq.AsInt64() != int64(i+1) {
			t.Fatalf("span[%d] event.seq=%d, want %d", i, seq.AsInt64(), i+1)
		}
		if _, ok := s.attrValue("event.size"); !ok {
			t.Fatalf("span[%d] missing event.size attr", i)
		}
	}
}

// --- skip tests -------------------------------------------------------------

// runSkip builds an engine whose access log skips /healthz, drives one request to
// the given path, and returns the recording meter + tracer so a test can assert
// which signals fired. It mirrors runStream's fake setup but is path-driven
// rather than SSE-driven, to exercise the skip toggle across all three signals.
func runSkip(t *testing.T, path string, handler gin.HandlerFunc) (*captureMeter, *captureTracer) {
	t.Helper()

	prevMeter := otel.GetMeterProvider()
	cm := &captureMeter{}
	otel.SetMeterProvider(&captureProvider{m: cm})
	t.Cleanup(func() { otel.SetMeterProvider(prevMeter) })

	ct := &captureTracer{}
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(&captureTracerProvider{t: ct})
	t.Cleanup(func() { otel.SetTracerProvider(prevTracer) })

	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(Observe(AccessLogConfig{
		SkipPaths: []string{"/healthz"},
		Metrics:   MetricsConfig{ActiveRequests: true},
	}))
	e.Use(ResponseCapture(false, payloadCaptureLimitForTest, false))
	e.GET("/healthz", handler)
	e.GET("/api", handler)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	e.ServeHTTP(httptest.NewRecorder(), req)
	return cm, ct
}

// A skipped path (the health probe) emits no signal: no server span, no
// request-duration record, no active_requests bump. All three are filtered, not
// just the access log.
func TestSkip_FiltersAllThreeSignals(t *testing.T) {
	cm, ct := runSkip(t, "/healthz", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	// T: no server span for the skipped path.
	if got := len(ct.spansByName("GET /healthz")); got != 0 {
		t.Fatalf("server spans=%d, want 0 (skipped path emits no span)", got)
	}
	// M: no duration record, and the gauge net is 0 (no +1, so no -1 either).
	if h := cm.histogram("http.server.request.duration"); h != nil && len(h.values()) != 0 {
		t.Fatalf("duration records=%d, want 0 (skipped path records no metric)", len(h.values()))
	}
	if u := cm.updown("http.server.active_requests"); u != nil && u.netValue() != 0 {
		t.Fatalf("active_requests net=%d, want 0 (skipped path doesn't bump the gauge)", u.netValue())
	}
}

// A non-skipped path (/api) records all three signals - the skip is scoped to
// the configured paths, not a blanket suppression.
func TestSkip_NonSkippedPathRecordsAllSignals(t *testing.T) {
	cm, ct := runSkip(t, "/api", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	if got := len(ct.spansByName("GET /api")); got != 1 {
		t.Fatalf("server spans=%d, want 1 (non-skipped path records a span)", got)
	}
	if h := cm.histogram("http.server.request.duration"); h == nil || len(h.values()) != 1 {
		t.Fatalf("duration records=%d, want 1 (non-skipped path records the metric)", len(h.values()))
	}
}

// runSkipRoute is runSkip for a custom route set: it builds an engine with the
// given skipPaths, registers routes via reg, drives one GET at requestPath, and
// returns the captured meter/tracer so a test can assert which signals fired.
// Used by the RESTful route-pattern skip tests, where the skip entry is a gin
// route pattern (e.g. /users/:id) rather than a concrete path.
func runSkipRoute(t *testing.T, skipPaths []string, reg func(e *gin.Engine), requestPath string) (*captureMeter, *captureTracer) {
	t.Helper()

	prevMeter := otel.GetMeterProvider()
	cm := &captureMeter{}
	otel.SetMeterProvider(&captureProvider{m: cm})
	t.Cleanup(func() { otel.SetMeterProvider(prevMeter) })

	ct := &captureTracer{}
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(&captureTracerProvider{t: ct})
	t.Cleanup(func() { otel.SetTracerProvider(prevTracer) })

	gin.SetMode(gin.TestMode)
	e := gin.New()
	e.Use(Observe(AccessLogConfig{
		SkipPaths: skipPaths,
		Metrics:   MetricsConfig{ActiveRequests: true},
	}))
	e.Use(ResponseCapture(false, payloadCaptureLimitForTest, false))
	reg(e)

	req := httptest.NewRequest(http.MethodGet, requestPath, nil)
	e.ServeHTTP(httptest.NewRecorder(), req)
	return cm, ct
}

// A skip entry given as a gin route pattern (e.g. /users/:id) suppresses every
// signal for the concrete request (/users/123) - the RESTful case the old
// concrete-path-only check silently missed. The span name uses the route pattern
// too, so this also confirms skip lives in the same namespace as the signals it
// suppresses.
func TestSkip_RestfulRoutePattern(t *testing.T) {
	cm, ct := runSkipRoute(t,
		[]string{"/users/:id"},
		func(e *gin.Engine) {
			e.GET("/users/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		},
		"/users/123",
	)

	if got := len(ct.spansByName("GET /users/:id")); got != 0 {
		t.Fatalf("server spans=%d, want 0 (route-pattern skip suppresses the span)", got)
	}
	if h := cm.histogram("http.server.request.duration"); h != nil && len(h.values()) != 0 {
		t.Fatalf("duration records=%d, want 0 (route-pattern skip records no metric)", len(h.values()))
	}
	if u := cm.updown("http.server.active_requests"); u != nil && u.netValue() != 0 {
		t.Fatalf("active_requests net=%d, want 0 (route-pattern skip doesn't bump the gauge)", u.netValue())
	}
}

// A skip entry that doesn't match the route pattern leaves the signals on, so
// the route-pattern match is scoped, not a blanket suppression.
func TestSkip_NonMatchingRoutePatternRecordsSignals(t *testing.T) {
	cm, ct := runSkipRoute(t,
		[]string{"/users/:id"},
		func(e *gin.Engine) {
			e.GET("/orders/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		},
		"/orders/42",
	)

	if got := len(ct.spansByName("GET /orders/:id")); got != 1 {
		t.Fatalf("server spans=%d, want 1 (non-matching route records a span)", got)
	}
	if h := cm.histogram("http.server.request.duration"); h == nil || len(h.values()) != 1 {
		t.Fatalf("duration records=%d, want 1 (non-matching route records the metric)", len(h.values()))
	}
}

// A skip entry given as a concrete path still matches a concrete request even
// when the route is parametrized - the concrete-path check is preserved, so
// existing literal-path configs keep working under the new dual-match rule.
func TestSkip_ConcretePathOnParametrizedRoute(t *testing.T) {
	cm, ct := runSkipRoute(t,
		[]string{"/users/123"},
		func(e *gin.Engine) {
			e.GET("/users/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		},
		"/users/123",
	)

	if got := len(ct.spansByName("GET /users/:id")); got != 0 {
		t.Fatalf("server spans=%d, want 0 (concrete-path skip still suppresses the span)", got)
	}
	if h := cm.histogram("http.server.request.duration"); h != nil && len(h.values()) != 0 {
		t.Fatalf("duration records=%d, want 0 (concrete-path skip records no metric)", len(h.values()))
	}
}

// A panic on a skipped path is not silenced: the metric and access log still
// fire (the span is a no-op for a skipped path, so the panic surfaces via
// log/metric). The gauge stays balanced at 0 (no +1 happened, so no -1 either).
func TestSkip_PanicOnSkippedPathIsNotSilenced(t *testing.T) {
	cm, _ := runSkip(t, "/healthz", func(c *gin.Context) { panic("boom") })

	// M: the duration is recorded (panic overrides the skip) ...
	h := cm.histogram("http.server.request.duration")
	if h == nil || len(h.values()) != 1 {
		t.Fatalf("duration records=%d, want 1 (panic on skipped path is not silenced)", len(h.values()))
	}
	// ... but the gauge is balanced (no +1 -> no -1).
	if u := cm.updown("http.server.active_requests"); u != nil && u.netValue() != 0 {
		t.Fatalf("active_requests net=%d, want 0 (skipped path never bumped the gauge)", u.netValue())
	}
}

// gin routes BEFORE running any middleware: handleHTTPRequest sets c.fullPath
// (the route template) and only then calls c.Next() to start the chain. So
// Observe - even as the outermost middleware - reads a populated FullPath for a
// matched route. This pins that timing invariant, which the skip dual-match and
// the span/metric/log http.route attribute all depend on: a parametrized route
// yields its template (/users/:id), not the concrete path or "".
func TestObserve_FullPathSetBeforeMiddleware(t *testing.T) {
	_, ct := runSkipRoute(t,
		nil,
		func(e *gin.Engine) {
			e.GET("/users/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		},
		"/users/123",
	)
	// span name is "{method} {route}" with route=FullPath() => "GET /users/:id",
	// the template - proof the router ran first and populated fullPath.
	if got := len(ct.spansByName("GET /users/:id")); got != 1 {
		t.Fatalf("spans \"GET /users/:id\"=%d, want 1 (router must populate FullPath before middleware)", got)
	}
}

// For a 404/NoRoute request Observe still runs (gin folds the global middlewares
// into the NoRoute handler chain via combineHandlers), but FullPath() is "" since
// no route matched - and reset() cleared it at request start, so no stale
// fullPath leaks across pooled requests. The skip dual-match must therefore
// consult only the concrete path on a 404, never an empty (or stale) route; this
// test pins that boundary so a regression - in gin or here - is caught.
func TestObserve_404RouteIsEmpty(t *testing.T) {
	cm, ct := runSkipRoute(t,
		nil,
		func(e *gin.Engine) {
			e.GET("/api", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
		},
		"/nonexistent",
	)
	// Observe ran (404 still goes through the global middleware chain): a server
	// span was created, named "GET" alone (method + "" route).
	if got := len(ct.spansByName("GET")); got != 1 {
		t.Fatalf("404 server spans named \"GET\"=%d, want 1 (Observe runs on NoRoute, route empty)", got)
	}
	// No template-named span exists - route was empty, not stale from the pool.
	if got := len(ct.spansByName("GET /nonexistent")) + len(ct.spansByName("GET /api")); got != 0 {
		t.Fatalf("404 produced a template-named span (route leaked?), got %d", got)
	}
	// The 404 still records a duration (it is observed, not skipped - skip is
	// opt-in, and this engine has no skipPaths).
	h := cm.histogram("http.server.request.duration")
	if h == nil {
		t.Fatalf("no duration histogram captured (Observe should run on 404)")
	}
	if got := len(h.values()); got != 1 {
		t.Fatalf("404 duration records=%d, want 1 (404 is observed, not skipped)", got)
	}
}

// --- EngineMiddleware + manual composition ---------------------------------

// applyOuter reproduces NewSimpleGinServer's nullable-single-hook ordering: a
// nil EngineMiddleware is a no-op, a non-nil one runs before ApplyMiddlewares.
// Used by the ordering test so it mirrors the real ctor's body, not a bespoke
// loop.
func applyOuter(e *gin.Engine, outer EngineMiddleware) {
	if outer != nil {
		outer(e)
	}
}

// An EngineMiddleware hook runs BEFORE the built-in chain, so app middleware
// ends up outermost (before RequestID). Here a hook stamps X-Outer; the built-in
// RequestID still stamps X-Request-Id. Both must be present, and X-Outer must
// wrap RequestID (the hook's e.Use ran first). This locks the ordering
// NewSimpleGinServer relies on: the outer hook, then ApplyMiddlewares.
func TestEngineMiddleware_RunsBeforeBuiltins(t *testing.T) {
	prevMeter := otel.GetMeterProvider()
	cm := &captureMeter{}
	otel.SetMeterProvider(&captureProvider{m: cm})
	t.Cleanup(func() { otel.SetMeterProvider(prevMeter) })

	gin.SetMode(gin.TestMode)
	e := gin.New()

	// The hook stamps a header so we can see it wrapped the chain.
	outer := EngineMiddleware(func(e *gin.Engine) {
		e.Use(func(c *gin.Context) { c.Header("X-Outer", "yes") })
	})
	applyOuter(e, outer)
	if err := ApplyMiddlewares(e, Config{
		Middleware: MiddlewareConfig{
			Enabled:   true,
			RequestID: RequestIDConfig{Enabled: true},
		},
	}); err != nil {
		t.Fatalf("ApplyMiddlewares: %v", err)
	}
	e.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Outer"); got != "yes" {
		t.Fatalf("X-Outer=%q, want \"yes\" (outer hook must run before built-ins)", got)
	}
	if rid := rec.Header().Get("X-Request-Id"); rid == "" {
		t.Fatalf("X-Request-Id missing (built-in RequestID must still run after the hook)")
	}
}

// A nil EngineMiddleware (the case when no bean is provided - the "?" nullable
// injection) is a no-op: ApplyMiddlewares still runs and the server still works.
// This locks the nullable-single-hook contract NewSimpleGinServer relies on.
func TestEngineMiddleware_NilIsNoOp(t *testing.T) {
	prevMeter := otel.GetMeterProvider()
	cm := &captureMeter{}
	otel.SetMeterProvider(&captureProvider{m: cm})
	t.Cleanup(func() { otel.SetMeterProvider(prevMeter) })

	gin.SetMode(gin.TestMode)
	e := gin.New()

	// nil hook - mirrors the nullable-injection case (no EngineMiddleware bean).
	applyOuter(e, nil)
	if err := ApplyMiddlewares(e, Config{
		Middleware: MiddlewareConfig{
			Enabled:   true,
			RequestID: RequestIDConfig{Enabled: true},
		},
	}); err != nil {
		t.Fatalf("ApplyMiddlewares: %v", err)
	}
	e.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	e.ServeHTTP(rec, req)

	// No outer header, but the built-in chain still ran (RequestID present).
	if got := rec.Header().Get("X-Outer"); got != "" {
		t.Fatalf("X-Outer=%q, want \"\" (nil hook must not install anything)", got)
	}
	if rid := rec.Header().Get("X-Request-Id"); rid == "" {
		t.Fatalf("X-Request-Id missing (built-ins must still run with a nil outer hook)")
	}
}

// In manual mode (middleware.enabled=false) the starter installs nothing, but an
// application can call ApplyMiddlewares itself from its RouterRegister to place
// the built-ins at a chosen point - here after an app middleware (so the app
// middleware is outermost). The standard signals (request id, span) still fire,
// proving ApplyMiddlewares is usable standalone.
func TestApplyMiddlewares_ManualComposition(t *testing.T) {
	ct := &captureTracer{}
	prevTracer := otel.GetTracerProvider()
	otel.SetTracerProvider(&captureTracerProvider{t: ct})
	t.Cleanup(func() { otel.SetTracerProvider(prevTracer) })

	gin.SetMode(gin.TestMode)
	e := gin.New()

	// App middleware first (outermost), then the built-in set via ApplyMiddlewares.
	e.Use(func(c *gin.Context) { c.Header("X-App", "manual") })
	if err := ApplyMiddlewares(e, Config{
		Middleware: MiddlewareConfig{
			Enabled:   true,
			RequestID: RequestIDConfig{Enabled: true},
		},
	}); err != nil {
		t.Fatalf("ApplyMiddlewares: %v", err)
	}
	e.GET("/m", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/m", nil)
	e.ServeHTTP(rec, req)

	// App middleware (outermost) + built-in RequestID both fired.
	if got := rec.Header().Get("X-App"); got != "manual" {
		t.Fatalf("X-App=%q, want \"manual\" (app middleware must run)", got)
	}
	if rid := rec.Header().Get("X-Request-Id"); rid == "" {
		t.Fatalf("X-Request-Id missing (ApplyMiddlewares must install RequestID in manual mode)")
	}
	// Built-in Observe fired too: a server span was created.
	if got := len(ct.spansByName("GET /m")); got != 1 {
		t.Fatalf("server spans \"GET /m\"=%d, want 1 (ApplyMiddlewares must install Observe)", got)
	}
}
