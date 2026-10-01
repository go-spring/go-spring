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

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is spring.governance.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// buildServerPolicy builds the inbound admission middleware. The resilience
// executor is built from the injected [resilience.Manager], so this server gets
// its rate-limit / bulkhead / breaker limits from the governance document's SERVER
// block (spring.governance.server.*)
// WITHOUT naming *governance.Center. A nil manager — a standalone call, or an app
// that does not import starter-governance — is normalized to an unarmed one,
// whose executor is a transparent pass-through, so the admission middleware runs
// but never rejects (next runs once, untouched). The executor handle resolves its
// backing implementation per call and follows the manager's hot-reload, so an
// operator can tighten inbound admission without a restart, the same way every
// outbound client's policy is tuned.
//
// The label is "echo:<address>", the same one the fault middleware's sibling
// tables use for this server, so one governance rule covers the whole inbound side.
func buildServerPolicy(cfg Config, mgr *resilience.Manager) echo.MiddlewareFunc {
	if mgr == nil {
		mgr = resilience.NewManager()
	}
	service := resilience.ServiceLabel("echo", cfg.Address)
	return resilienceServerPolicy(mgr.ServerExecutorFor("echo", service), service)
}

// resilienceServerPolicy is the inbound admission middleware: each request runs
// through exec so the configured rate-limit / bulkhead / breaker limits are
// enforced before the handler chain. Rejects map to 429 (rate/bulkhead) or 503
// (circuit open); a handler error or a committed 5xx counts as a failure for the
// breaker.
//
// Inbound admission must NOT retry — a handler that has already produced side
// effects cannot be replayed (inbound serving is not idempotent). Leave
// [resilience.ServerPolicy] has no retry field at all, so the executor built from the
// server block has no retry stage; the committed guard also prevents reentry.
func resilienceServerPolicy(exec resilience.ServerExecutor, service string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			var served bool
			var handlerErr error
			err := exec.Execute(c.Request().Context(), func(ctx context.Context) error {
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
