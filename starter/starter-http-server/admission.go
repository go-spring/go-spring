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

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is govern.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// ServerPolicy returns the inbound admission filter: every request runs through
// the governance executor for label, so the configured rate-limit / bulkhead /
// breaker policy is enforced before the wrapped handler. It is the stdlib
// member of the same family as gin's and echo's admission middleware, which
// their starters install automatically — here you compose it yourself, because
// this package provides filters rather than owning a server.
//
// label is the governance service label this server is addressed as, e.g.
// "http-server::9090" (see cloud/governance/README.md 设计说明 §6). Build it with
// resilience.ServiceLabel(system, addr) if you want the conventional shape;
// govern rules match it with govern.client.rules[N].service=<label>.
//
// mgr is the resilience authority the filter's executor is resolved from. This
// package provides filters rather than owning a server, so it has no bean to
// inject into: whoever composes the chain takes the *resilience.Manager bean as
// a parameter on their own bean and passes it here. A nil manager is the
// standalone case — the filter then runs under an unarmed manager, i.e. a
// transparent pass-through that never rejects.
//
// Rejections map to 429 (rate limit, bulkhead full) and 503 (circuit open); a
// handler-committed 5xx is fed back to the executor so the breaker sees
// server-side errors. Inbound admission never retries — a handler that already
// produced side effects cannot be replayed, and [resilience.ServerPolicy] has no
// retry field to express one with; a reentry guard makes a stray retry harmless
// regardless.
//
// Compose it inside Chain, after the security filters that must run even for a
// rejected request, and outside the handler:
//
//	Chain(CORS(corsCfg), ServerPolicy("http-server::9090", mgr))(mux)
func ServerPolicy(label string, mgr *resilience.Manager) Middleware {
	if mgr == nil {
		mgr = resilience.NewManager()
	}
	return admissionWith(mgr.ServerExecutorFor("http-server", label), label)
}

// admissionWith builds the filter over an already-resolved executor. It is the
// seam the tests build on, and keeps the executor lookup out of the filter
// itself.
func admissionWith(exec resilience.ServerExecutor, label string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sw := &statusWriter{ResponseWriter: w}
			var served bool
			err := exec.Execute(r.Context(), func(ctx context.Context) error {
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
// use http.NewResponseController(w) instead, or put ServerPolicy outside the
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
