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
// A sidecar (Istio/Envoy, Linkerd, ...) already does discovery and load
// balancing, so the application's own client-side discovery and load balancing
// must not run on top of it — traffic would be balanced twice and the two
// layers would fight over outlier ejection and failure-domain decisions.
// A starter that supports mesh reads [Enabled] and, when it is on, dials the
// service's stable DNS address (letting the sidecar balance) instead of
// building a discovery Resolver or a load-balance Pool.
//
// [Enabled] resolves the GS_MESH environment variable:
//
//   - "on"  — forced on.
//   - "off" — forced off.
//   - "auto" or unset — on iff a sidecar is detected ([Detect]).
//
// The package is a pure leaf (stdlib only): no logging, no config binding.
package mesh

import (
	"os"
	"strings"
)

// ModeEnv is the environment variable that selects mesh mode. See the package
// doc for the accepted values ("on", "off", "auto"/unset).
const ModeEnv = "GS_MESH"

// envPrefixes are environment-variable name prefixes injected into a workload
// container by common service meshes; their presence signals a sidecar.
var envPrefixes = []string{
	"ISTIO_META_",     // Istio / Envoy
	"LINKERD2_PROXY_", // Linkerd
}

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

// Detect reports whether the process appears to be running inside a service
// mesh, inferred from sidecar-injected environment variables. It performs no
// network I/O and is safe to call at startup. It backs the "auto" mode of
// [Enabled].
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
