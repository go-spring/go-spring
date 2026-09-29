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

	// Blank import: importing this starter brings the governance authority with
	// it — starter-governance registers the *resilience.Manager, *loadbalance.
	// Manager, *fault.Injector and *governance.Center beans this package injects.
	// Turning governance OFF is govern.enabled=false (or binding no rule source),
	// not the absence of the starter. The injected parameters stay nullable, so a
	// container that somehow lacks these beans degrades to a transparent
	// pass-through instead of failing to boot.
	_ "go-spring.org/starter-governance"
)

// buildServerPolicy builds the inbound admission middleware. The admission executor
// is built from the injected [resilience.Manager], so this server gets its
// rate-limit / bulkhead / breaker limits from the governance document's SERVER
// block (govern.server.*) WITHOUT naming *governance.Center. A nil manager — a
// standalone call, or an app that does not import starter-governance — is
// normalized to an unarmed one, whose executor is a transparent pass-through, so
// the admission middleware runs but never rejects (fn runs once, untouched). The
// executor handle resolves its backing implementation per call and follows the
// manager's hot-reload, so an operator can tighten inbound admission without a
// restart, independently of every outbound client's policy. The executor is
// wrapped with observe-resilience so breaker trips / rejects emit span + counter
// + histogram + access log.
func buildServerPolicy(cfg Config, mgr *resilience.Manager) (gin.HandlerFunc, error) {
	if mgr == nil {
		mgr = resilience.NewManager()
	}
	service := resilience.ServiceLabel("gin", cfg.Address)
	exec := mgr.ServerExecutorFor("gin", service)
	return resilienceServerPolicy(exec, service), nil
}

// resilienceServerPolicy is the inbound admission middleware: each request runs
// through exec so the configured rate-limit / bulkhead / breaker policy is
// enforced before the handler chain. Rejects map to 429 (rate/bulkhead) or 503
// (circuit open); a handler-emitted 5xx counts as a failure for the breaker.
//
// Inbound admission cannot retry — a handler that has already produced side
// effects cannot be replayed (inbound serving is not idempotent). That is
// structural here: [resilience.ServerPolicy] has no retry field, so the executor
// built from the server block has no retry stage at all; a Written() guard also
// prevents reentry regardless.
func resilienceServerPolicy(exec resilience.ServerExecutor, service string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var served bool
		err := exec.Execute(c.Request.Context(), func(ctx context.Context) error {
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
// side counterpart to the client starters' fault.WrapClientExecutor: instead of
// wrapping an outbound Executor, it gates the handler call with [fault.ApplyServer] so
// a configured fraction of inbound requests are made to fail or slow down —
// letting an operator "set fire" to a running server to verify its observe, its
// own resilience admission, and the upstream clients' retry/breaker behavior.
//
// inj is the governance starter's injector bean, captured once here and reused
// for every request: the center hot-swaps the injector's config in place
// ([fault.Injector.SetConfig]) rather than replacing the bean, so this reference
// always observes the live config and fault stays runtime-toggleable without a
// restart. A nil inj — governance not imported — makes Apply a transparent
// pass-through, so the middleware is always installed.
func buildFault(inj *fault.Injector) gin.HandlerFunc {
	return func(c *gin.Context) {
		err := fault.ApplyServer(c.Request.Context(), inj, "gin", func() error {
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
