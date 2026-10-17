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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// serveServerPolicy drives one GET through an echo engine carrying only the
// inbound middleware, and reports the response plus how often the handler
// itself ran. The middleware runs with observation on, over a real server
// observer (the process-wide providers are no-ops here). The center is built
// over an authority armed with the empty document, so governance never rejects
// and the tests are about the middleware's pass-through behavior.
func serveServerPolicy(t *testing.T, handler echo.HandlerFunc) (*httptest.ResponseRecorder, int) {
	t.Helper()
	e := echo.New()
	handlerRuns := 0
	in := testCenter(nil).InboundFor("echo", "echo:test")
	e.Use(resilienceServerPolicy(in, "echo:test", true))
	e.GET("/x", func(c echo.Context) error {
		handlerRuns++
		return handler(c)
	})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	return rec, handlerRuns
}

func TestServerPolicy_PassThroughRunsHandlerOnce(t *testing.T) {
	rec, handlerRuns := serveServerPolicy(t, func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", rec.Code)
	}
	if handlerRuns != 1 {
		t.Fatalf("handler runs: got %d, want 1", handlerRuns)
	}
}

// A handler's own error belongs to the caller: it must reach echo's
// HTTPErrorHandler untouched.
func TestServerPolicy_HandlerErrorPassesThrough(t *testing.T) {
	rec, _ := serveServerPolicy(t, func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusTeapot, "handler said so")
	})
	if rec.Code != http.StatusTeapot {
		t.Fatalf("status: got %d, want 418", rec.Code)
	}
}

// A handler-emitted 5xx must survive the middleware untouched.
func TestServerPolicy_Handler5xxSurvives(t *testing.T) {
	rec, _ := serveServerPolicy(t, func(c echo.Context) error {
		return c.String(http.StatusInternalServerError, "boom")
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("the handler's own response must survive: got %d, want 500", rec.Code)
	}
}
