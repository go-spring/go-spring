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

// Package mesh holds the service-mesh switch.
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
// Mesh mode is a fixed trait of a deployment, so the environment — not runtime
// config or code — is its natural carrier. [Enabled] resolves the GS_MESH
// environment variable:
//
//   - "on"  — forced on (a sidecar owns discovery + load balancing).
//   - "off" — forced off (the app's client-side discovery/LB stays active).
//   - "auto" or unset — on iff a sidecar is detected ([Detect]).
//
// The package is a pure leaf (stdlib only): no logging, no config binding.
package mesh

import (
	"os"
	"strings"
	"sync"
)

// ModeEnv is the environment variable that selects mesh mode. See the package
// doc for the accepted values ("on", "off", "auto"/unset).
const ModeEnv = "GS_MESH"

// Enabled reports whether mesh mode is currently on. It resolves [ModeEnv]:
// "on"/"off" force the answer; any other value (including unset and "auto")
// infers it from sidecar-injected environment variables via [Detect]. The value
// is matched case-insensitively after trimming surrounding whitespace.
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(ModeEnv))) {
	case "on":
		return true
	case "off":
		return false
	default: // unset, "auto", or unrecognized — infer from the environment.
		return Detect()
	}
}

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
// The result is computed once and cached ([sync.OnceValue]): the environment
// does not change during a process's lifetime, and scanning os.Environ on every
// call (the "auto" default of [Enabled]) would walk the whole environment per
// lookup. Tests that mutate env vars call the package-internal resetDetect to
// drop the cache.
//
// It backs the "auto" mode of [Enabled]: when GS_MESH is unset or "auto", this
// inference decides whether mesh mode is on.
func Detect() bool { return detectOnce() }

var detectOnce = sync.OnceValue(detectEnv)

// detectEnv is the uncached body of [Detect].
func detectEnv() bool {
	for _, kv := range os.Environ() {
		for _, p := range envPrefixes {
			if strings.HasPrefix(kv, p) {
				return true
			}
		}
	}
	return false
}

// resetDetect drops [Detect]'s cache. Test-only: it exists because tests mutate
// the process environment, which the cache would otherwise hide.
func resetDetect() { detectOnce = sync.OnceValue(detectEnv) }
