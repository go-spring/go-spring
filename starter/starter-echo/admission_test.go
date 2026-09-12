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

package StarterEcho

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"go-spring.org/cloud/governance/resilience"
)

// stubExecutor is an Executor whose behavior the test dictates, so the
// admission middleware's HTTP mapping can be exercised without a governance
// center. reject simulates a pre-fn rejection; attempts > 1 simulates a policy
// with retries (fn invoked repeatedly until it succeeds); fnErr records what
// the middleware fed back as the call's outcome.
type stubExecutor struct {
	reject   error
	attempts int
	runs     int
	fnErr    error
}

func (s *stubExecutor) Execute(ctx context.Context, _ string, fn func(context.Context) error) error {
	if s.reject != nil {
		return s.reject
	}
	n := max(s.attempts, 1)
	var err error
	for range n {
		s.runs++
		err = fn(ctx)
		s.fnErr = err
		if err == nil {
			return nil
		}
	}
	return err
}

func (s *stubExecutor) Close() error                    { return nil }
func (s *stubExecutor) Refresh(resilience.Policy) error { return nil }

// serveAdmission drives one GET through an echo engine carrying only the
// admission middleware, and reports the response plus how often the handler
// itself ran.
func serveAdmission(t *testing.T, exec resilience.Executor, handler echo.HandlerFunc) (*httptest.ResponseRecorder, int) {
	t.Helper()
	e := echo.New()
	handlerRuns := 0
	e.Use(resilienceAdmission(exec, "echo:test"))
	e.GET("/x", func(c echo.Context) error {
		handlerRuns++
		return handler(c)
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec, handlerRuns
}

func TestAdmission_PassThroughRunsHandlerOnce(t *testing.T) {
	exec := &stubExecutor{}
	rec, handlerRuns := serveAdmission(t, exec, func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
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
			exec := &stubExecutor{reject: tc.exec}
			rec, handlerRuns := serveAdmission(t, exec, func(c echo.Context) error {
				return c.String(http.StatusOK, "ok")
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

// A handler's own error belongs to the caller: it must reach echo's
// HTTPErrorHandler untouched, so the admission layer only owns its rejections.
func TestAdmission_HandlerErrorPassesThrough(t *testing.T) {
	exec := &stubExecutor{}
	rec, _ := serveAdmission(t, exec, func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusTeapot, "handler said so")
	})
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status: got %d, want 418", rec.Code)
	}
}

// A handler-emitted 5xx must be reported to the executor as a failure, or the
// breaker would never see server-side errors.
func TestAdmission_Handler5xxFeedsBreaker(t *testing.T) {
	exec := &stubExecutor{}
	rec, _ := serveAdmission(t, exec, func(c echo.Context) error {
		return c.String(http.StatusInternalServerError, "boom")
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("the handler's own response must survive: got %d, want 500", rec.Code)
	}
	if exec.fnErr == nil {
		t.Fatal("a 5xx must be fed back to the executor as a failure")
	}
}

// Inbound serving is not idempotent, so the reentry guard must stop a retrying
// policy from replaying the handler: the executor may call fn again, but the
// handler runs exactly once. The retry therefore sees no error and the loop
// stops — a second fn invocation, not a third.
func TestAdmission_DoesNotReplayHandler(t *testing.T) {
	exec := &stubExecutor{attempts: 3}
	rec, handlerRuns := serveAdmission(t, exec, func(c echo.Context) error {
		return c.String(http.StatusInternalServerError, "boom")
	})
	if handlerRuns != 1 {
		t.Fatalf("handler must run exactly once despite retries, ran %d times", handlerRuns)
	}
	if exec.runs != 2 {
		t.Fatalf("guard should stop the retry loop at the second fn call: got %d runs, want 2", exec.runs)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d, want 500", rec.Code)
	}
}
