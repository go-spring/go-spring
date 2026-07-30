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
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go-spring.org/stdlib/bufutil"
	"go-spring.org/stdlib/httputil"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// tracerName identifies spans emitted by this starter.
const tracerName = "go-spring.org/starter-gin"

// meterName identifies metrics emitted by this starter.
const meterName = "go-spring.org/starter-gin"

// OTel HTTP semantic-convention attribute keys, stable since semconv v1.27.0.
// Hardcoded (rather than importing semconv/v1.27.0) to keep the starter
// schemaless and free of a version coupling, mirroring starter-otel's stance.
const (
	attrHTTPRequestMethod      = "http.request.method"
	attrHTTPRoute              = "http.route"
	attrHTTPResponseStatusCode = "http.response.status_code"
	attrHTTPRequestBodySize    = "http.request.body.size"
	attrHTTPResponseBodySize   = "http.response.body.size"
	attrURLScheme              = "url.scheme"
	attrURLPath                = "url.path"
	attrServerAddress          = "server.address"
	attrServerPort             = "server.port"
	attrClientAddress          = "client.address"
	attrUserAgentOriginal      = "user_agent.original"
	attrNetworkProtocolVersion = "network.protocol.version"
	attrErrorType              = "error.type"
)

// attrHTTPResponseStream is a starter-defined (non-semconv) attribute used to
// tag SSE streams on http.server.request.duration, so they can be filtered out
// of slow-request percentiles: a 30s stream is normal, a 30s request is not.
const attrHTTPResponseStream = "http.response.stream"

// accessLogTag categorizes the structured access records emitted by the
// Observe middleware (registered as the "_app_gin_access" tag).
var accessLogTag = log.RegisterAppTag("gin", "access")

// bodyTee makes an io.ReadCloser that reads from a TeeReader (copying into a
// capture buffer) and closes the original body.
type bodyTee struct {
	io.Reader
	io.Closer
}

// payloadString returns the captured bytes as a loggable string, or a
// "<N bytes>" placeholder when the content type is binary, so raw bytes are
// never dumped into the log.
func payloadString(b []byte, contentType string) string {
	if !isLoggableContentType(contentType) {
		return fmt.Sprintf("<%d bytes>", len(b))
	}
	return string(b)
}

// isLoggableContentType reports whether a body of this content type is safe to
// log as text; binary types are excluded (logged as a placeholder instead).
// SSE responses are logged per-event by sseAccessLog, not as a body here.
func isLoggableContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	switch {
	case strings.HasPrefix(ct, "text/"),
		ct == "application/json", strings.HasSuffix(ct, "+json"),
		ct == "application/xml", strings.HasSuffix(ct, "+xml"),
		ct == "application/x-www-form-urlencoded",
		ct == "application/javascript":
		return true
	}
	return false
}

// observe is the single per-request observability middleware for the built-in
// gin server. It bundles four concerns that share one per-request lifecycle:
//
//   - Recovery: catches handler panics so a single request can't crash the
//     process.
//   - Tracing: starts an OTel server span per request.
//   - Metrics: records request duration and an in-flight gauge.
//   - AccessLog: emits one structured access record per request.
//
// All four are mandatory and always on whenever the built-in middleware set is
// enabled (the default); disabling the set (middleware.enabled=false) opts out
// of all of them at once, leaving recovery to the application. They are unified
// here rather than chained as separate middlewares so that one deferred
// finalize owns every signal's end-of-request work. That fixes two problems
// the separate-middlewares design had:
//
//   - On a handler panic the inner middlewares' post-c.Next code was skipped
//     (the panic unwound past them before the outer Recovery caught it), so
//     the span was never ended, the in-flight gauge leaked +1, and no metric
//     or access log was recorded for the failing request. Here the defer runs
//     on the unwind path: it recovers the panic, stamps a 500, and finalizes
//     every signal correctly.
//   - Each signal measured its own start time and read status independently,
//     so the same request reported slightly different durations across trace,
//     metric, and log. Here one start time and one status feed all three, and
//     the access log also picks up the span's trace_id/span_id for free.
//
// Tracing and metrics ride the OTel globals that starter-otel installs. When
// starter-otel is not imported the global TracerProvider and MeterProvider are
// no-ops, so those two signals cost almost nothing and the span's SpanContext
// stays invalid (no trace_id is stamped on the access log). Recovery and the
// access log always run.
//
// Span attributes, metric instruments, and access-log fields follow the OTel
// HTTP semantic conventions (stable since semconv v1.27.0): the span name is
// "{method} {route}", the metrics are http.server.request.duration and
// http.server.active_requests, and the access log carries the same semconv
// attribute names so the three signals share a vocabulary. 5xx responses and
// panics mark the span errored and set error.type; a panic additionally records
// an exception event. 4xx does neither, per the convention that client errors
// are not server errors.
func observe(cfg AccessLogConfig) gin.HandlerFunc {
	// Build the skip set once at registration; the skip check consults it per
	// request. applyMiddlewares folds the health endpoint path into cfg.SkipPaths
	// so liveness/readiness probes don't flood the backends.
	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}

	// The three signal objects are built once at registration and shared across
	// requests (they hold only per-middleware state - instruments, tracer, config -
	// never per-request data, so they're concurrency-safe).
	tracer := newHTTPTracer()
	metrics := newHTTPMetrics(cfg.Metrics)
	logger := newHTTPAccessLog(cfg)

	return func(c *gin.Context) {
		facts := collectRequestFacts(c)

		// Skip applies to all three signals - a skipped path (e.g. a health probe)
		// emits no span, no metric, no access log, so probes don't flood the
		// backends. It is decided up front (before the span starts and the gauge
		// bumps) so nothing is created for a skipped request. A panic overrides
		// the skip in the defer below - a failure is never silenced, even on a
		// skipped path.
		//
		// An entry matches the concrete request path (e.g. /healthz) OR the
		// matched gin route pattern (facts.route, e.g. /users/:id), so a
		// RESTful route can be skipped without enumerating every concrete id.
		// The concrete check stays first so existing literal-path configs behave
		// exactly as before; the route-pattern check is additive and reuses the
		// same facts.route the span/metric/log key on, so skip lives in the same
		// namespace as the signals it suppresses. For an unmatched/404 request
		// facts.route is "" and only the concrete path is consulted.
		_, skipped := skip[c.Request.URL.Path]
		if !skipped && facts.route != "" {
			_, skipped = skip[facts.route]
		}

		// Entry operations: begin the server span (T) and bump the in-flight gauge
		// (M). Skipped requests do neither - a no-op span keeps the span non-nil (its
		// methods are all no-ops) and the gauge stays unbumped so the defer doesn't
		// have to balance a -1.
		start := time.Now()
		var inflight metric.MeasurementOption
		var span trace.Span = tracenoop.Span{}
		if !skipped {
			var ctx context.Context
			span, ctx = tracer.Begin(c.Request.Context(), c.Request, facts)
			c.Request = c.Request.WithContext(ctx)
			inflight = metrics.Begin(c.Request.Context(), facts)
		}
		reqBody := logger.CaptureBody(c.Request)

		// One deferred finalize owns every signal's end-of-request work, so a
		// handler panic unwinds through all three (this is why Observe is one
		// middleware, not chained): recover stamps a 500, then the trace/metric/
		// log finalizes run in order via the shared signal objects. The per-request
		// values (facts/start/span/inflight/reqBody) are captured from the closure.
		defer func() {
			rec := recover()
			if rec != nil {
				// The handler panicked. Stamp a 500 so the status recorded below
				// matches what the client receives; this middleware owns recovery
				// (gin.Recovery is no longer installed separately). If the handler
				// had already written a status, AbortWithStatus is a no-op on the
				// wire and the original status is what gets recorded.
				c.AbortWithStatus(http.StatusInternalServerError)
			}

			// A panic overrides the skip: a failure is never silenced, even on a
			// skipped path, so the panic is visible in the metric and the access
			// log (the span is a no-op for a skipped path - the panic surfaces via
			// log/metric instead). A clean skipped request emits nothing.
			if skipped && rec == nil {
				return
			}

			ctx := c.Request.Context()

			// Collect the response-side facts from gin (Writer.Size/Header) and the
			// capture layer (SSE count, captured body) into a plain struct, so the
			// signal objects below stay free of any *gin.Context dependency.
			resp := responseFacts{
				status:          c.Writer.Status(),
				respBodySize:    c.Writer.Size(),
				respContentType: c.Writer.Header().Get("Content-Type"),
			}
			cw := getResponseCapture(c)
			if cw != nil {
				resp.sseCount = cw.sseCount
				resp.respBody = cw.capturedBody()
			}
			isError := rec != nil || resp.status >= http.StatusInternalServerError
			wasSSE := cw != nil && cw.wasSSE

			// Every signal's end-of-request work shares the same per-request values
			// (the response facts, the start time, the span, the body buffer, and the
			// error/panic/skip flags). Pack them once into finalizeFacts and hand the
			// single struct to each finalize call, so Tracer.End/Metrics.End/Emit each
			// take one argument instead of restating a long parameter list.
			ff := finalizeFacts{
				ctx:     ctx,
				req:     facts,
				logReq:  logRequestFacts{c.Request.URL.Path, c.Request.URL.RawQuery, c.Request.Header, c.Request.Header.Get("Content-Type"), c.Request.ContentLength},
				resp:    resp,
				mf:      metricFacts{wasSSE, skipped},
				start:   start,
				span:    span,
				isError: isError,
				rec:     rec,
				reqBody: reqBody,
			}
			tracer.End(ff)            // T
			metrics.End(ff, inflight) // M
			logger.Emit(ff)           // L
		}()

		c.Next()
	}
}

// requestFacts bundles the request-side facts Observe derives up front and feeds
// to the trace/metric/log entry operations. Collecting them once keeps each
// operation a focused function of these facts rather than of *gin.Context.
type requestFacts struct {
	method     string
	route      string
	urlScheme  string
	proto      string
	serverAddr string
	serverPort int
	clientAddr string
	userAgent  string
}

// collectRequestFacts gathers the OTel-relevant facts from a request: method,
// route, scheme, protocol version, server address/port, client address, user
// agent. All are known at request entry, so they're collected once and reused by
// the span attributes, the metric dimensions, and the access-log fields.
func collectRequestFacts(c *gin.Context) requestFacts {
	scheme := httputil.Scheme(c.Request)
	addr, port := httputil.ServerAddrPort(c.Request.Host, scheme)
	return requestFacts{
		method:     c.Request.Method,
		route:      c.FullPath(),
		urlScheme:  scheme,
		proto:      httputil.ProtocolVersion(c.Request.Proto),
		serverAddr: addr,
		serverPort: port,
		clientAddr: c.ClientIP(),
		userAgent:  c.Request.UserAgent(),
	}
}

// responseFacts bundles the response-side facts the access log needs at finalize.
// These are collected from the response writer / capture layer at request end
// (status, body size, content type, and - for SSE - the event count and captured
// body). Carrying them as plain values keeps httpAccessLog.Emit free of any
// framework dependency: gin-specific access (Writer.Size, captureWriter) stays in
// the observe closure, which fills this struct before calling Emit.
type responseFacts struct {
	status          int
	respBodySize    int
	respContentType string
	sseCount        int
	respBody        []byte // captured uncompressed response body (nil for SSE)
}

// metricFacts bundles the metrics-only response flags httpMetrics.End needs
// beyond the shared response facts: whether the response was an SSE stream
// (tagged on the duration histogram so streaming responses can be filtered out
// of slow-request percentiles) and whether the request was skipped (no gauge -1,
// because it never did a +1). status and isError are not duplicated here - they
// live on finalizeFacts (resp.status / isError) and are read directly by End.
type metricFacts struct {
	wasSSE  bool
	skipped bool
}

// logRequestFacts bundles the request-side pure-value facts httpAccessLog.Emit
// needs for the access-log record beyond requestFacts: the URL path/query, the
// request headers, content type, and body size. They are all derivable from an
// *http.Request, so the observe closure extracts them once and passes this
// struct to Emit, keeping Emit free of *http.Request and any framework type.
type logRequestFacts struct {
	path        string
	query       string
	headers     http.Header
	contentType string
	bodySize    int64
}

// finalizeFacts bundles everything the three signal objects need at end-of-request.
// The observe closure builds it once (from the request facts, the response facts,
// the start time, the span, and the error/panic/skip flags) and hands the single
// struct to httpTracer.End, httpMetrics.End, and httpAccessLog.Emit. That keeps
// each finalize method a one-argument function of these facts rather than the
// long, partly-overlapping parameter lists they had before (Emit alone took nine).
type finalizeFacts struct {
	ctx     context.Context
	req     requestFacts
	logReq  logRequestFacts
	resp    responseFacts
	mf      metricFacts
	start   time.Time
	span    trace.Span
	isError bool
	rec     any
	reqBody *bufutil.LimitedBuffer
}

// --- trace (T) --------------------------------------------------------------

// httpTracer owns the OTel tracer and performs the per-request span lifecycle:
// start a server span at entry, end it (with status/error/exception) at finalize.
// It holds only the tracer (created once at registration), so it is safe to share
// across requests; per-request state (the span itself) is returned to the caller.
type httpTracer struct{}

func newHTTPTracer() *httpTracer {
	return &httpTracer{}
}

// Begin extracts the incoming trace context (via the global propagator that
// starter-otel installs, or the no-op default), starts an OTel server span named
// "{method} {route}" (or "{method}" when no route matched), sets the request
// semconv attributes up front, and returns the span along with the context that
// carries it. Without starter-otel the span is non-recording with an invalid
// SpanContext, so the access log omits the trace id. The caller stamps the
// returned context onto the request.
//
// Named Begin to match httpMetrics.Begin - both are the entry half of a symmetric
// Begin/End lifecycle (span open->close, gauge +1->-1). The access log's entry
// side is CaptureBody, intentionally not Begin: its lifecycle is asymmetric
// (capture at entry, emit at exit), not a Begin/End pair.
func (t *httpTracer) Begin(ctx context.Context, r *http.Request, f requestFacts) (trace.Span, context.Context) {
	spanName := f.method
	if f.route != "" {
		spanName = f.method + " " + f.route
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, propagation.HeaderCarrier(r.Header))
	ctx, span := otel.Tracer(tracerName).Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindServer),
	)
	// Request attributes, set up front so they appear even when a later
	// middleware (e.g. a CORS preflight) short-circuits.
	reqAttrs := []attribute.KeyValue{
		attribute.String(attrHTTPRequestMethod, f.method),
		attribute.String(attrURLScheme, f.urlScheme),
		attribute.String(attrNetworkProtocolVersion, f.proto),
		attribute.String(attrClientAddress, f.clientAddr),
	}
	if f.route != "" {
		reqAttrs = append(reqAttrs, attribute.String(attrHTTPRoute, f.route))
	}
	if f.serverAddr != "" {
		reqAttrs = append(reqAttrs, attribute.String(attrServerAddress, f.serverAddr))
		if f.serverPort != 0 {
			reqAttrs = append(reqAttrs, attribute.Int(attrServerPort, f.serverPort))
		}
	}
	if f.userAgent != "" {
		reqAttrs = append(reqAttrs, attribute.String(attrUserAgentOriginal, f.userAgent))
	}
	span.SetAttributes(reqAttrs...)
	return span, ctx
}

// End finalizes the server span: the response status code, and on error an ERROR
// status + error.type; a panic additionally records an exception event with stack
// (the OTel convention). SpanContext stays valid after End, so the access log can
// still read the trace id.
func (t *httpTracer) End(ff finalizeFacts) {
	span := ff.span
	span.SetAttributes(attribute.Int(attrHTTPResponseStatusCode, ff.resp.status))
	if ff.isError {
		span.SetStatus(codes.Error, http.StatusText(ff.resp.status))
		span.SetAttributes(attribute.String(attrErrorType, strconv.Itoa(ff.resp.status)))
	}
	if ff.rec != nil {
		err, ok := ff.rec.(error)
		if !ok {
			err = fmt.Errorf("%v", ff.rec)
		}
		span.RecordError(err, trace.WithStackTrace(true))
	}
	span.End()
}

// --- metrics (M) ------------------------------------------------------------

// httpMetrics owns the request-level metric instruments: the request-duration
// histogram (always on) and the in-flight gauge (opt-in; a no-op when off). Built
// once at registration and shared across requests; per-request data (start time,
// status, the inflight attribute set) is passed to its methods.
type httpMetrics struct {
	duration metric.Float64Histogram
	active   metric.Int64UpDownCounter
}

func newHTTPMetrics(cfg MetricsConfig) *httpMetrics {
	meter := otel.Meter(meterName)
	duration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("s"),
		// OTel HTTP semconv recommended buckets (seconds).
		metric.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	// The in-flight gauge is off by default (low-value for short requests;
	// derivable from QPS+latency). When off, don't even create the instrument -
	// a no-op satisfies the calls so the per-request +1/-1 record nowhere at no
	// cost, with no branch at the call sites. Opt in for long-lived connections
	// (SSE), where in-flight count is a capacity essential.
	active := metric.Int64UpDownCounter(metricnoop.Int64UpDownCounter{})
	if cfg.ActiveRequests {
		active, _ = meter.Int64UpDownCounter(
			"http.server.active_requests",
			metric.WithDescription("Number of active HTTP server requests"),
			metric.WithUnit("{request}"),
		)
	}
	return &httpMetrics{duration: duration, active: active}
}

// Begin bumps the in-flight gauge (+1) and returns the attribute set the matching
// End must pass to its -1 so the gauge balances. Per HTTP semconv the gauge
// carries method + scheme + proto (no route, no status - unknown at start).
func (m *httpMetrics) Begin(ctx context.Context, f requestFacts) metric.MeasurementOption {
	inflightAttrs := metric.WithAttributes(
		attribute.String(attrHTTPRequestMethod, f.method),
		attribute.String(attrURLScheme, f.urlScheme),
		attribute.String(attrNetworkProtocolVersion, f.proto),
	)
	m.active.Add(ctx, 1, inflightAttrs)
	return inflightAttrs
}

// End records the request-duration histogram and balances the in-flight gauge
// (-1). The duration carries method + route + status + scheme + proto, plus
// error.type on error and http.response.stream=sse for SSE streams - so a (often
// long) streaming response can be filtered out of slow-request percentiles: a 30s
// stream is normal, a 30s request is not. The gauge -1 is skipped when the
// request was skipped: a skipped request never did the +1 (panic-on-skipped-path
// records the duration but the gauge was never bumped, so it must not be
// decremented). status and isError come straight off finalizeFacts (resp.status /
// isError); only the metrics-only wasSSE/skipped flags are read from mf.
func (m *httpMetrics) End(ff finalizeFacts, inflight metric.MeasurementOption) {
	f := ff.req
	mf := ff.mf
	status := ff.resp.status
	durAttrs := []attribute.KeyValue{
		attribute.String(attrHTTPRequestMethod, f.method),
		attribute.String(attrURLScheme, f.urlScheme),
		attribute.String(attrNetworkProtocolVersion, f.proto),
		attribute.String(attrHTTPResponseStatusCode, strconv.Itoa(status)),
	}
	if f.route != "" {
		durAttrs = append(durAttrs, attribute.String(attrHTTPRoute, f.route))
	}
	if ff.isError {
		durAttrs = append(durAttrs, attribute.String(attrErrorType, strconv.Itoa(status)))
	}
	if mf.wasSSE {
		durAttrs = append(durAttrs, attribute.String(attrHTTPResponseStream, "sse"))
	}
	m.duration.Record(ff.ctx, time.Since(ff.start).Seconds(), metric.WithAttributes(durAttrs...))
	if !mf.skipped {
		m.active.Add(ff.ctx, -1, inflight)
	}
}

// --- logging (L) ------------------------------------------------------------

// httpAccessLog owns the access-log config (payload capture, skip paths) and
// performs the per-request log lifecycle: capture the request body at entry, emit
// the structured access record at finalize. Built once at registration; the
// per-request body buffer is returned to the caller.
type httpAccessLog struct {
	cfg AccessLogConfig
}

func newHTTPAccessLog(cfg AccessLogConfig) *httpAccessLog {
	return &httpAccessLog{cfg: cfg}
}

// CaptureBody wraps the request body in a TeeReader that copies bytes into a
// bounded capture buffer for the access log's req.body field, so the handler
// still reads every byte unchanged. Returns nil when payload capture is off.
// Redaction is the log layer's job; here we only bound the copy.
func (l *httpAccessLog) CaptureBody(r *http.Request) *bufutil.LimitedBuffer {
	if !l.cfg.Payload.Enabled {
		return nil
	}
	origBody := r.Body
	reqBody := bufutil.New(l.cfg.Payload.Limit)
	r.Body = bodyTee{io.TeeReader(origBody, reqBody), origBody}
	return reqBody
}

// Emit emits the single structured access record for the request. A panic is
// never silenced - even on a skipped path - so a failure is always visible; its
// value and stack are appended to the record. All framework-specific data
// (response size, captured body, SSE count) is pre-collected into resp by the
// caller, so Emit itself depends only on standard types and the project log.
func (l *httpAccessLog) Emit(ff finalizeFacts) {
	f := ff.req
	lr := ff.logReq
	resp := ff.resp
	span := ff.span
	isError := ff.isError
	rec := ff.rec
	reqBody := ff.reqBody
	ctx := ff.ctx

	latency := time.Since(ff.start)
	fields := []log.Field{
		log.String(attrHTTPRequestMethod, f.method),
		log.String(attrURLPath, lr.path),
		log.Int(attrHTTPResponseStatusCode, resp.status),
		log.Int(attrHTTPResponseBodySize, resp.respBodySize),
		log.String(attrClientAddress, f.clientAddr),
		log.String(attrURLScheme, f.urlScheme),
		log.String(attrNetworkProtocolVersion, f.proto),
		log.Float("duration_ms", float64(latency.Nanoseconds())/1e6),
	}
	if f.route != "" {
		fields = append(fields, log.String(attrHTTPRoute, f.route))
	}
	if f.serverAddr != "" {
		fields = append(fields, log.String(attrServerAddress, f.serverAddr))
	}
	if lr.bodySize >= 0 {
		fields = append(fields, log.Int(attrHTTPRequestBodySize, int(lr.bodySize)))
	}
	if f.userAgent != "" {
		fields = append(fields, log.String(attrUserAgentOriginal, f.userAgent))
	}
	if rid := RequestIDFromContext(ctx); rid != "" {
		fields = append(fields, log.String("request_id", rid))
	}
	// SpanContext stays valid after End, so the access log picks up the trace id
	// for free - tying the three signals together.
	if sc := span.SpanContext(); sc.IsValid() {
		fields = append(fields,
			log.String("trace_id", sc.TraceID().String()),
			log.String("span_id", sc.SpanID().String()),
		)
	}
	if isError {
		fields = append(fields, log.String(attrErrorType, strconv.Itoa(resp.status)))
	}
	if rec != nil {
		fields = append(fields,
			log.Any("panic", rec),
			log.String("stack", string(debug.Stack())),
		)
	}

	if l.cfg.Payload.Enabled {
		fields = append(fields,
			log.String("req.body", payloadString(reqBody.Bytes(), lr.contentType)),
			log.String("req.query", lr.query),
			log.String("req.headers", httputil.FlattenHeader(lr.headers)),
		)
		if resp.sseCount > 0 {
			fields = append(fields, log.Int("event.count", resp.sseCount))
		} else if resp.respBody != nil {
			fields = append(fields, log.String("resp.body", payloadString(resp.respBody, resp.respContentType)))
		}
	}

	switch {
	case resp.status >= http.StatusInternalServerError:
		log.Error(ctx, accessLogTag, fields...)
	case resp.status >= http.StatusBadRequest:
		log.Warn(ctx, accessLogTag, fields...)
	default:
		log.Info(ctx, accessLogTag, fields...)
	}
}
