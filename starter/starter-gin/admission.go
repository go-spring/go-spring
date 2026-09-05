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

package StarterGin

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go-spring.org/cloud/governance/fault"
	"go-spring.org/cloud/governance/resilience"
)

// buildAdmission builds the inbound admission middleware. The resilience
// executor is resolved through the NEUTRAL provider seam
// [resilience.ExecutorFor]: starter-govern registers a provider backed by the
// governance center, so this server gets its rate-limit / bulkhead / breaker
// policy WITHOUT injecting *governance.Center or even importing cloud/governance. When
// governance is not configured the seam yields a transparent no-op executor, so
// the admission middleware runs but never rejects (fn runs once, untouched).
// Hot-reload is driven on the backing executor by the provider, so an operator
// can tighten inbound admission without a restart, the same way every outbound
// client's policy is tuned. The executor is wrapped with observe-resilience so
// breaker trips / rejects emit span + counter + histogram + access log.
func buildAdmission(cfg Config) (gin.HandlerFunc, error) {
	resource := resilience.ResourceLabel("gin", cfg.Address)
	exec := resilience.ExecutorFor(resource)
	exec = resilience.WrapExecutor(exec, "gin")
	return resilienceAdmission(exec, resource), nil
}

// resilienceAdmission is the inbound admission middleware: each request runs
// through exec so the configured rate-limit / bulkhead / breaker policy is
// enforced before the handler chain. Rejects map to 429 (rate/bulkhead) or 503
// (circuit open); a handler-emitted 5xx counts as a failure for the breaker.
//
// Inbound admission must NOT retry — a handler that has already produced side
// effects cannot be replayed (inbound serving is not idempotent). Leave
// Policy.MaxRetries at 0; a Written() guard also prevents reentry regardless.
func resilienceAdmission(exec resilience.Executor, resource string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var served bool
		err := exec.Execute(c.Request.Context(), resource, func(ctx context.Context) error {
			if served {
				return nil // reentry guard: handler already ran this request
			}
			c.Next()
			served = c.Writer.Written()
			if c.Writer.Status() >= 500 {
				return errHTTP5xx{code: c.Writer.Status()}
			}
			return nil
		})
		if err == nil {
			return
		}
		switch {
		case errors.Is(err, resilience.ErrRateLimited), errors.Is(err, resilience.ErrBulkheadFull):
			if !c.Writer.Written() {
				c.AbortWithStatus(http.StatusTooManyRequests)
			}
		case errors.Is(err, resilience.ErrCircuitOpen):
			if !c.Writer.Written() {
				c.AbortWithStatus(http.StatusServiceUnavailable)
			}
		}
	}
}

// errHTTP5xx is the failure signal a handler-emitted 5xx feeds back into the
// breaker (the breaker counts non-nil errors from fn).
type errHTTP5xx struct{ code int }

func (e errHTTP5xx) Error() string { return fmt.Sprintf("http: server returned %d", e.code) }

// buildFault builds the inbound fault-injection middleware. It is the server-
// side counterpart to the client starters' fault.WrapExecutor: instead of
// wrapping an outbound Executor, it gates the handler call with [fault.Apply] so
// a configured fraction of inbound requests are made to fail or slow down —
// letting an operator "set fire" to a running server to verify its observe, its
// own resilience admission, and the upstream clients' retry/breaker behavior.
// The injector comes from the neutral [fault.InjectorFor] seam (backed by the
// governance center); when no injector is registered Apply is a transparent
// pass-through, so this is always installed. Unlike the static-cfg build it
// replaces, fault can now be hot-toggled at runtime without a restart.
func buildFault() gin.HandlerFunc {
	return func(c *gin.Context) {
		err := fault.Apply(c.Request.Context(), fault.InjectorFor(), "gin", func() error {
			c.Next()
			return nil
		})
		if err != nil && !c.Writer.Written() {
			// An injected fault (or a latency cancelled by the request deadline)
			// surfaces as 503 — the server is "unavailable" for this request.
			c.AbortWithStatus(http.StatusServiceUnavailable)
		}
	}
}
