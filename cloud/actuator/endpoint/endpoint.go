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

import "net/http"

// Endpoint is an HTTP handler mounted on the actuator's management port.
type Endpoint struct {
	// Pattern is the ServeMux mount pattern, e.g. "/metrics" or "GET /env"
	// (method-restricted). It must not collide with the actuator's built-in
	// patterns (/healthz, /readyz, /info, ...) or with another endpoint; a
	// duplicate pattern panics at startup.
	Pattern string

	// Handler serves requests to Pattern. An endpoint that exists is served:
	// whether it is contributed at all is the contributor's own enable
	// switch, and access control is the management port's business (the
	// actuator's authentication guard), not per-endpoint filtering.
	Handler http.Handler
}
