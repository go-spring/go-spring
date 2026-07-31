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

package metric

import "net/http"

// Endpoint adapts the Prometheus scrape handler to endpoint.Endpoint so
// the actuator mounts /metrics on its management port. It is the seam that lets
// starter-otel expose metrics through the actuator without either starter
// importing the other — both depend only on stdlib.
type Endpoint struct {
	path    string
	handler http.Handler
}

// NewEndpoint creates an Endpoint that serves the Prometheus scrape handler
// at the given path.
func NewEndpoint(path string, handler http.Handler) *Endpoint {
	return &Endpoint{path: path, handler: handler}
}

// Path returns the endpoint path (e.g. "/metrics").
func (m *Endpoint) Path() string { return m.path }

// ServeHTTP delegates to the underlying Prometheus scrape handler.
func (m *Endpoint) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.handler.ServeHTTP(w, r)
}