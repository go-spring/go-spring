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

// Group identifies which Kubernetes probe an indicator contributes to. The
// three groups mirror the container lifecycle probes; a collector consults a
// group's indicators for the matching probe endpoint.
type Group string

const (
	// GroupLiveness is checked by the liveness probe. Indicators in this group
	// should test only that the process itself is functioning — never an
	// external dependency — because a liveness failure restarts the pod. Most
	// applications register nothing here (liveness = process is up).
	GroupLiveness Group = "liveness"

	// GroupReadiness is checked by the readiness probe: whether the app can
	// currently serve traffic. Dependency indicators (database, cache, ...)
	// belong here so a degraded dependency removes the pod from Service
	// endpoints without restarting it.
	GroupReadiness Group = "readiness"

	// GroupStartup is checked by the startup probe: whether the app has finished
	// starting. Dependency indicators that must be reachable before the app is
	// considered started belong here.
	GroupStartup Group = "startup"
)

// Grouped is an optional interface an Indicator may implement to declare which
// probe groups it contributes to. An indicator that does not implement it is
// treated as belonging to GroupReadiness and GroupStartup (dependency health),
// never GroupLiveness — the safe default, since a dependency check must not be
// able to trigger a liveness restart.
type Grouped interface {
	// HealthGroups returns the probe groups this indicator contributes to.
	HealthGroups() []Group
}

// GroupsOf returns the probe groups an indicator contributes to, applying the
// default (readiness + startup) when the indicator does not implement Grouped.
func GroupsOf(ind Indicator) []Group {
	if g, ok := ind.(Grouped); ok {
		return g.HealthGroups()
	}
	return []Group{GroupReadiness, GroupStartup}
}

// InGroup reports whether an indicator contributes to the given probe group.
func InGroup(ind Indicator, group Group) bool {
	for _, g := range GroupsOf(ind) {
		if g == group {
			return true
		}
	}
	return false
}

// Critical is an optional interface an Indicator may implement to declare
// whether its failure should fail the aggregate probe.
//
// An indicator that does not implement it is treated as critical: any DOWN
// critical indicator flips the probe to DOWN (503), so Kubernetes removes the
// pod from Service endpoints. A non-critical indicator's status is still
// reported for observability, but a DOWN result does not lower the aggregate —
// use it for a degraded-but-tolerable dependency (an optional cache, a
// best-effort downstream) that should not take the pod out of rotation.
type Critical interface {
	// IsCritical reports whether a failure of this indicator should fail the
	// aggregate probe.
	IsCritical() bool
}

// IsCritical reports whether a DOWN result from ind should lower the aggregate
// probe status, applying the default (true) when ind does not implement
// [Critical].
func IsCritical(ind Indicator) bool {
	if c, ok := ind.(Critical); ok {
		return c.IsCritical()
	}
	return true
}
