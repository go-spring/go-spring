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

	// HealthGroups returns the Kubernetes probe groups this indicator
	// contributes to. An empty slice means "no opinion": the collector applies
	// its default, conventionally readiness + startup - never liveness, so a
	// dependency check cannot trigger a pod restart. Use [WithGroups] to declare
	// explicit groups.
	HealthGroups() []Group

	// IsCritical reports whether a DOWN result from this indicator should fail
	// the aggregate probe. A non-critical indicator is still reported
	// per-component but does not take the pod out of rotation; use it for a
	// degraded-but-tolerable dependency such as an optional cache.
	IsCritical() bool
}

// indicator is the shared implementation behind NewIndicator. It holds the
// component name, the probe function, the explicit probe groups (nil when
// unset), and the critical flag (true by default). Its methods are trivial
// accessors; routing and severity are the collector's call.
type indicator struct {
	name     string
	probe    func(ctx context.Context) error
	groups   []Group
	critical bool
}

func (i *indicator) HealthName() string { return i.name }

func (i *indicator) CheckHealth(ctx context.Context) error { return i.probe(ctx) }

func (i *indicator) IsCritical() bool { return i.critical }

func (i *indicator) HealthGroups() []Group { return i.groups }

// IndicatorOption customizes an indicator built by NewIndicator.
type IndicatorOption func(*indicator)

// NonCritical marks the indicator as non-critical: a DOWN result is still
// reported per-component but does not fail the aggregate probe. Use it for a
// degraded-but-tolerable dependency that should not take the pod out of
// rotation.
func NonCritical() IndicatorOption {
	return func(i *indicator) { i.critical = false }
}

// WithGroups declares which probe groups the indicator contributes to. When
// omitted, HealthGroups returns nil and the collector applies its default
// (conventionally readiness + startup, never liveness).
func WithGroups(groups ...Group) IndicatorOption {
	return func(i *indicator) { i.groups = groups }
}

// NewIndicator builds an Indicator from a name and a probe function, replacing
// the near-identical little "{ name string; client X }" structs that each
// client starter would otherwise declare just to satisfy the interface.
//
// The probe should perform a bounded check (honoring ctx) and return nil when
// the component is usable. Without options the indicator is critical and
// declares no explicit groups, so the collector applies its default routing
// (conventionally readiness + startup, never liveness).
//
// Example:
//
//	ind := health.NewIndicator("redis:"+name, func(ctx context.Context) error {
//	    return client.Ping(ctx).Err()
//	})
func NewIndicator(name string, probe func(ctx context.Context) error, opts ...IndicatorOption) Indicator {
	ind := indicator{name: name, probe: probe, critical: true}
	for _, opt := range opts {
		opt(&ind)
	}
	return &ind
}
