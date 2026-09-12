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
	"errors"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"go-spring.org/cloud/governance/resilience"
)

// buildAdmission builds the inbound admission middleware. The resilience
// executor is resolved through the NEUTRAL provider seam
// [resilience.ExecutorFor]: starter-govern registers a provider backed by the
// governance center, so this server gets its rate-limit / bulkhead / breaker
// policy WITHOUT injecting *governance.Center or even importing cloud/governance.
// When governance is not configured the seam yields a transparent no-op
// executor, so the admission middleware runs but never rejects (next runs once,
// untouched). Hot-reload is driven on the backing executor by the provider, so
// an operator can tighten inbound admission without a restart, the same way
// every outbound client's policy is tuned.
//
// The label is "echo:<address>", the same one the fault middleware's sibling
// tables use for this server, so one govern rule covers the whole inbound side.
func buildAdmission(cfg Config) echo.MiddlewareFunc {
	resource := resilience.ResourceLabel("echo", cfg.Address)
	return resilienceAdmission(resilience.ExecutorFor("echo", resource), resource)
}

// resilienceAdmission is the inbound admission middleware: each request runs
// through exec so the configured rate-limit / bulkhead / breaker policy is
// enforced before the handler chain. Rejects map to 429 (rate/bulkhead) or 503
// (circuit open); a handler error or a committed 5xx counts as a failure for the
// breaker.
//
// Inbound admission must NOT retry — a handler that has already produced side
// effects cannot be replayed (inbound serving is not idempotent). Leave
// Policy.MaxRetries at 0; the committed guard also prevents reentry regardless.
func resilienceAdmission(exec resilience.Executor, resource string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var served bool
			var handlerErr error
			err := exec.Execute(c.Request().Context(), resource, func(ctx context.Context) error {
				if served {
					return nil // reentry guard: handler already ran this request
				}
				handlerErr = next(c)
				served = c.Response().Committed || handlerErr != nil
				if handlerErr != nil {
					return handlerErr
				}
				if c.Response().Status >= 500 {
					return errHTTP5xx{code: c.Response().Status}
				}
				return nil
			})

			// A handler's own error (or a committed response) belongs to the
			// caller: return it so echo's HTTPErrorHandler renders it, exactly as
			// the fault middleware lets it through.
			if handlerErr != nil {
				return handlerErr
			}
			switch {
			case errors.Is(err, resilience.ErrRateLimited), errors.Is(err, resilience.ErrBulkheadFull):
				return echo.NewHTTPError(http.StatusTooManyRequests)
			case errors.Is(err, resilience.ErrCircuitOpen):
				return echo.NewHTTPError(http.StatusServiceUnavailable)
			}
			return nil
		}
	}
}

// errHTTP5xx is the failure signal a handler-emitted 5xx feeds back into the
// breaker (the breaker counts non-nil errors from fn).
type errHTTP5xx struct{ code int }

func (e errHTTP5xx) Error() string { return fmt.Sprintf("http: server returned %d", e.code) }
