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
	"strings"

	"github.com/gin-gonic/gin"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/trace"
)

// respCaptureKey is the gin-context key under which responseCapture publishes
// its captureWriter, so the Observe middleware can read the captured
// (uncompressed) response body in its end-of-request finalize.
const respCaptureKey = "_gs_gin_resp_capture"

// responseCapture installs the innermost response-writer wrapper that captures
// the UNCOMPRESSED response body. It must sit inside the gzip middleware (and any
// other response transformer) so it sees the bytes the handler writes before
// they are compressed - the logical response body, not the wire bytes. It
// publishes its captureWriter on the gin context (respCaptureKey) for Observe to
// read in its defer.
//
// Splitting capture out of Observe resolves the gzip conflict: when Observe did
// its own capture its tee sat outside gzip and recorded compressed bytes (so the
// access log's resp.body was garbage under gzip). Now capture is innermost, so
// it records uncompressed bytes whether or not gzip is on. SSE per-event logging
// lives here too, since it is coupled to the response-writer wrap (Write/Flush).
func responseCapture() gin.HandlerFunc {
	return func(c *gin.Context) {
		cw := &captureWriter{
			ResponseWriter: c.Writer,
			capture:        &limitedBuffer{max: payloadCaptureLimit},
			sse: &sseLogger{
				c:   c,
				tag: accessLogTag,
				buf: limitedBuffer{max: payloadCaptureLimit},
			},
		}
		c.Set(respCaptureKey, cw)
		c.Writer = cw
	}
}

// getResponseCapture returns the captureWriter published by responseCapture, or
// nil when capture is not installed (payload capture disabled).
func getResponseCapture(c *gin.Context) *captureWriter {
	v, ok := c.Get(respCaptureKey)
	if !ok {
		return nil
	}
	cw, _ := v.(*captureWriter)
	return cw
}

// captureWriter wraps gin's ResponseWriter to capture the uncompressed response
// body for the access log. For a normal response it copies writes into a capture
// buffer; for an SSE response (text/event-stream) it instead accumulates writes
// and logs each flushed chunk in real time (see sseLogger), so a live stream's
// events hit the log as they are sent - not all at once on disconnect.
type captureWriter struct {
	gin.ResponseWriter
	capture *limitedBuffer
	sse     *sseLogger
}

func (w *captureWriter) Write(b []byte) (int, error) {
	if w.sse != nil && isSSEContentType(w.Header().Get("Content-Type")) {
		w.sse.active = true
		w.sse.buf.Write(b)
	} else if w.capture != nil {
		w.capture.Write(b)
	}
	return w.ResponseWriter.Write(b)
}

func (w *captureWriter) WriteString(s string) (int, error) {
	return w.Write([]byte(s))
}

// Flush forwards the flush to the client and, for an SSE response, emits a log
// record for the events written since the last flush.
func (w *captureWriter) Flush() {
	w.ResponseWriter.Flush()
	if w.sse != nil && w.sse.active {
		w.sse.flush()
	}
}

// finalizeSSE flushes any trailing unflushed SSE bytes and reports how many
// event records were logged and whether this response was SSE.
func (w *captureWriter) finalizeSSE() (count int, wasSSE bool) {
	if w.sse == nil || !w.sse.active {
		return 0, false
	}
	w.sse.flush()
	return w.sse.count, true
}

// capturedBody returns the captured uncompressed response bytes (empty for SSE
// responses, whose events are logged separately via finalizeSSE).
func (w *captureWriter) capturedBody() []byte {
	if w.capture == nil {
		return nil
	}
	return w.capture.buf.Bytes()
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
	if rid := RequestIDFromContext(s.c.Request.Context()); rid != "" {
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
