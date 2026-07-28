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
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/requestid"
	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
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

// payloadCaptureLimit caps how many bytes of the request body and of the
// response body the access log captures per request. With payload capture on,
// a single access record is therefore bounded to ~1 MiB.
const payloadCaptureLimit = 512 * 1024

// accessLogTag categorizes the structured access records emitted by the
// Observe middleware (registered as the "_app_gin_access" tag).
var accessLogTag = log.RegisterAppTag("gin", "access")

// limitedBuffer is a bytes buffer that silently discards writes past max, so a
// runaway body or response can't exhaust memory. It reports all bytes as
// written so a TeeReader feeding it never blocks or errors.
type limitedBuffer struct {
	buf bytes.Buffer
	max int
}

func (l *limitedBuffer) Write(p []byte) (int, error) {
	if l.buf.Len() < l.max {
		n := l.max - l.buf.Len()
		if n > len(p) {
			n = len(p)
		}
		l.buf.Write(p[:n])
	}
	return len(p), nil
}

// bodyTee makes an io.ReadCloser that reads from a TeeReader (copying into a
// capture buffer) and closes the original body.
type bodyTee struct {
	io.Reader
	io.Closer
}

// teeResponseWriter wraps gin's ResponseWriter to observe response bytes. For a
// normal response it copies writes into a capture buffer for the end-of-request
// access log. For an SSE response (text/event-stream) it instead accumulates
// writes and logs each flushed chunk in real time (see sseLogger), so a live
// stream's events hit the log as they are sent - not all at once on disconnect.
type teeResponseWriter struct {
	gin.ResponseWriter
	capture *limitedBuffer
	sse     *sseLogger
}

func (w *teeResponseWriter) Write(b []byte) (int, error) {
	if w.sse != nil && isSSEContentType(w.Header().Get("Content-Type")) {
		w.sse.active = true
		w.sse.buf.Write(b)
	} else if w.capture != nil {
		w.capture.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *teeResponseWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Flush forwards the flush to the client and, for an SSE response, emits a log
// record for the events written since the last flush.
func (w *teeResponseWriter) Flush() {
	w.ResponseWriter.Flush()
	if w.sse != nil && w.sse.active {
		w.sse.flush()
	}
}

// finalizeSSE flushes any trailing unflushed SSE bytes and reports how many
// event records were logged and whether this response was SSE.
func (w *teeResponseWriter) finalizeSSE() (count int, wasSSE bool) {
	if w.sse == nil || !w.sse.active {
		return 0, false
	}
	w.sse.flush()
	return w.sse.count, true
}

// sseLogger emits one access-log record per flushed SSE chunk, in real time, so
// a streaming response is observable as it happens rather than only on close.
type sseLogger struct {
	c      *gin.Context
	tag    *log.Tag
	buf    limitedBuffer
	seq    int
	count  int
	active bool
}

func (s *sseLogger) flush() {
	if s.buf.buf.Len() == 0 {
		return
	}
	s.seq++
	s.count++
	fields := []log.Field{
		log.Int("event.seq", s.seq),
		log.String("resp.event", s.buf.buf.String()),
	}
	if rid := requestid.Get(s.c); rid != "" {
		fields = append(fields, log.String("request_id", rid))
	}
	if sc := trace.SpanContextFromContext(s.c.Request.Context()); sc.IsValid() {
		fields = append(fields,
			log.String("trace_id", sc.TraceID().String()),
			log.String("span_id", sc.SpanID().String()),
		)
	}
	log.Info(s.c.Request.Context(), s.tag, fields...)
	s.buf.buf.Reset()
}

// isSSEContentType reports whether the content type is text/event-stream.
func isSSEContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	return ct == "text/event-stream"
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
// SSE responses are logged per-event by sseLogger, not as a body here.
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

// flattenHeaders renders a header map as "Key: Value; Key: Value" for the log.
func flattenHeaders(h http.Header) string {
	var b strings.Builder
	for k, vs := range h {
		for _, v := range vs {
			if b.Len() > 0 {
				b.WriteString("; ")
			}
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
		}
	}
	return b.String()
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
	// Build the skip set once at registration; the access log consults it per
	// request. applyMiddlewares folds the health endpoint path into cfg.SkipPaths
	// so liveness/readiness probes don't flood the log.
	skip := make(map[string]struct{}, len(cfg.SkipPaths))
	for _, p := range cfg.SkipPaths {
		skip[p] = struct{}{}
	}

	meter := otel.GetMeterProvider().Meter(meterName)
	requestDuration, _ := meter.Float64Histogram(
		"http.server.request.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("s"),
		// OTel HTTP semconv recommended buckets (seconds).
		metric.WithExplicitBucketBoundaries(
			0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10),
	)
	activeRequests, _ := meter.Int64UpDownCounter(
		"http.server.active_requests",
		metric.WithDescription("Number of active HTTP server requests"),
		metric.WithUnit("{request}"),
	)

	return func(c *gin.Context) {
		// Request-side facts, all known up front.
		method := c.Request.Method
		route := c.FullPath()
		urlScheme := scheme(c.Request)
		proto := httpProtocolVersion(c.Request.Proto)
		serverAddr, serverPort := serverAddrPort(c.Request.Host, urlScheme)
		clientAddr := c.ClientIP()
		userAgent := c.Request.UserAgent()

		// Extract the incoming trace context (via the global propagator that
		// starter-otel installs, or the no-op default) and start a server span.
		// Span name follows the HTTP semconv: "{method} {route}" when a route
		// matched, else "{method}". Without starter-otel this yields a
		// non-recording span whose SpanContext is invalid, so the access log
		// below omits the trace id.
		spanName := method
		if route != "" {
			spanName = method + " " + route
		}
		ctx := otel.GetTextMapPropagator().Extract(
			c.Request.Context(),
			propagation.HeaderCarrier(c.Request.Header),
		)
		ctx, span := otel.Tracer(tracerName).Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
		)
		// Request attributes, set up front so they appear even when a later
		// middleware (e.g. body limit) short-circuits.
		reqAttrs := []attribute.KeyValue{
			attribute.String(attrHTTPRequestMethod, method),
			attribute.String(attrURLScheme, urlScheme),
			attribute.String(attrNetworkProtocolVersion, proto),
			attribute.String(attrClientAddress, clientAddr),
		}
		if route != "" {
			reqAttrs = append(reqAttrs, attribute.String(attrHTTPRoute, route))
		}
		if serverAddr != "" {
			reqAttrs = append(reqAttrs, attribute.String(attrServerAddress, serverAddr))
			if serverPort != 0 {
				reqAttrs = append(reqAttrs, attribute.Int(attrServerPort, serverPort))
			}
		}
		if userAgent != "" {
			reqAttrs = append(reqAttrs, attribute.String(attrUserAgentOriginal, userAgent))
		}
		span.SetAttributes(reqAttrs...)
		c.Request = c.Request.WithContext(ctx)

		// In-flight gauge: per HTTP semconv, method + scheme + proto (no route,
		// no status - those are unknown/irrelevant at start). The -1 in the
		// defer reuses these exact attributes so the gauge balances its +1.
		inflightAttrs := metric.WithAttributes(
			attribute.String(attrHTTPRequestMethod, method),
			attribute.String(attrURLScheme, urlScheme),
			attribute.String(attrNetworkProtocolVersion, proto),
		)
		activeRequests.Add(c.Request.Context(), 1, inflightAttrs)
		start := time.Now()

		// Optionally capture request/response payload for the access log.
		// Redaction is the log layer's job; here we only wrap the body and
		// writer so bytes still flow through unchanged to handler and client.
		var reqBody, respBody *limitedBuffer
		var tw *teeResponseWriter
		if cfg.Payload.Enabled {
			reqBody = &limitedBuffer{max: payloadCaptureLimit}
			respBody = &limitedBuffer{max: payloadCaptureLimit}
			sseLog := &sseLogger{
				c:   c,
				tag: accessLogTag,
				buf: limitedBuffer{max: payloadCaptureLimit},
			}
			tw = &teeResponseWriter{ResponseWriter: c.Writer, capture: respBody, sse: sseLog}
			origBody := c.Request.Body
			c.Request.Body = bodyTee{io.TeeReader(origBody, reqBody), origBody}
			c.Writer = tw
		}

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

			status := c.Writer.Status()
			sc := span.SpanContext()
			isError := rec != nil || status >= http.StatusInternalServerError

			// Finalize the span: status code; on error, ERROR status + error.type
			// and, for a panic, an exception event (the OTel convention).
			span.SetAttributes(attribute.Int(attrHTTPResponseStatusCode, status))
			if isError {
				span.SetStatus(codes.Error, http.StatusText(status))
				span.SetAttributes(attribute.String(attrErrorType, strconv.Itoa(status)))
			}
			if rec != nil {
				err, ok := rec.(error)
				if !ok {
					err = fmt.Errorf("%v", rec)
				}
				span.RecordError(err, trace.WithStackTrace(true))
			}
			span.End()

			// Duration histogram: method + route + status + scheme + proto,
			// plus error.type on error (per HTTP semconv).
			durAttrs := []attribute.KeyValue{
				attribute.String(attrHTTPRequestMethod, method),
				attribute.String(attrURLScheme, urlScheme),
				attribute.String(attrNetworkProtocolVersion, proto),
				attribute.String(attrHTTPResponseStatusCode, strconv.Itoa(status)),
			}
			if route != "" {
				durAttrs = append(durAttrs, attribute.String(attrHTTPRoute, route))
			}
			if isError {
				durAttrs = append(durAttrs, attribute.String(attrErrorType, strconv.Itoa(status)))
			}
			requestDuration.Record(c.Request.Context(), time.Since(start).Seconds(),
				metric.WithAttributes(durAttrs...))
			activeRequests.Add(c.Request.Context(), -1, inflightAttrs)

			// Emit the access log. A panic is never silenced - even on a skipped
			// path - so a failure is always visible; its value and stack are
			// appended to the record.
			path := c.Request.URL.Path
			_, skipped := skip[path]
			if rec != nil {
				skipped = false
			}
			if !skipped {
				latency := time.Since(start)
				fields := []log.Field{
					log.String(attrHTTPRequestMethod, method),
					log.String(attrURLPath, path),
					log.Int(attrHTTPResponseStatusCode, status),
					log.Int(attrHTTPResponseBodySize, c.Writer.Size()),
					log.String(attrClientAddress, clientAddr),
					log.String(attrURLScheme, urlScheme),
					log.String(attrNetworkProtocolVersion, proto),
					log.Float("duration_ms", float64(latency.Nanoseconds())/1e6),
				}
				if route != "" {
					fields = append(fields, log.String(attrHTTPRoute, route))
				}
				if serverAddr != "" {
					fields = append(fields, log.String(attrServerAddress, serverAddr))
				}
				if cl := c.Request.ContentLength; cl >= 0 {
					fields = append(fields, log.Int(attrHTTPRequestBodySize, int(cl)))
				}
				if userAgent != "" {
					fields = append(fields, log.String(attrUserAgentOriginal, userAgent))
				}
				if rid := requestid.Get(c); rid != "" {
					fields = append(fields, log.String("request_id", rid))
				}
				if sc.IsValid() {
					fields = append(fields,
						log.String("trace_id", sc.TraceID().String()),
						log.String("span_id", sc.SpanID().String()),
					)
				}
				if isError {
					fields = append(fields, log.String(attrErrorType, strconv.Itoa(status)))
				}
				if rec != nil {
					fields = append(fields,
						log.Any("panic", rec),
						log.String("stack", string(debug.Stack())),
					)
				}
				if cfg.Payload.Enabled {
					var sseCount int
					var wasSSE bool
					if tw != nil {
						sseCount, wasSSE = tw.finalizeSSE()
					}
					fields = append(fields,
						log.String("req.body", payloadString(reqBody.buf.Bytes(), c.Request.Header.Get("Content-Type"))),
						log.String("req.query", c.Request.URL.RawQuery),
						log.String("req.headers", flattenHeaders(c.Request.Header)),
					)
					if wasSSE {
						fields = append(fields, log.Int("event.count", sseCount))
					} else {
						fields = append(fields, log.String("resp.body", payloadString(respBody.buf.Bytes(), c.Writer.Header().Get("Content-Type"))))
					}
				}
				switch {
				case status >= http.StatusInternalServerError:
					log.Error(c.Request.Context(), accessLogTag, fields...)
				case status >= http.StatusBadRequest:
					log.Warn(c.Request.Context(), accessLogTag, fields...)
				default:
					log.Info(c.Request.Context(), accessLogTag, fields...)
				}
			}
		}()

		c.Next()
	}
}

// scheme returns "https" when the request arrived over TLS, "http" otherwise.
func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// httpProtocolVersion maps a request's Proto ("HTTP/1.1", "HTTP/2.0", ...) to
// the OTel network.protocol.version value ("1.1", "2", "3").
func httpProtocolVersion(proto string) string {
	switch proto {
	case "HTTP/1.0":
		return "1.0"
	case "HTTP/1.1":
		return "1.1"
	case "HTTP/2.0", "HTTP/2":
		return "2"
	case "HTTP/3.0", "HTTP/3":
		return "3"
	default:
		return proto
	}
}

// serverAddrPort splits a Host header into the OTel server.address (host) and
// server.port. The port is dropped when absent or the scheme default, since the
// semconv makes server.port conditionally required only when non-default.
func serverAddrPort(host, urlScheme string) (addr string, port int) {
	addr = host
	if h, p, err := net.SplitHostPort(host); err == nil {
		addr = h
		port, _ = strconv.Atoi(p)
	}
	if (urlScheme == "https" && port == 443) || (urlScheme == "http" && port == 80) {
		port = 0
	}
	return addr, port
}
