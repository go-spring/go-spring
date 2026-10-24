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
	"go-spring.org/cloud/runtime"
	"go-spring.org/cloud/runtime/dataplane/inbound/admission"
	observability "go-spring.org/cloud/runtime/dataplane/inbound/observe"
	"go-spring.org/cloud/runtime/dataplane/kernel/fault"
	"go-spring.org/cloud/runtime/dataplane/kernel/label"
)

// buildServerPolicy builds the inbound governance middleware. The inbound
// governance object comes from the injected [runtime.Runtime], so this server
// gets its rate-limit / bulkhead limits from the governance document's
// SERVER block (spring.governance.server.*). A nil center — a standalone call, or
// an app that does not import governance — is normalized inside the center to an
// unarmed authority, whose guard is a transparent pass-through, so the middleware
// runs but never rejects (the handler runs once, untouched). The object resolves
// its backing engine per call and follows the manager's hot-reload, so an operator
// can tighten inbound governance without a restart, independently of every
// outbound client's policy. The center composes the observe object around the
// guard, so refusals emit span + counter + histogram + access log.
func buildServerPolicy(cfg Config, center *runtime.Runtime) (gin.HandlerFunc, error) {
	service := label.ServiceLabel("gin", cfg.Address)
	in, err := center.InboundFor("gin", service)
	if err != nil {
		return nil, err
	}
	return resilienceServerPolicy(in), nil
}

// resilienceServerPolicy is the inbound governance middleware: each request runs
// through the inbound object so the configured rate-limit / bulkhead policy is
// enforced before the handler chain. Rate/bulkhead refusals map to 429 and an
// injected fault to 503.
//
// Inbound cannot retry — a handler that has already produced side effects cannot
// be replayed (inbound serving is not idempotent). That is structural:
// [admission.Policy] has no retry field, so the engine built from the
// server block has no retry stage at all; a Written() guard also prevents reentry
// regardless.
func resilienceServerPolicy(in *runtime.Inbound) gin.HandlerFunc {
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
		var ref *runtime.Refusal
		if errors.As(err, &ref) && !c.Writer.Written() {
			c.AbortWithStatus(httpStatusFor(ref.Verdict()))
		}
	}
}

// httpStatusFor maps an inbound refusal to the HTTP status a client can act on:
// rate/bulkhead rejections are 429, an injected fault is 503.
func httpStatusFor(kind admission.Verdict) int {
	if kind == admission.VerdictFaultInjected {
		return http.StatusServiceUnavailable
	}
	return http.StatusTooManyRequests
}

// errHTTP5xx is the failure signal a handler-emitted 5xx carries into the
// request record and the span status.
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
