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
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/guard"
	"go-spring.org/cloud/label"
	"go-spring.org/cloud/observability"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// buildServerPolicy builds the inbound middleware. The resilience
// executor is built from the injected [guard.Manager], so this server gets
// its rate-limit / bulkhead / breaker limits from the governance document's SERVER
// block (spring.governance.server.*)
// WITHOUT naming *governance.Center. A nil manager — a standalone call, or an app
// that does not import — is normalized to an unarmed one,
// whose executor is a transparent pass-through, so the inbound middleware runs
// but never rejects (next runs once, untouched). The executor handle resolves its
// backing implementation per call and follows the manager's hot-reload, so an
// operator can tighten inbound without a restart, the same way every
// outbound client's policy is tuned.
//
// The label is "echo:<address>", the same one the fault middleware's sibling
// tables use for this server, so one governance rule covers the whole inbound side.
func buildServerPolicy(cfg Config, center *governance.Center) echo.MiddlewareFunc {
	service := label.ServiceLabel("echo", cfg.Address)
	return resilienceServerPolicy(center.InboundFor("echo", service), service, observeEnabled(cfg))
}

// resilienceServerPolicy is the inbound middleware: each request runs
// through exec so the configured rate-limit / bulkhead / breaker limits are
// enforced before the handler chain. Rejects map to 429 (rate/bulkhead) or 503
// (circuit open); a handler error or a committed 5xx counts as a failure for the
// breaker.
//
// Inbound inbound must NOT retry — a handler that has already produced side
// effects cannot be replayed (inbound serving is not idempotent). Leave
// [guard.Policy] has no retry field at all, so the executor built from the
// server block has no retry stage; the committed guard also prevents reentry.
func resilienceServerPolicy(in *governance.Inbound, service string, observe bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			var served bool
			var handlerErr error
			// run executes the handler chain. h is the split-form observation when
			// observe is on (so the response half can be recorded), nil otherwise.
			run := func(ctx context.Context, h *governance.Hooks) error {
				if served {
					return nil // reentry guard: handler already ran this request
				}
				// Hand the observed context to the handler: the object derived it,
				// so the span the handler and its outbound clients run under is the
				// one this request produced.
				c.SetRequest(c.Request().WithContext(ctx))
				handlerErr = next(c)
				served = c.Response().Committed || handlerErr != nil
				if h != nil {
					// The response half: the status code is the outcome of the work,
					// so it exists only now. Recorded here, read back by the observer.
					h.Observation().Response().Add(
						attribute.Int("http.response.status_code", c.Response().Status))
				}
				if handlerErr != nil {
					return handlerErr
				}
				if c.Response().Status >= 500 {
					return errHTTP5xx{code: c.Response().Status}
				}
				return nil
			}

			if observe {
				// Declare what this request IS — the observer names the span, the
				// HTTP family metrics and the access log from it. The route is
				// known here: echo resolves it before the handler chain runs. The
				// split form is used so the handler's response half can be recorded.
				h, err := in.Before(ctx, operation(c))
				if err != nil {
					return renderInjectedOrRefusal(c, err)
				}
				runErr := run(h.Context(), h)
				in.After(h, runErr)
			} else {
				// Observability off: governance still runs, silently.
				if gErr := in.GovernedRun(ctx, operation(c), func(ctx context.Context) error { return run(ctx, nil) }); gErr != nil {
					var ref *governance.Refusal
					if errors.As(gErr, &ref) {
						return renderInjectedOrRefusal(c, gErr)
					}
					// A handler error is already in handlerErr and is returned below.
				}
			}

			// A handler's own error (or a committed response) belongs to the
			// caller: return it so echo's HTTPErrorHandler renders it, exactly as
			// the fault middleware lets it through.
			return handlerErr
		}
	}
}

// renderRefusal maps an inbound refusal to an echo error a client can act on; an
// infrastructure failure surfaces as an internal error.
func renderRefusal(err error) error {
	if err == nil {
		return nil
	}
	var ref *governance.Refusal
	if errors.As(err, &ref) {
		return echo.NewHTTPError(httpStatusFor(ref.Verdict()), ref.Error())
	}
	return err
}

// renderInjectedOrRefusal writes an injected fault as a plain-text 503 (the
// shape the standalone fault middleware used) and renders any other refusal as
// an echo HTTP error.
func renderInjectedOrRefusal(c echo.Context, err error) error {
	var ref *governance.Refusal
	if errors.As(err, &ref) && ref.Verdict() == guard.VerdictFaultInjected {
		return c.String(http.StatusServiceUnavailable, "service unavailable")
	}
	return renderRefusal(err)
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

// observeEnabled reports whether this server DECLARES its requests, which is what
// puts the request span, the HTTP family metrics and the access log on them (the
// resilience executor emits all three from the declaration).
//
// The single switch is [ObservabilityConfig]; the three per-signal switches it
// replaced are still read, so an existing configuration keeps working — but
// their granularity is gone, because the signals are one set now. Turning any of
// them off therefore turns the set off, and says so once: a server that quietly
// ignored an operator's "no metrics for this route" would be worse than one that
// reports it cannot honour the split any more.
func observeEnabled(cfg Config) bool {
	if !cfg.Middleware.Observability.Enabled {
		return false
	}
	m := cfg.Middleware
	if !m.Tracing.Enabled || !m.Metrics.Enabled || !m.AccessLog.Enabled {
		log.Warn(context.Background(), log.TagAppDef,
			"echo: middleware.tracing/metrics/accessLog 的按信号开关已合并 —— 三者现在是一组,"+
				"任一为 false 即整组关闭(span/指标/访问日志同生共死);请改用 middleware.observability.enabled")
		return false
	}
	return true
}

// operation is the semantic identity of one inbound request, declared on the
// context before the inbound executor runs. It is this package's WHOLE
// contribution to the request's signals: the emitter names the span, the family
// metrics and the access log from it.
//
// The route is the span's word — echo resolves it before this middleware runs —
// and it is bounded, so it labels a metric alongside the method. The raw path,
// the host and the scheme are per-request and open-ended, so they are Detail:
// span and log only. The one thing not known here is the response status, which
// the handler records as it answers (see resilienceServerPolicy).
func operation(c echo.Context) observability.ServerOperation {
	r := c.Request()
	route := c.Path()
	return observability.ServerOperation{
		Action: actionName(c),
		Observe: observability.ServerObserve{
			Metric: observability.ServerMetric{
				Name: "http.server",
				Attrs: []attribute.KeyValue{
					attribute.String("http.request.method", r.Method),
					attribute.String("http.route", route),
				},
			},
			Span: observability.ServerSpan{Attrs: []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
				attribute.String("url.path", r.URL.Path),
				attribute.String("server.address", r.Host),
				attribute.String("url.scheme", scheme(r)),
			}},
			Log: observability.ServerLog{
				Tag:          accessLogTag,
				QuietSuccess: true,
				Fields: []log.Field{
					log.String("http.request.method", r.Method),
					log.String("http.route", route),
					log.String("url.path", r.URL.Path),
					log.String("server.address", r.Host),
					log.String("url.scheme", scheme(r)),
				},
			},
		},
	}
}

// actionName names the inbound call the way an interface-precise rule needs it:
// "METHOD /route", falling back to the concrete path when no route was resolved
// (a 404). It is spelled once because two readers want the same name — the
// observed operation above and the fault target the middleware builds ([fault.Target]).
func actionName(c echo.Context) string {
	r := c.Request()
	if route := c.Path(); route != "" {
		return r.Method + " " + route
	}
	return r.Method + " " + r.URL.Path
}

// scheme returns "https" when the request arrived over TLS, "http" otherwise.
func scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
