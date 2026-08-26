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
// [Driver] interface, the name-keyed registry backends register into via
// [RegisterDriver]/[GetDriver], and the bundled "default" [Driver] implementation
// (a self-contained, zero-dependency [Executor] builder). Production drivers
// (e.g. sentinel-golang) live in their own modules and register on blank import.
// The runtime surface (Executor + its default impl, Policy, breaker types) lives
// in executor.go and resilience.go.

package resilience

import (
	"fmt"
	"sort"
	"sync"
)

var (
	mu       sync.RWMutex
	registry = map[string]Driver{}
)

// registerInto is the shared body of the Driver and LimiterDriver
// registrations: the empty-name / nil-driver / duplicate-name panic triple and
// the guarded map insert. kind only feeds the panic messages. The registries
// stay separate maps with separate interfaces (no type merging) — this only
// removes the copy-pasted guard boilerplate.
func registerInto[T any](kind string, m map[string]T, mu *sync.RWMutex, name string, v T) {
	if name == "" {
		panic("resilience: register " + kind + " with empty name")
	}
	if any(v) == nil {
		panic("resilience: register nil " + kind + " for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := m[name]; ok {
		panic("resilience: " + kind + " already registered: " + name)
	}
	m[name] = v
}

// The bundled "default" driver: a self-contained implementation with no
// third-party dependencies, so the framework is usable out of the box and in
// tests. Production deployments select the recommended sentinel-golang driver
// (separate module, registers itself as "sentinel" on blank import) purely by
// changing the driver name — the [Executor] seam and every adapter stay put.
func init() { RegisterDriver("default", defaultDriver{}) }

// Driver builds an [Executor] from a [Policy]. Backends implement it and
// register under a name via [RegisterDriver].
type Driver interface {
	NewExecutor(Policy) (Executor, error)
}

// defaultDriver is the bundled [Driver]: it builds a [defaultExecutor] from a
// [Policy]. The executor itself (per-resource limiter/breaker/bulkhead state +
// the Execute loop) lives in executor.go.
type defaultDriver struct{}

func (defaultDriver) NewExecutor(p Policy) (Executor, error) {
	if p.RateLimit < 0 {
		return nil, fmt.Errorf("resilience: negative rate limit %v", p.RateLimit)
	}
	return &defaultExecutor{policy: p, states: map[string]*resourceState{}}, nil
}

// RegisterDriver makes a [Driver] available under name. It panics if name is
// empty, d is nil, or name is already registered, mirroring the driver-registry
// idiom used elsewhere (discovery.Register, starter-go-redis RegisterDriver) so
// duplicate wiring fails loudly at init.
func RegisterDriver(name string, d Driver) {
	registerInto("driver", registry, &mu, name, d)
}

// GetDriver returns the [Driver] registered under name, or an error that lists
// the available drivers when none matches.
func GetDriver(name string) (Driver, error) {
	mu.RLock()
	defer mu.RUnlock()
	d, ok := registry[name]
	if !ok {
		names := make([]string, 0, len(registry))
		for k := range registry {
			names = append(names, k)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("resilience: no driver registered as %q (registered: %v)", name, names)
	}
	return d, nil
}

// NewExecutor resolves the [Driver] registered under name and builds an
// [Executor] from p in one step — the convenience most callers want instead of
// the two-step GetDriver + Driver.NewExecutor (with its two error checks). It
// returns the lookup or build error; callers that need the [Driver] for
// something beyond building an executor still use [GetDriver] directly.
func NewExecutor(name string, p Policy) (Executor, error) {
	drv, err := GetDriver(name)
	if err != nil {
		return nil, err
	}
	return drv.NewExecutor(p)
}
