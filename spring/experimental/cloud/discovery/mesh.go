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

package discovery

import (
	"context"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

// meshEnabled is the process-global service-mesh switch. It is read at the
// dialer factory points ([NewLiveDialer], [NewClientDialer]) and by the
// load-balancing Pool, so the degradation happens once, centrally, instead of
// every client starter having to test it.
var meshEnabled atomic.Bool

// SetMeshMode turns service-mesh mode on or off for the whole process.
//
// When a sidecar (Istio/Envoy, Linkerd, ...) is injected it already does
// discovery and load balancing at L4/L7. Leaving the application's own
// client-side discovery and load balancing on top of that means traffic is
// balanced twice, topology awareness and outlier ejection fight the mesh, and
// failure-domain decisions get confused. Turning mesh mode on degrades both
// layers to a pass-through so the sidecar owns those concerns.
//
// It is intended to be set once at startup (before any client builds its
// dialer) from process-level infra config such as ${spring.mesh.enabled}.
func SetMeshMode(enabled bool) { meshEnabled.Store(enabled) }

// MeshMode reports whether service-mesh mode is currently on.
func MeshMode() bool { return meshEnabled.Load() }

// newMeshDialer builds a [LiveDialer] degraded for mesh mode. It never resolves
// or watches a discovery backend and never rotates: it exposes a single stable
// endpoint whose address is the service name itself. In Kubernetes that name
// resolves via DNS to the Service ClusterIP, which the sidecar intercepts and
// load-balances across the real pods — so the application connects to one
// stable target and its own discovery/LB effectively disappear, while the
// caller sees an ordinary LiveDialer and needs no special casing.
func newMeshDialer(name string) *LiveDialer {
	ld := &LiveDialer{name: name}
	eps := []Endpoint{{Addr: name, Healthy: true}}
	ld.eps.Store(&eps)
	return ld
}

// NewClientDialer is the centralized factory a client starter uses to obtain a
// dialer for a logical service name; it is where the mesh switch is read on the
// discovery side.
//
// In normal mode it resolves the named discovery backend and returns a
// [LiveDialer] over its live endpoints (equivalent to MustGet + NewLiveDialer).
// In mesh mode it skips the backend entirely — not even requiring one to be
// registered — and returns a pass-through dialer to the stable service address,
// letting the sidecar do discovery and load balancing.
func NewClientDialer(ctx context.Context, backend, name string) (*LiveDialer, error) {
	if MeshMode() {
		return newMeshDialer(name), nil
	}
	d, err := MustGet(backend)
	if err != nil {
		return nil, err
	}
	return NewLiveDialer(ctx, d, name)
}

// meshEnvPrefixes are environment-variable name prefixes injected into a
// workload container by common service meshes. Their presence is a reliable,
// side-effect-free signal that a sidecar is already handling discovery and load
// balancing, so the application's own client-side stack should degrade.
var meshEnvPrefixes = []string{
	"ISTIO_META_",     // Istio / Envoy
	"LINKERD2_PROXY_", // Linkerd
}

// DetectMesh reports whether the process appears to be running inside a service
// mesh, inferred from sidecar-injected environment variables. It performs no
// network I/O and is safe to call at startup.
//
// It backs the "auto" mesh mode: the explicit ${spring.mesh.enabled}=true|false
// stays the single source of truth (see [SetMeshMode]); auto consults this
// inference only when the operator has not decided.
func DetectMesh() bool {
	for _, kv := range os.Environ() {
		for _, p := range meshEnvPrefixes {
			if strings.HasPrefix(kv, p) {
				return true
			}
		}
	}
	return false
}

// TraceInjector writes the trace context carried by ctx into an outbound
// request's headers so the callee — and any mesh sidecar (Istio/Envoy) on the
// path — joins the same distributed trace instead of starting a new one.
type TraceInjector func(ctx context.Context, header http.Header)

// traceInjector holds the process-wide injector. It is a seam: stdlib stays free
// of an OpenTelemetry dependency and starter-otel fills it with an injector
// backed by the global OTel propagator (W3C traceparent + B3), mirroring how it
// installs log.FieldsFromContext for trace<->log correlation.
var traceInjector atomic.Pointer[TraceInjector]

// SetTraceInjector installs the process-wide trace injector; pass nil to clear
// it. It is intended to be called once at startup by the tracing owner
// (starter-otel), before any client issues an outbound request.
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
// application's spans and the mesh sidecar's spans stay on one trace across a
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
