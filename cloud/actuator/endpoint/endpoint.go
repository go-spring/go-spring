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

// Package endpoint defines the bean a component contributes to mount an HTTP
// handler on the actuator's management port: any *Endpoint bean is collected
// by the actuator and served on its address, next to the built-in probe
// endpoints.
package endpoint

import (
	"net/http"
	"sync/atomic"
)

// Endpoint is an HTTP handler mounted on the actuator's management port.
type Endpoint struct {
	// Pattern is the ServeMux mount pattern, e.g. "/metrics" or "GET /env"
	// (method-restricted). It must not collide with the actuator's built-in
	// patterns (/healthz, /readyz, /info, ...) or with another endpoint; a
	// duplicate pattern panics at startup. The endpoint's filter name — what
	// spring.actuator.endpoints.include matches — is the pattern's path
	// without the leading slash (and without any method prefix).
	Pattern string

	// Handler serves requests to Pattern.
	Handler http.Handler

	// Sensitive marks the endpoint as exposing configuration or internals. A
	// sensitive endpoint registers only when explicitly listed in
	// spring.actuator.endpoints.include; a non-sensitive one is default-on
	// (still subject to the include whitelist when that list is non-empty).
	// The contributor knows what its endpoint exposes, so sensitivity is
	// declared here, not adjudicated by the actuator.
	Sensitive bool
}

// serving records whether a management server that collects [Endpoint] beans
// is linked into the process, so contributors can warn about endpoints that
// would otherwise be mounted nowhere.
var serving atomic.Bool

// MarkServing records that a management server collecting [Endpoint] beans is
// present. Called by the actuator, never by contributors.
func MarkServing() { serving.Store(true) }

// IsServing reports whether a management server is present. False means
// contributed Endpoint beans have nowhere to mount; a contributor that is
// only reachable through this seam should WARN.
func IsServing() bool { return serving.Load() }
