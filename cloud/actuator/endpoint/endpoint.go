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

// Package endpoint defines a framework-agnostic, zero-dependency seam for
// contributing an operational HTTP handler to a management server.
//
// It lets a component surface an endpoint on the single management port owned
// by the actuator (Prometheus /metrics, a build-info page, ...) without the
// actuator having to import that component: the actuator autowires every bean
// exported as [Endpoint] and mounts each on its mux, exactly as it collects
// health indicators. The seam lives in the zero-dependency foundation layer so
// both the contributor (e.g. starter-otel) and the collector (starter-actuator)
// depend only on this package, never on each other.
//
// This mirrors health.Indicator: a small interface any starter can implement to
// plug into the actuator with no cross-module dependency beyond this package.
package endpoint

import (
	"net/http"
	"sync/atomic"
)

// Endpoint is an operational HTTP handler contributed to the actuator's
// management server by another module.
//
// Implementations must be safe for concurrent use: the management server serves
// requests concurrently.
type Endpoint interface {
	// Path is the HTTP path the handler is mounted at (e.g. "/metrics"). It
	// should be distinct from the actuator's built-in paths (/health,
	// /readiness, /info) and from other contributed endpoints.
	Path() string

	// http.Handler serves requests to Path.
	http.Handler
}

// serving is flipped by the management server (starter-actuator) at init. A
// contributor that relies on the actuator to serve its endpoint (e.g.
// starter-otel's Prometheus /metrics with metrics.port=0) can then WARN at
// startup when nothing will actually mount it - instead of running with a
// silently homeless endpoint.
var serving atomic.Bool

// MarkServing records that a management server collecting [Endpoint] beans is
// linked into the process. The actuator calls it from an init; contributors
// never do.
func MarkServing() { serving.Store(true) }

// IsServing reports whether a management server is present. False means
// contributed Endpoint beans have nothing to mount them.
func IsServing() bool { return serving.Load() }
