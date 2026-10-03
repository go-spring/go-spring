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
	"go-spring.org/cloud/chain"
	"net/http"

	"go-spring.org/cloud/observability"
	"go-spring.org/cloud/propagate"
	"go-spring.org/cloud/resilience"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// ServerPolicy returns the inbound inbound filter: every request runs through
// the governance executor for label, so the configured rate-limit / bulkhead /
// breaker policy is enforced before the wrapped handler. It is the stdlib
// member of the same family as gin's and echo's inbound middleware, which
// their starters install automatically — here you compose it yourself, because
// this package provides filters rather than owning a server.
//
// label is the governance service label this server is addressed as, e.g.
// "http-server::9090" (see cloud/governance/README.md 设计说明 §6). Build it with
// resilience.ServiceLabel(system, addr) if you want the conventional shape;
// governance rules match it with spring.governance.client.rules[N].service=<label>.
//
// mgr is the resilience authority the filter's executor is resolved from. This
// package provides filters rather than owning a server, so it has no bean to
// inject into: whoever composes the chain takes the governance center bean
// (*governance.Center) on their own bean and passes center.Resilience() here. A
// nil manager is the standalone case — the filter then runs under an unarmed
// manager, i.e. a transparent pass-through that never rejects.
//
// Rejections map to 429 (rate limit, bulkhead full) and 503 (circuit open); a
// handler-committed 5xx is fed back to the executor so the breaker sees
// server-side errors. Inbound inbound never retries — a handler that already
// produced side effects cannot be replayed, and [resilience.ServerPolicy] has no
// retry field to express one with; a reentry guard makes a stray retry harmless
// regardless.
//
// Compose it inside Chain, after the security filters that must run even for a
// rejected request, and outside the handler:
//
//	Chain(CORS(corsCfg), ServerPolicy("http-server::9090", center.Resilience()))(mux)
func ServerPolicy(label string, mgr *resilience.Manager) Middleware {
	if mgr == nil {
		mgr = resilience.NewManager(nil)
	}
	return serverPolicyWith(mgr.ServerExecutorFor("http-server", label), label)
}

// serverPolicyWith builds the filter over an already-resolved executor. It is the
// seam the tests build on, and keeps the executor lookup out of the filter
// itself.
func serverPolicyWith(exec chain.Executor, label string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Declare what this request IS, so the executor's emitter can name the
			// span, the family metrics and the access log from it. This package
			// emits nothing itself — the same division the client starters use.
			ctx := observability.WithOperation(r.Context(), operation(r))
			// The caller's remaining budget, when it sent one: the handler — and
			// every outbound call it makes — then runs on the earlier of that and
			// this server's own handling budget, so a request chain spends one
			// allowance instead of one per hop.
			ctx, cancel := resilience.WithBudget(ctx, propagate.Header(r.Header))
			defer cancel()

			sw := &statusWriter{ResponseWriter: w}
			var served bool
			err := exec.Execute(ctx, func(ctx context.Context) error {
				if served {
					return nil // reentry guard: the handler already ran this request
				}
				served = true
				// Hand the bounded context to the handler: the inbound executor
				// derived it, so the deadline that bounds this request is the one
				// the handler and its outbound clients actually run under.
				next.ServeHTTP(sw, r.WithContext(ctx))
				// The response half: what only the handler knows once it has
				// answered. An unwritten response is a 200, which is what net/http
				// sends on its behalf.
				code := sw.status
				if !sw.written {
					code = http.StatusOK
				}
				observability.ResponseFrom(ctx).Add(attribute.Int("http.response.status_code", code))
				if sw.written && sw.status >= 500 {
					return errHTTP5xx{code: sw.status}
				}
				return nil
			})
			switch {
			case errors.Is(err, chain.ErrRateLimited), errors.Is(err, chain.ErrBulkheadFull):
				if !sw.written {
					http.Error(sw, "too many requests", http.StatusTooManyRequests)
				}
			case errors.Is(err, chain.ErrCircuitOpen):
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
// so the inbound filter can tell a handler 5xx from a success and can avoid
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

// accessTag is the static log tag for this server's access log. The tag name
// uses an underscore because a tag is a syntax marker, not a path. It is
// registered here, at package init, because a tag must exist before the
// framework's first property refresh — see [log.RegisterAppTag].
var accessTag = log.RegisterAppTag("http_server", "access")

// operation is the semantic identity of one inbound request, declared on the
// context before the inbound executor runs. It is this package's WHOLE
// contribution to the request's signals: the emitter names the span, the family
// metrics and the access log from it (see the resilience observe layer), and
// this package emits nothing itself.
//
// What is known up front rides here — the method (bounded, so it labels a
// metric) and the URL path (drawn from the caller, so it is Detail: span and log
// only, never a label). What is NOT known up front — the response status code —
// is recorded by the handler as it answers, through [observability.Response];
// see the inbound filter below.
func operation(r *http.Request) observability.Operation {
	return observability.Operation{
		Name:   r.Method + " " + r.URL.Path,
		Metric: "http.server",
		Attrs: []attribute.KeyValue{
			attribute.String("http.request.method", r.Method),
		},
		Detail: []attribute.KeyValue{
			attribute.String("url.path", r.URL.Path),
		},
		LogTag: accessTag,
	}
}
