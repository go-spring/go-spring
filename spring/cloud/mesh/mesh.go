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

// Package mesh holds the process-global service-mesh switch.
//
// When a sidecar (Istio/Envoy, Linkerd, ...) is injected it already does
// discovery and load balancing at L4/L7. Leaving the application's own
// client-side discovery and load balancing on top of that means traffic is
// balanced twice, topology awareness and outlier ejection fight the mesh, and
// failure-domain decisions get confused. A starter that supports mesh reads
// [Enabled] and, when it is on, connects straight to the service's stable DNS
// address (letting the sidecar balance) instead of building a discovery
// [Resolver] or a load-balance Pool.
//
// This switch lives in its own package — not in discovery — because "am I in a
// mesh?" is a deployment/infra question, orthogonal to name resolution.
package mesh

import (
	"os"
	"strings"
	"sync/atomic"
)

// enabled is the process-global service-mesh switch.
var enabled atomic.Bool

// SetEnabled turns mesh mode on or off for the whole process. It is intended to
// be set once at startup, before any client builds, from process-level infra
// config such as ${spring.mesh.enabled}.
func SetEnabled(on bool) { enabled.Store(on) }

// Enabled reports whether mesh mode is currently on.
func Enabled() bool { return enabled.Load() }

// envPrefixes are environment-variable name prefixes injected into a workload
// container by common service meshes. Their presence is a reliable,
// side-effect-free signal that a sidecar is already handling discovery and load
// balancing.
var envPrefixes = []string{
	"ISTIO_META_",     // Istio / Envoy
	"LINKERD2_PROXY_", // Linkerd
}

// Detect reports whether the process appears to be running inside a service
// mesh, inferred from sidecar-injected environment variables. It performs no
// network I/O and is safe to call at startup.
//
// It backs the "auto" mesh mode: an explicit ${spring.mesh.enabled}=true|false
// stays the single source of truth (see [SetEnabled]); auto consults this
// inference only when the operator has not decided.
func Detect() bool {
	for _, kv := range os.Environ() {
		for _, p := range envPrefixes {
			if strings.HasPrefix(kv, p) {
				return true
			}
		}
	}
	return false
}
