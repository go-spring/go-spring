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

package resilience

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// NewHandler wraps next so every inbound request flows through exec. It is the
// server-side seam of the framework: rate limiting and bulkhead isolation are
// applied at admission, protecting the process from overload before a request
// reaches the business handler. Drop it in front of any http.Handler, including
// an http.ServeMux, to guard a whole server, or wrap a single route.
//
// Rejections are translated to HTTP status codes so the seam stays transparent
// to clients: [ErrRateLimited] and [ErrBulkheadFull] become 429 Too Many
// Requests, [ErrCircuitOpen] becomes 503 Service Unavailable. A response whose
// status is 5xx counts as a failure for the breaker, so a resource that keeps
// failing inbound is shed. When exec is nil next is returned unchanged.
//
// Inbound serving is not idempotent, so the request is served exactly once even
// under a retry policy (a retry would re-run an already-written handler); retry
// is meaningful for the client-side seams, not here.
func NewHandler(next http.Handler, exec Executor, resource ResourceFunc) http.Handler {
	if exec == nil {
		return next
	}
	if resource == nil {
		resource = func(r *http.Request) string { return r.URL.Path }
	}
	return &handler{next: next, exec: exec, resource: resource}
}

type handler struct {
	next     http.Handler
	exec     Executor
	resource ResourceFunc
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	served := false
	err := h.exec.Execute(r.Context(), h.resource(r), func(ctx context.Context) error {
		if served {
			// Guard against a retry policy re-invoking an already-served request:
			// the response is committed, so report success and stop the loop.
			return nil
		}
		served = true
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.next.ServeHTTP(rec, r.WithContext(ctx))
		if rec.status >= 500 {
			return fmt.Errorf("resilience: handler returned %d", rec.status)
		}
		return nil
	})
	if err != nil && !served {
		// Rejected before serving: nothing has been written yet, so map the
		// admission decision onto a status code.
		writeRejection(w, err)
	}
}

// writeRejection maps a neutral admission error onto an HTTP status code.
func writeRejection(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrCircuitOpen):
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	default: // ErrRateLimited, ErrBulkheadFull
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	}
}

// statusRecorder captures the status code so the breaker can see whether the
// handler failed, while writes pass straight through to the real writer.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.wroteHeader {
		return
	}
	s.wroteHeader = true
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if !s.wroteHeader {
		s.WriteHeader(http.StatusOK)
	}
	return s.ResponseWriter.Write(b)
}
