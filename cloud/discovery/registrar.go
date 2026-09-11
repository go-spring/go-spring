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

import "context"

// Instance is the process advertisement every registrar publishes: one
// physical instance of a named service. It is the write-side mirror of
// [Endpoint] — the registry-center starters fill it from the global
// ${spring.registry} identity block, so every backend registers the SAME
// instance content (a multi-registry setup is one publication fanned out to
// several centers, not several publications).
type Instance struct {
	// ServiceName is the service this instance belongs to; consumers resolve
	// endpoints by this name.
	ServiceName string

	// ID identifies this instance within the service; empty lets the backend
	// derive one (e.g. from the address).
	ID string

	// Addr is the instance's host:port as consumers dial it.
	Addr string

	// Weight is the load-balancing weight handed to consumers; 0 means
	// "drained" under the repo-wide Weight=0 drain semantics.
	Weight int

	// Metadata carries backend-agnostic attributes (zone, version, ...). Each
	// registry backend stores it the way its protocol allows and hands it back
	// through [Endpoint.Metadata].
	Metadata map[string]string
}

// Registrar publishes one [Instance] into ONE registry center. Each backend
// starter derives one Registrar per configured block; the starter-registry
// core collects them all and drives them through a single lifecycle —
// register on app-ready, deregister on shutdown, broadcast weight changes.
// Implementations must make Deregister and UpdateWeight idempotent.
type Registrar interface {
	// Register publishes inst into this center. It is called once, after the
	// application is ready.
	Register(ctx context.Context, inst Instance) error

	// Deregister removes inst from this center. It is called on shutdown and
	// must be safe to call again (deregister fallbacks).
	Deregister(ctx context.Context, inst Instance) error

	// UpdateWeight re-advertises inst with a new weight without the instance
	// leaving discovery.
	UpdateWeight(ctx context.Context, inst Instance, weight int) error
}
