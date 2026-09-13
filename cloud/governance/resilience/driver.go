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

// driver.go holds the pluggable-backend seam of the resilience package: the
// [Driver] interface and the bundled "default" implementation. Production
// drivers (e.g. sentinel-golang) live in their own modules and are contributed
// as NAMED beans: the container is the driver directory, and the governance
// wiring bean collects them into a map keyed by bean name, which the center
// looks the configured driver name up in. The runtime surface (Executor + its
// default impl, Policy, breaker types) lives in executor.go and resilience.go.
//
// This package holds no registry of its own: it stays container-free and
// dependency-free, so it is usable from any runtime. Selecting a driver by name
// is the caller's job — see the discovery package for the same shape.

package resilience

import (
	"sort"

	"go-spring.org/stdlib/errutil"
)

// DefaultDriverName is the name the bundled [NewDefaultDriver] backend answers
// to, and the name the governance center falls back to when its driver
// directory holds no entry for the configured name.
const DefaultDriverName = "default"

// Resolve picks the backend named name out of dir, the name-keyed directory of
// backends the container provided (see the discovery package for the same
// shape). The empty name and defaultName both resolve to fallback, so a
// process that contributes no backend still runs on the bundled one. A name
// matching nothing is an error listing what IS available, so a typo in a
// configured backend name is diagnosable instead of silently ignored. kind
// labels that error; dir may be nil.
func Resolve[T any](dir map[string]T, name, defaultName, kind string, fallback T) (T, error) {
	if d, ok := dir[name]; ok {
		return d, nil
	}
	if name == "" || name == defaultName {
		return fallback, nil
	}
	available := []string{defaultName}
	for k := range dir {
		if k != defaultName {
			available = append(available, k)
		}
	}
	sort.Strings(available)
	var zero T
	// defaultName is already in the list, so the name that missed is the only
	// one absent from it.
	return zero, errutil.Explain(nil, "resilience: no %s named %q (available: %v)", kind, name, available)
}

// Driver builds an [Executor] from a [Policy]. Backends implement it and are
// contributed to the container as a bean named after the backend (e.g.
// "sentinel"), exported as a [Driver] so name-keyed directory injection finds
// them.
type Driver interface {
	NewExecutor(Policy) (Executor, error)
}

// NewDefaultDriver returns the bundled driver: a self-contained [Executor]
// builder with no third-party dependencies, so the framework is usable out of
// the box and in tests. Production deployments select a richer driver (for
// example sentinel-golang, in its own module) purely by changing the configured
// driver name — the [Executor] seam and every adapter stay put.
func NewDefaultDriver() Driver { return defaultDriver{} }

// defaultDriver is the bundled [Driver]: it builds a [defaultExecutor] from a
// [Policy]. The executor itself (per-resource limiter/breaker/bulkhead state +
// the Execute loop) lives in executor.go.
type defaultDriver struct{}

func (defaultDriver) NewExecutor(p Policy) (Executor, error) {
	if p.RateLimit < 0 {
		return nil, errutil.Explain(nil, "resilience: negative rate limit %v", p.RateLimit)
	}
	return &defaultExecutor{policy: p, states: map[string]*resourceState{}}, nil
}
