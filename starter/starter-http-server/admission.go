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

package StarterHTTPServer

import (
	"context"
	"errors"
	"net/http"

	"go-spring.org/cloud/governance/resilience"
)

// Admission returns the inbound admission filter: every request runs through
// the governance executor for label, so the configured rate-limit / bulkhead /
// breaker policy is enforced before the wrapped handler. It is the stdlib
// member of the same family as gin's and echo's admission middleware, which
// their starters install automatically — here you compose it yourself, because
// this package provides filters rather than owning a server.
//
// label is the governance resource label this server is addressed as, e.g.
// "http-server::9090" (see cloud/governance/DESIGN_CN.md §6). Build it with
// resilience.ResourceLabel(system, addr) if you want the conventional shape;
// govern rules match it with govern.rules[N].resources=<label>.
//
// The executor is resolved through the NEUTRAL provider seam
// [resilience.ExecutorFor]: starter-govern registers a provider backed by the
// governance center, so this filter gets its policy WITHOUT importing
// cloud/governance. With governance off the seam yields a transparent no-op
// executor, so the filter runs and never rejects.
//
// Rejections map to 429 (rate limit, bulkhead full) and 503 (circuit open); a
// handler-committed 5xx is fed back to the executor so the breaker sees
// server-side errors. Inbound admission never retries — a handler that already
// produced side effects cannot be replayed — so leave Policy.MaxRetries at 0; a
// reentry guard makes a retrying policy harmless anyway.
//
// Compose it inside Chain, after the security filters that must run even for a
// rejected request, and outside the handler:
//
//	Chain(CORS(corsCfg), Admission("http-server::9090"))(mux)
func Admission(label string) Middleware {
	return admissionWith(resilience.ExecutorFor("http-server", label), label)
}

// admissionWith builds the filter over an already-resolved executor. It is the
// seam the tests build on, and keeps the executor lookup out of the filter
// itself.
func admissionWith(exec resilience.Executor, label string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w}
			var served bool
			err := exec.Execute(r.Context(), label, func(ctx context.Context) error {
				if served {
					return nil // reentry guard: the handler already ran this request
				}
				served = true
				next.ServeHTTP(sw, r)
				if sw.written && sw.status >= 500 {
					return errHTTP5xx{code: sw.status}
				}
				return nil
			})
			switch {
			case errors.Is(err, resilience.ErrRateLimited), errors.Is(err, resilience.ErrBulkheadFull):
				if !sw.written {
					http.Error(sw, "too many requests", http.StatusTooManyRequests)
				}
			case errors.Is(err, resilience.ErrCircuitOpen):
				if !sw.written {
					http.Error(sw, "service unavailable", http.StatusServiceUnavailable)
				}
			}
		})
	}
}

// errHTTP5xx is the failure signal a handler-committed 5xx feeds back into the
// breaker (the breaker counts non-nil errors from fn).
type errHTTP5xx struct{ code int }

func (e errHTTP5xx) Error() string { return http.StatusText(e.code) }

// statusWriter records the status code and whether the response was committed,
// so the admission filter can tell a handler 5xx from a success and can avoid
// replaying a handler that already answered.
//
// Unwrap exposes the original writer to http.ResponseController, which is how
// Flush / Hijack / SetWriteDeadline keep working through this wrapper; Flush is
// also forwarded directly for handlers that assert http.Flusher. A handler that
// asserts http.Hijacker directly (older websocket libraries) will not see it —
// use http.NewResponseController(w) instead, or put Admission outside the
// upgrade path.
type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (w *statusWriter) WriteHeader(code int) {
	if !w.written {
		w.status = code
		w.written = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if !w.written {
		w.status = http.StatusOK
		w.written = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Flush forwards to the underlying writer when it supports flushing, so
// streaming handlers keep working without going through ResponseController.
func (w *statusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
