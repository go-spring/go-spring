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
	"net/http"
	"net/http/httptest"
	"testing"

	"go-spring.org/cloud/governance/resilience"
)

// stubAdmissionExecutor is an Executor whose outcome the test dictates. reject
// simulates a pre-handler rejection; attempts > 1 simulates a policy with
// retries; fnErr records what the filter fed back as the call's outcome.
type stubAdmissionExecutor struct {
	reject   error
	attempts int
	runs     int
	fnErr    error
}

func (s *stubAdmissionExecutor) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	if s.reject != nil {
		return s.reject
	}
	var err error
	for range max(s.attempts, 1) {
		s.runs++
		err = fn(ctx)
		s.fnErr = err
		if err == nil {
			return nil
		}
	}
	return err
}

func (s *stubAdmissionExecutor) Close() error                    { return nil }
func (s *stubAdmissionExecutor) Refresh(resilience.Policy) error { return nil }

// serveAdmission drives one request through the admission filter built over a
// stub executor, and reports the response plus how often the handler ran.
func serveAdmission(t *testing.T, exec *stubAdmissionExecutor, handler http.HandlerFunc) (*httptest.ResponseRecorder, int) {
	t.Helper()
	handlerRuns := 0
	// Wrap the stub in a filter built the same way Admission builds one, so the
	// upstream resolution stays out of the test's way.
	mw := admissionWith(exec, "http-server::9090")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerRuns++
		handler(w, r)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec, handlerRuns
}

func TestAdmission_PassThroughRunsHandlerOnce(t *testing.T) {
	exec := &stubAdmissionExecutor{}
	rec, handlerRuns := serveAdmission(t, exec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if handlerRuns != 1 || exec.runs != 1 {
		t.Fatalf("handler/executor runs: got %d/%d, want 1/1", handlerRuns, exec.runs)
	}
}

func TestAdmission_RejectsMapToStatus(t *testing.T) {
	cases := []struct {
		name string
		exec error
		want int
	}{
		{"rate limited", resilience.ErrRateLimited, http.StatusTooManyRequests},
		{"bulkhead full", resilience.ErrBulkheadFull, http.StatusTooManyRequests},
		{"circuit open", resilience.ErrCircuitOpen, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exec := &stubAdmissionExecutor{reject: tc.exec}
			rec, handlerRuns := serveAdmission(t, exec, func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			})
			if rec.Code != tc.want {
				t.Fatalf("status: got %d, want %d", rec.Code, tc.want)
			}
			if handlerRuns != 0 {
				t.Fatalf("a rejected request must not reach the handler, ran %d times", handlerRuns)
			}
		})
	}
}

// A handler-committed 5xx must be reported to the executor, or the breaker
// would never see server-side errors.
func TestAdmission_Handler5xxFeedsBreaker(t *testing.T) {
	exec := &stubAdmissionExecutor{}
	rec, _ := serveAdmission(t, exec, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("the handler's own response must survive: got %d, want 500", rec.Code)
	}
	if exec.fnErr == nil {
		t.Fatal("a committed 5xx must be fed back to the executor as a failure")
	}
}

// Inbound serving is not idempotent: a retrying policy must not replay the
// handler. The retry sees no error (the guard returns nil) and the handler
// still runs exactly once.
func TestAdmission_DoesNotReplayHandler(t *testing.T) {
	exec := &stubAdmissionExecutor{attempts: 3}
	rec, handlerRuns := serveAdmission(t, exec, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	if handlerRuns != 1 {
		t.Fatalf("handler must run exactly once despite retries, ran %d times", handlerRuns)
	}
	if exec.runs != 2 {
		t.Fatalf("guard should stop the retry loop at the second call: got %d, want 2", exec.runs)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}

// The wrapper must stay transparent for the interfaces handlers rely on:
// http.ResponseController reaches the original writer through Unwrap, and
// http.Flusher is forwarded.
func TestAdmission_StatusWriterStaysTransparent(t *testing.T) {
	rec := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: rec}

	if got := sw.Unwrap(); got != http.ResponseWriter(rec) {
		t.Fatalf("Unwrap must return the original writer, got %T", got)
	}
	if _, ok := any(sw).(http.Flusher); !ok {
		t.Fatal("statusWriter must implement http.Flusher")
	}
	sw.WriteHeader(http.StatusTeapot)
	if sw.status != http.StatusTeapot || !sw.written {
		t.Fatalf("WriteHeader should be recorded: status=%d written=%v", sw.status, sw.written)
	}
	// A second WriteHeader must not overwrite the recorded status, matching
	// net/http's own "first header wins" rule.
	sw.WriteHeader(http.StatusOK)
	if sw.status != http.StatusTeapot {
		t.Fatalf("first WriteHeader wins: got %d, want 418", sw.status)
	}
}
