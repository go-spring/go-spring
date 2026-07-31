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
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go-spring.org/stdlib/bufutil"
	"go-spring.org/stdlib/httputil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

// respCaptureKey is the gin-context key under which responseCapture publishes
// its captureWriter, so the Observe middleware can read the captured
// (uncompressed) response body in its end-of-request finalize.
const respCaptureKey = "_gs_gin_resp_capture"

// responseCapture installs the innermost response-writer wrapper. It always runs
// the SSE per-event hook - the http.server.sse.events counter and a child span
// per event under the request's server span (both always on), plus, when payload
// capture is on, one access-log record per flushed chunk - so SSE observability
// does not depend on payload capture; body capture for the access log's
// resp.body is gated by payloadEnabled, so turning payload capture off drops
// only the body-copy cost, not SSE metrics. It must sit inside the gzip
// middleware (and any other response transformer) so it sees the bytes the
// handler writes before they are compressed - the logical response body, not the
// wire bytes. It publishes its captureWriter on the gin context (respCaptureKey)
// for Observe to read in its defer.
//
// Splitting capture out of Observe resolves the gzip conflict: when Observe did
// its own capture its tee sat outside gzip and recorded compressed bytes (so the
// access log's resp.body was garbage under gzip). Now capture is innermost, so
// it records uncompressed bytes whether or not gzip is on. SSE per-event signals
// (counter + child span + log + trailing finalize) live here too, since they are
// coupled to the response-writer wrap (Write/Flush); Observe, which only sees
// the stream at request end, reads the finalized event count from the
// captureWriter.
//
// The SSE event counter rides the OTel global MeterProvider and the per-event
// child span rides the global TracerProvider (both no-ops without starter-otel),
// mirroring Observe's request-level metrics and span. Per-event is observable
// only at the flush hook, so both live here rather than in Observe's
// request-granular finalize - Observe's duration histogram and in-flight gauge
// can't see event rate, the one signal unique to streaming. Each child span's
// duration is the interval since the previous event (the first spans from stream
// start), backdated via WithTimestamp so the trace shows event rate as a chain
// of timed child spans under the server span rather than one opaque span.
//
// Together with Observe (the pre-gzip layer) this forms two observability layers
// bracketing gzip. They share a vocabulary - both emit tracing, metrics, and
// logging against the same OTel HTTP semconv - but split responsibilities along
// the gzip boundary, since each side sees different bytes: Observe owns the
// request lifecycle (one span, request-duration histogram, in-flight gauge, one
// access record) measured outside gzip over the whole request; this layer owns
// the response body and streaming events (body capture, per-event counter,
// per-event child spans, per-event log) measured inside gzip on uncompressed
// bytes. The layers are complementary, not duplicated: request-level signals
// belong only on the outer side, event-level signals only on the inner.
//
// Exported so an application that disables the built-in set (middleware.enabled
// = false) and owns its chain can install the innermost capture wrapper at a
// chosen point - it must sit inside any response transformer (e.g. gzip) so it
// records uncompressed bytes. See ApplyMiddlewares for the full chain.
func ResponseCapture(payloadEnabled bool, payloadLimit int, sseDistributions bool) gin.HandlerFunc {
	tracer := newSSETracer()
	metrics := newSSEMetrics(sseDistributions)
	accessLog := newSSEAccessLog(payloadEnabled, payloadLimit)

	return func(c *gin.Context) {
		// Low-cardinality attributes for the per-event counter - the same
		// vocabulary as Observe's request-level metrics, minus status (always
		// 200 for a streaming SSE response) so the counter breaks down event
		// rate by endpoint, not by a constant.
		eventAttrs := []attribute.KeyValue{
			attribute.String(attrHTTPRequestMethod, c.Request.Method),
			attribute.String(attrURLScheme, httputil.Scheme(c.Request)),
			attribute.String(attrNetworkProtocolVersion, httputil.ProtocolVersion(c.Request.Proto)),
		}
		if route := c.FullPath(); route != "" {
			eventAttrs = append(eventAttrs, attribute.String(attrHTTPRoute, route))
		}

		// sseStream is the per-stream accumulator: it holds only state that
		// changes as events arrive (seq/count/chunkSize/lastEvent/buf), plus a
		// reference to c so the signal objects can read the live request context
		// at flush time. The three signal objects (tracer/metrics/accessLog) are
		// shared across streams and passed into flush, mirroring how Observe's
		// observe closure shares httpTracer/httpMetrics/httpAccessLog.
		stream := &sseStream{
			c:          c,
			eventAttrs: eventAttrs,
			lastEvent:  time.Now(),
			buf:        accessLog.newBuffer(),
		}
		cw := &captureWriter{
			ResponseWriter: c.Writer,
			sse:            stream,
			tracer:         tracer,
			metrics:        metrics,
			accessLog:      accessLog,
		}
		if payloadEnabled {
			cw.capture = bufutil.New(payloadLimit)
		}

		c.Set(respCaptureKey, cw)
		c.Writer = cw

		// Finalize SSE on the unwind (after the handler) so a trailing unflushed
		// event is still counted - and, when payload capture is on, logged.
		// Observe reads the stored count afterwards. c.Next() is required so the
		// defer runs after the handler rather than before it.
		defer func() { cw.sseCount, cw.wasSSE = cw.finalizeSSE() }()
		c.Next()
	}
}

// getResponseCapture returns the captureWriter published by responseCapture, or
// nil when capture is not installed (middleware set disabled).
func getResponseCapture(c *gin.Context) *captureWriter {
	v, ok := c.Get(respCaptureKey)
	if !ok {
		return nil
	}
	cw, _ := v.(*captureWriter)
	return cw
}

// captureWriter wraps gin's ResponseWriter. For a normal response it copies
// writes into a capture buffer (only when payload capture is on) for the access
// log's resp.body; for an SSE response (text/event-stream) it instead drives the
// sseStream at each flush - counting every event and stamping a child span
// (both always on) and, when payload capture is on, logging each chunk in real
// time, so a live stream's events are observable as they are sent rather than
// only on disconnect. sseCount/wasSSE are set by finalizeSSE and read by
// Observe's access log and duration metric.
//
// The three SSE signal objects (tracer/metrics/accessLog) are built once at
// registration by responseCapture and stored here per request, so Flush() and
// finalizeSSE() can dispatch to sseStream.flush without changing their
// parameter-less signatures - Flush must stay gin.ResponseWriter/http.Flusher
// compatible since handlers call c.Writer.Flush() with no args. This mirrors
// how Observe's closure shares httpTracer/httpMetrics/httpAccessLog, adapted to
// the ResponseWriter-override boundary.
type captureWriter struct {
	gin.ResponseWriter
	capture   *bufutil.LimitedBuffer
	sse       *sseStream
	tracer    *sseTracer
	metrics   *sseMetrics
	accessLog *sseAccessLog
	sseCount  int  // events counted for this stream; set by finalizeSSE
	wasSSE    bool // whether this response was SSE; set by finalizeSSE
}

func (w *captureWriter) Write(b []byte) (int, error) {
	if w.sse != nil && isSSEContentType(w.Header().Get("Content-Type")) {
		w.sse.active = true
		w.sse.dirty = true
		w.sse.chunkSize += len(b)
		// Buffer the chunk only when it will be logged; when payload capture is
		// off the per-event signals only need to know bytes arrived (dirty +
		// chunkSize), not their content, so the copy is skipped. The toggle now
		// lives on sseAccessLog (the shared log signal object), not on the
		// per-stream sseStream.
		if w.accessLog.logEnabled {
			w.sse.buf.Write(b)
		}
	} else if w.capture != nil {
		w.capture.Write(b)
	}

	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Flush forwards the flush to the client and, for an SSE response, drives the
// per-event hook (bumps the counter and stamps a child span always; emits a log
// record when payload capture is on) for the events written since the last flush.
// The three signal objects are held on the captureWriter (set at construction),
// so Flush keeps the parameter-less gin.ResponseWriter/http.Flusher signature:
// handlers and gin itself call c.Writer.Flush() with no args.
func (w *captureWriter) Flush() {
	w.ResponseWriter.Flush()

	if w.sse != nil && w.sse.active {
		w.sse.flush(w.tracer, w.metrics, w.accessLog)
	}
}

// finalizeSSE flushes any trailing unflushed SSE bytes and reports how many
// event records were counted and whether this response was SSE. It is called
// from responseCapture's defer so the always-on counter and child span catch a
// final unflushed chunk even when payload capture (and thus per-event logging)
// is off.
func (w *captureWriter) finalizeSSE() (count int, wasSSE bool) {
	if w.sse == nil || !w.sse.active {
		return 0, false
	}
	w.sse.flush(w.tracer, w.metrics, w.accessLog)
	return w.sse.count, true
}

// capturedBody returns the captured uncompressed response bytes (empty for SSE
// responses, whose events are logged separately via finalizeSSE).
func (w *captureWriter) capturedBody() []byte {
	if w.capture == nil {
		return nil
	}
	return w.capture.Bytes()
}

// --- metrics (M) ------------------------------------------------------------

// sseMetrics owns the per-event SSE metric instruments: the event counter (always
// on) and the size/interval distribution histograms (toggleable; no-ops when off).
// Built once at registration and shared across streams; per-event data (chunk
// size, interval) is passed to Record.
type sseMetrics struct {
	counter       metric.Int64Counter
	eventSize     metric.Int64Histogram
	eventInterval metric.Float64Histogram
}

func newSSEMetrics(sseDistributions bool) *sseMetrics {
	meter := otel.Meter(meterName)
	counter, _ := meter.Int64Counter(
		"http.server.sse.events",
		metric.WithDescription("Number of SSE events streamed by the server"),
		metric.WithUnit("{event}"),
	)
	// Per-event distributions. Unlike the per-event child span, these survive
	// trace sampling (metrics are counted every time), so they are the reliable
	// source for "p99 event size" and "event interval / stall" dashboards and
	// alerts - the quantities that only lived on the span before and were
	// therefore unreliable under sampling. Toggleable via metrics.sseDistributions;
	// when off, fall back to no-op histograms so the flush path needs no per-event
	// branch (the records just go nowhere).
	var eventSize metric.Int64Histogram
	var eventInterval metric.Float64Histogram
	if sseDistributions {
		eventSize, _ = meter.Int64Histogram(
			"http.server.sse.event.size",
			metric.WithDescription("Size of each SSE event streamed by the server"),
			metric.WithUnit("By"),
			// Bytes: from a tiny heartbeat to a sizable JSON payload.
			metric.WithExplicitBucketBoundaries(16, 64, 256, 1024, 4096, 16384, 65536, 262144),
		)
		eventInterval, _ = meter.Float64Histogram(
			"http.server.sse.event.interval",
			metric.WithDescription("Interval between consecutive SSE events (seconds)"),
			metric.WithUnit("s"),
			// Seconds: heartbeat (sub-second) to a stalled stream (tens of seconds).
			metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30),
		)
	} else {
		eventSize = metricnoop.Int64Histogram{}
		eventInterval = metricnoop.Float64Histogram{}
	}
	return &sseMetrics{counter: counter, eventSize: eventSize, eventInterval: eventInterval}
}

// Record bumps the event counter and records the per-event size and interval
// distributions. Recorded on every event (sampling-independent), so they are the
// reliable source for p99-event-size / stall-alert dashboards that the sampled
// child span can't back. interval is the span since the previous event - the same
// "event rate" signal the span's duration carries, but aggregated rather than
// per-instance.
func (m *sseMetrics) Record(ef eventFacts) {
	attrs := metric.WithAttributes(ef.attrs...)
	m.counter.Add(ef.ctx, 1, attrs)
	m.eventSize.Record(ef.ctx, int64(ef.chunkSize), attrs)
	m.eventInterval.Record(ef.ctx, ef.interval.Seconds(), attrs)
}

// eventFacts bundles the per-event values the three SSE signal objects share at
// each flush, mirroring Observe's finalizeFacts: the request context (read live
// from the gin.Context so a handler that wraps the context mid-stream is still
// seen), the low-cardinality metric attributes, the event's sequence number and
// chunk size, and the interval since the previous event. sseStream builds it
// once per flush and hands the single struct to sseMetrics.Record / sseTracer.
// Stamp / sseAccessLog.Emit - keeping each a one-argument method rather than
// restating a long parameter list (the same shape the pre-gzip layer settled on).
type eventFacts struct {
	ctx       context.Context
	attrs     []attribute.KeyValue
	seq       int
	chunkSize int
	interval  time.Duration
}

// --- trace (T) --------------------------------------------------------------

// sseTracer owns the OTel tracer and performs the per-event span lifecycle,
// mirroring httpTracer on the post-gzip side. It holds only the tracer (created
// once at registration), so it is safe to share across streams; per-event state
// (seq, size, interval) is passed in via eventFacts.
type sseTracer struct {
	tracer trace.Tracer
}

func newSSETracer() *sseTracer {
	return &sseTracer{tracer: otel.Tracer(tracerName)}
}

// Stamp opens an sse.event child span under the request's server span (started
// by Observe, still active while the handler streams) so each event is its own
// node on the trace. Its duration is the interval since the previous event (the
// first spans from stream start), backdated via WithTimestamp - the natural
// "event rate" signal for streaming. A no-op, like the metrics, when
// starter-otel is absent: the noop tracer returns a non-recording span. The span
// carries seq + size only; the full event text lives in the log record.
func (t *sseTracer) Stamp(ef eventFacts, start, end time.Time) {
	_, span := t.tracer.Start(ef.ctx, "sse.event",
		trace.WithTimestamp(start),
	)
	span.SetAttributes(
		attribute.Int("event.seq", ef.seq),
		attribute.Int("event.size", ef.chunkSize),
	)
	span.End(trace.WithTimestamp(end))
}

// --- logging (L) ------------------------------------------------------------

// sseAccessLog owns the per-event access-log config and performs the per-event
// log lifecycle, mirroring httpAccessLog on the post-gzip side: the access-log
// tag (shared) and the payload-capture toggle (whether per-event records are
// emitted at all). Built once at registration and shared across streams; the
// per-event body buffer is created per stream via newBuffer (one per
// sseStream), since each stream buffers its own chunks.
type sseAccessLog struct {
	tag        *log.Tag
	logEnabled bool
	limit      int
}

func newSSEAccessLog(payloadEnabled bool, payloadLimit int) *sseAccessLog {
	return &sseAccessLog{tag: accessLogTag, logEnabled: payloadEnabled, limit: payloadLimit}
}

// newBuffer returns a per-stream capture buffer sized to the payload limit.
func (l *sseAccessLog) newBuffer() *bufutil.LimitedBuffer {
	return bufutil.New(l.limit)
}

// Emit emits one access-log record for the events written since the last flush,
// in real time, so a live stream is observable as it happens rather than only on
// close. Only when payload capture is on - the per-event metrics and span above
// are always on, but the log carries the full event text and is volume-heavy, so
// it is gated. seq is read from eventFacts; the event text is read from the
// stream's own buffer (one per stream), passed in as body so sseAccessLog holds
// no per-stream state.
func (l *sseAccessLog) Emit(ef eventFacts, body *bufutil.LimitedBuffer) {
	if !l.logEnabled {
		return
	}
	fields := []log.Field{
		log.Int("event.seq", ef.seq),
		log.String("resp.event", body.String()),
	}
	if rid := RequestIDFromContext(ef.ctx); rid != "" {
		fields = append(fields, log.String("request_id", rid))
	}
	if sc := trace.SpanContextFromContext(ef.ctx); sc.IsValid() {
		fields = append(fields,
			log.String("trace_id", sc.TraceID().String()),
			log.String("span_id", sc.SpanID().String()),
		)
	}
	log.Info(ef.ctx, l.tag, fields...)
}

// --- per-stream collector ---------------------------------------------------

// sseStream is the per-stream accumulator for an SSE response, run at each
// flush. It holds only state that changes as events arrive (seq/count/chunkSize/
// lastEvent/buf) plus a reference to c so the signal objects can read the live
// request context at flush time (a handler that wraps the context mid-stream is
// still seen). The three signal objects (sseTracer/sseMetrics/sseAccessLog) are
// shared across streams and passed into flush by captureWriter.Flush /
// finalizeSSE - the same split as Observe, where shared signal objects plus a
// per-request collector own one flush path. It is the renamed, slimmed-down
// successor to sseLogger: tracer/tag/logEnabled/metrics moved off into the
// shared signal objects; only the per-stream accumulator remains.
type sseStream struct {
	c          *gin.Context
	buf        *bufutil.LimitedBuffer
	eventAttrs []attribute.KeyValue
	lastEvent  time.Time // span start for the next event; updated to flush time
	seq        int
	count      int
	chunkSize  int // bytes accumulated since the last flush (span/metric attr)
	active     bool
	dirty      bool // bytes arrived since the last flush (count/span signal)
}

// flush emits the per-event signals (M/T/L) for the bytes written since the last
// flush, then resets the per-event accumulator. A no-op when nothing arrived
// since the last flush (dirty is false). The three signal objects are passed in
// by the caller (captureWriter.Flush / finalizeSSE), which holds them per
// request; sseStream itself stays free of any registration-time state.
func (s *sseStream) flush(tracer *sseTracer, metrics *sseMetrics, accessLog *sseAccessLog) {
	if !s.dirty {
		return
	}
	s.seq++
	s.count++
	now := time.Now()

	ef := eventFacts{
		ctx:       s.c.Request.Context(),
		attrs:     s.eventAttrs,
		seq:       s.seq,
		chunkSize: s.chunkSize,
		interval:  now.Sub(s.lastEvent),
	}
	metrics.Record(ef)                 // M
	tracer.Stamp(ef, s.lastEvent, now) // T
	accessLog.Emit(ef, s.buf)          // L

	s.lastEvent = now
	s.buf.Reset()
	s.chunkSize = 0
	s.dirty = false
}

// isSSEContentType reports whether the content type is text/event-stream.
func isSSEContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct == "text/event-stream"
}
