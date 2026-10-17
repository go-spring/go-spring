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
	"go-spring.org/cloud/fault"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/guard"
	"go-spring.org/cloud/label"
	"go-spring.org/cloud/observability"
)

// buildServerPolicy builds the inbound governance middleware. The inbound
// governance object comes from the injected [governance.Center], so this server
// gets its rate-limit / bulkhead / breaker limits from the governance document's
// SERVER block (spring.governance.server.*). A nil center — a standalone call, or
// an app that does not import governance — is normalized inside the center to an
// unarmed authority, whose guard is a transparent pass-through, so the middleware
// runs but never rejects (the handler runs once, untouched). The object resolves
// its backing engine per call and follows the manager's hot-reload, so an operator
// can tighten inbound governance without a restart, independently of every
// outbound client's policy. The center composes the observe object around the
// guard, so breaker trips / rejects emit span + counter + histogram + access log.
func buildServerPolicy(cfg Config, center *governance.Center) (gin.HandlerFunc, error) {
	service := label.ServiceLabel("gin", cfg.Address)
	return resilienceServerPolicy(center.InboundFor("gin", service)), nil
}

// resilienceServerPolicy is the inbound governance middleware: each request runs
// through the inbound object so the configured rate-limit / bulkhead / breaker
// policy is enforced before the handler chain. Refusals map to 429
// (rate/bulkhead) or 503 (circuit open); a handler-emitted 5xx counts as a failure
// for the breaker.
//
// Inbound cannot retry — a handler that has already produced side effects cannot
// be replayed (inbound serving is not idempotent). That is structural:
// [guard.Policy] has no retry field, so the engine built from the
// server block has no retry stage at all; a Written() guard also prevents reentry
// regardless.
func resilienceServerPolicy(in *governance.Inbound) gin.HandlerFunc {
	return func(c *gin.Context) {
		var served bool
		// The object emits the request span, the inbound family metrics and the
		// access log; the route is named by the same word the fault rules match on.
		op := observability.ServerOperation{Action: faultTarget(c).Action}
		err := in.Run(c.Request.Context(), op, func(ctx context.Context) error {
			if served {
				return nil // reentry guard: handler already ran this request
			}
			// Hand the observed context to the handler chain: the object derived it,
			// and gin reads it back off the request, so the span the handler and its
			// outbound clients run under is the one this request produced.
			c.Request = c.Request.WithContext(ctx)
			c.Next()
			served = c.Writer.Written()
			if c.Writer.Status() >= 500 {
				return errHTTP5xx{code: c.Writer.Status()}
			}
			return nil
		})
		var ref *governance.Refusal
		if errors.As(err, &ref) && !c.Writer.Written() {
			c.AbortWithStatus(httpStatusFor(ref.Verdict()))
		}
	}
}

// httpStatusFor maps an inbound refusal to the HTTP status a client can act on:
// rate/bulkhead rejections are 429, an open circuit or an injected fault is 503.
func httpStatusFor(kind guard.Verdict) int {
	if kind == guard.VerdictCircuitOpen || kind == guard.VerdictFaultInjected || kind == guard.VerdictDegraded {
		return http.StatusServiceUnavailable
	}
	return http.StatusTooManyRequests
}

// errHTTP5xx is the failure signal a handler-emitted 5xx feeds back into the
// breaker (the breaker counts non-nil errors from fn).
type errHTTP5xx struct{ code int }

func (e errHTTP5xx) Error() string { return fmt.Sprintf("http: server returned %d", e.code) }

// faultTarget names the inbound request for the fault rules: the entrance
// family ("gin" — the family, not the address label the resilience server
// policy resolves) plus the interface being invoked, in the "METHOD /path" form
// an interface-precise rule needs. The route TEMPLATE is preferred over the
// concrete URL so a rule names the endpoint rather than one instance of it; it
// is empty when no route matched (a 404), and the concrete path is then the
// honest name.
func faultTarget(c *gin.Context) fault.Target {
	action := c.Request.Method
	if route := c.FullPath(); route != "" {
		action += " " + route
	} else {
		action += " " + c.Request.URL.Path
	}
	return fault.Target{Service: "gin", Action: action}
}
