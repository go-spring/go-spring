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

// Package health defines a framework-agnostic, zero-dependency abstraction for
// component health checks.
//
// It answers one question for operational tooling (K8s readiness probes,
// registry health checks, ops dashboards): "is this dependency currently
// usable?". A component that can report its health — a database pool, a cache
// client, a message-queue connection — implements the single [Indicator]
// interface and registers itself as a bean exported as [Indicator]. The
// actuator (or any other collector) autowires the whole set and aggregates
// them, without any per-component adaptation.
//
// This package deliberately says nothing about *how* the results are exposed
// (HTTP endpoints, gRPC, ...) or *when* they are polled; that stays with the
// collector. Keeping the contract this small is what lets it live in the
// zero-dependency foundation layer and be implemented by any starter without a
// cross-module import beyond stdlib.
package health

import "context"

// Status is the coarse health verdict of a component or of the aggregate.
type Status string

const (
	// StatusUp means the component is healthy and ready to serve.
	StatusUp Status = "UP"

	// StatusDown means the component is unhealthy; a readiness probe should
	// fail while any required component is down.
	StatusDown Status = "DOWN"
)

// Indicator is implemented by a component that can report its own health.
//
// Implementations must be safe for concurrent use: a collector may invoke
// CheckHealth from multiple probe requests at once.
type Indicator interface {
	// HealthName returns a short, stable identifier for this component (e.g.
	// "redis:cache", "mysql:orders"). It is used as the key under which the
	// component's status is reported, so it should be unique within an
	// application.
	HealthName() string

	// CheckHealth reports whether the component is currently usable. It returns
	// nil when healthy and a non-nil error describing the failure otherwise.
	// Implementations must honor ctx (deadline/cancellation) so a slow
	// dependency cannot stall a probe.
	CheckHealth(ctx context.Context) error
}
