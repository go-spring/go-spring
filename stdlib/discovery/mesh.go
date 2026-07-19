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
