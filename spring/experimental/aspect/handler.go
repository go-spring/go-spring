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

package aspect

import (
	"context"
	"fmt"
	"net/http"
)

// ResourceFunc derives the joinpoint method name from an inbound request. It lets
// a chain scope its interceptors (cache keys, pointcuts, audit labels) per route.
type ResourceFunc func(r *http.Request) string

// NewHandler wraps next so every inbound request flows through chain as a
// joinpoint. It is the server-side seam for HTTP handlers, the counterpart to the
// decorator convention used for service methods: cross-cutting concerns
// (timing/audit via [Timing], panic-to-error via [Recover], admission checks, ...)
// are declared once on the chain and applied around any http.Handler, whether a
// single route or a whole http.ServeMux.
//
// The request is served exactly once even under an interceptor that would retry,
// because inbound serving is not idempotent — a re-run would re-invoke an
// already-written handler. A response whose status is >= 500 is reported to the
// chain as an error so [Timing] and metrics see the failure. When chain is nil or
// empty, next is returned unchanged.
func NewHandler(next http.Handler, chain *Chain, method ResourceFunc) http.Handler {
	if chain == nil || len(chain.interceptors) == 0 {
		return next
	}
	if method == nil {
		method = func(r *http.Request) string { return r.URL.Path }
	}
	return &handler{next: next, chain: chain, method: method}
}

type handler struct {
	next   http.Handler
	chain  *Chain
	method ResourceFunc
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	served := false
	_ = h.chain.RunE(r.Context(), h.method(r), func(ctx context.Context) error {
		if served {
			// Guard against an interceptor re-invoking an already-served request:
			// the response is committed, so stop the loop without re-running.
			return nil
		}
		served = true
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h.next.ServeHTTP(rec, r.WithContext(ctx))
		if rec.status >= 500 {
			return fmt.Errorf("aspect: handler returned %d", rec.status)
		}
		return nil
	})
}

// statusRecorder captures the status code so interceptors can see whether the
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
