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

package StarterOTel

import (
	"context"
	"net/http"
	"sync/atomic"
)

// TraceInjector writes the trace context carried by ctx into an outbound
// request's headers so the callee — and any mesh sidecar (Istio/Envoy) on the
// path — joins the same distributed trace instead of starting a new one.
//
// It is a seam: starter-otel owns the OpenTelemetry propagator and installs the
// real injector via [SetTraceInjector] at startup (see setupTrace). HTTP
// starters that want outbound trace propagation wrap their transport with
// [TraceRoundTripper].
type TraceInjector func(ctx context.Context, header http.Header)

// traceInjector holds the process-wide injector installed by setupTrace.
var traceInjector atomic.Pointer[TraceInjector]

// SetTraceInjector installs the process-wide trace injector; pass nil to clear
// it. It is intended to be called once at startup (by setupTrace, with the
// global OTel propagator — W3C traceparent + B3), before any client issues an
// outbound request.
func SetTraceInjector(inj TraceInjector) {
	if inj == nil {
		traceInjector.Store(nil)
		return
	}
	traceInjector.Store(&inj)
}

// InjectTrace writes the current trace context into header using the installed
// injector. It is a no-op when none is set (e.g. tracing disabled), so callers
// can invoke it unconditionally.
func InjectTrace(ctx context.Context, header http.Header) {
	if p := traceInjector.Load(); p != nil {
		(*p)(ctx, header)
	}
}

// TraceRoundTripper returns an [http.RoundTripper] that stamps the current trace
// context onto every outbound request before delegating to base, so the
// application's spans and any mesh sidecar's spans stay on one trace across a
// hop. When base is nil, [http.DefaultTransport] is used; when no injector is
// installed it is a transparent pass-through.
func TraceRoundTripper(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return traceRoundTripper{base: base}
}

type traceRoundTripper struct{ base http.RoundTripper }

func (t traceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone before mutating headers: a RoundTripper must not modify the caller's
	// request (net/http contract).
	r2 := req.Clone(req.Context())
	InjectTrace(r2.Context(), r2.Header)
	return t.base.RoundTrip(r2)
}
