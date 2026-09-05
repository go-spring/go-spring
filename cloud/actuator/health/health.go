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

// Package health defines the bean a component contributes to report its own
// health to operational tooling (K8s readiness probes, registry health
// checks, ops dashboards): a component that can report its health (a database
// pool, a cache client, a message-queue connection) builds an [Indicator]
// bean, and a collector (e.g. starter-actuator) autowires the whole set and
// aggregates them. How the results are exposed and how often they are polled
// stay with the collector.
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

	// StatusDegraded is an aggregate-only verdict: at least one optional
	// component is DOWN while every critical component is UP. The application
	// can still serve traffic, so a readiness probe should NOT fail; the
	// failure stays visible in the per-component detail instead. A single
	// component is never DEGRADED; only a collector aggregating several
	// indicators can produce this status.
	StatusDegraded Status = "DEGRADED"
)

// Group identifies which Kubernetes probe an indicator contributes to. The
// three groups mirror the container lifecycle probes; a collector consults a
// group's indicators for the matching probe endpoint.
type Group string

const (
	// GroupLiveness is checked by the liveness probe. Indicators in this
	// group should test only that the process itself is functioning, never
	// an external dependency, because a liveness failure restarts the pod.
	// Most applications register nothing here (liveness = process is up).
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

// Indicator reports one component's health. It is exported as a bean; the
// collector (e.g. starter-actuator) autowires every [Indicator] bean and
// aggregates them.
type Indicator struct {
	// Name is a short, stable identifier for this component (e.g.
	// "redis:cache", "mysql:orders"). It is the key under which the
	// component's status is reported, so it should be unique within an
	// application.
	Name string

	// Probe reports whether the component is currently usable: nil when
	// healthy, a non-nil error describing the failure otherwise. It must
	// honor ctx (deadline/cancellation) so a slow dependency cannot stall a
	// probe.
	Probe func(ctx context.Context) error

	// Groups are the Kubernetes probe groups this indicator contributes to.
	// Empty means "no opinion": the collector applies its default
	// (conventionally readiness + startup, never liveness), so a dependency
	// check cannot trigger a pod restart.
	Groups []Group

	// Optional marks a degraded-but-tolerable dependency (e.g. an optional
	// cache): its DOWN result is still reported per-component but does not
	// take the pod out of rotation.
	Optional bool
}
