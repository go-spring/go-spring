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
	"go-spring.org/cloud/chain"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go-spring.org/cloud/resilience"
)

// stubServerExecutor is an chain.Executor whose outcome the test dictates. reject
// simulates a pre-handler rejection; attempts > 1 simulates a policy with
// retries; fnErr records what the filter fed back as the call's outcome.
type stubServerExecutor struct {
	reject   error
	attempts int
	runs     int
	fnErr    error
}

func (s *stubServerExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
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

func (s *stubServerExecutor) Close() error { return nil }

// serveServerPolicy drives one request through the inbound filter built over a
// stub executor, and reports the response plus how often the handler ran.
func serveServerPolicy(t *testing.T, exec *stubServerExecutor, handler http.HandlerFunc) (*httptest.ResponseRecorder, int) {
	t.Helper()
	handlerRuns := 0
	// Wrap the stub in a filter built the same way ServerPolicy builds one, so the
	// upstream resolution stays out of the test's way.
	mw := serverPolicyWith(exec, "http-server::9090")
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerRuns++
		handler(w, r)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec, handlerRuns
}

// TestServerPolicy_BoundsHandlerByCallersBudget proves the receiving half of a
// budget-carrying hop: a request that arrives with a remaining-budget header
// reaches the handler under a context bounded by it, so the handler — and any
// outbound call it makes — cannot outlive the caller's allowance.
func TestServerPolicy_BoundsHandlerByCallersBudget(t *testing.T) {
	exec := &stubServerExecutor{}
	var hadDeadline bool
	var remaining time.Duration
	mw := serverPolicyWith(exec, "http-server::9090")
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		d, ok := r.Context().Deadline()
		hadDeadline, remaining = ok, time.Until(d)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(resilience.BudgetHeader, "50")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !hadDeadline {
		t.Fatal("handler ran with no deadline: the caller's budget was not applied")
	}
	if remaining > 50*time.Millisecond {
		t.Fatalf("handler deadline %v exceeds the budget the caller sent (50ms)", remaining)
	}
}

// TestServerPolicy_NoBudgetHeaderLeavesContextAlone proves a request that carries no
// budget is not given one: a server invents no deadline it was not asked for.
func TestServerPolicy_NoBudgetHeaderLeavesContextAlone(t *testing.T) {
	exec := &stubServerExecutor{}
	var hadDeadline bool
	mw := serverPolicyWith(exec, "http-server::9090")
	h := mw(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, hadDeadline = r.Context().Deadline()
	}))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	if hadDeadline {
		t.Fatal("handler ran with a deadline the caller never sent")
	}
}

func TestServerPolicy_PassThroughRunsHandlerOnce(t *testing.T) {
	exec := &stubServerExecutor{}
	rec, handlerRuns := serveServerPolicy(t, exec, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if handlerRuns != 1 || exec.runs != 1 {
		t.Fatalf("handler/executor runs: got %d/%d, want 1/1", handlerRuns, exec.runs)
	}
}

func TestServerPolicy_RejectsMapToStatus(t *testing.T) {
	cases := []struct {
		name string
		exec error
		want int
	}{
		{"rate limited", chain.ErrRateLimited, http.StatusTooManyRequests},
		{"bulkhead full", chain.ErrBulkheadFull, http.StatusTooManyRequests},
		{"circuit open", chain.ErrCircuitOpen, http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			exec := &stubServerExecutor{reject: tc.exec}
			rec, handlerRuns := serveServerPolicy(t, exec, func(w http.ResponseWriter, r *http.Request) {
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
func TestServerPolicy_Handler5xxFeedsBreaker(t *testing.T) {
	exec := &stubServerExecutor{}
	rec, _ := serveServerPolicy(t, exec, func(w http.ResponseWriter, r *http.Request) {
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
func TestServerPolicy_DoesNotReplayHandler(t *testing.T) {
	exec := &stubServerExecutor{attempts: 3}
	rec, handlerRuns := serveServerPolicy(t, exec, func(w http.ResponseWriter, r *http.Request) {
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
func TestServerPolicy_StatusWriterStaysTransparent(t *testing.T) {
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
