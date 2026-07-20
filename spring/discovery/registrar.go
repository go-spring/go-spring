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

import (
	"context"
	"fmt"
	"sort"
)

// Registration describes a single instance to publish to a service registry.
// It is the write-side counterpart to the read-side [Endpoint]: once Register
// succeeds, a [Discovery] backend reading the same registry returns this
// instance as an Endpoint to its clients.
type Registration struct {
	// ServiceName is the logical name the instance is published under — the same
	// name discovery clients later pass to Discovery.Resolve / Discovery.Watch.
	ServiceName string

	// ID uniquely identifies this instance within the service. When empty a
	// Registrar derives a stable one (e.g. from ServiceName and Addr) so a
	// restart replaces the previous entry instead of accumulating duplicates.
	ID string

	// Addr is the connectable "host:port" advertised to clients.
	Addr string

	// Weight is the load-balancing weight advertised to clients; 0 means the
	// backend default.
	Weight int

	// Metadata carries backend-specific attributes (zone, unit, version, ...)
	// stored alongside the instance.
	Metadata map[string]string
}

// Registrar publishes the current instance to a service registry — the
// provider-side counterpart to [Discovery]'s client-side Resolve/Watch.
//
// It exists for deployments where the platform does not register instances for
// you: VM / bare-metal / hybrid setups that rely on an external registry
// (Nacos, Consul, Eureka, ...). In pure Kubernetes the platform already
// registers every Pod behind a Service, so a Registrar is unnecessary there —
// discover peers through starter-discovery-k8s instead.
//
// A Registrar owns whatever TTL/heartbeat renewal its backend requires:
// Register starts it and keeps the instance live until Deregister stops it.
// RPC-framework provider registration is deliberately out of scope — that stays
// bound to each framework (kitex, kratos, dubbo, ...); see starter/DESIGN §3.
//
// Implementations must be safe for concurrent use.
type Registrar interface {
	// Register publishes reg and starts the backend's keep-alive so the instance
	// stays live until Deregister. Registering the same instance again refreshes
	// the entry rather than duplicating it.
	Register(ctx context.Context, reg Registration) error

	// Deregister removes reg and stops the keep-alive started by Register. It is
	// safe to call for an instance that is not currently registered.
	Deregister(ctx context.Context, reg Registration) error
}

var registrars = map[string]Registrar{}

// RegisterRegistrar makes a Registrar backend available under name. It panics if
// name is already registered, mirroring the driver-registry idiom used for
// Discovery backends (see [Register]), so duplicate wiring fails loudly at init.
//
// It shares the package lock with the Discovery registry; both are populated
// during init/bean-registration, before any lookup, so a single mutex is enough.
func RegisterRegistrar(name string, r Registrar) {
	if name == "" {
		panic("discovery: register registrar with empty name")
	}
	if r == nil {
		panic("discovery: register nil registrar for " + name)
	}
	mu.Lock()
	defer mu.Unlock()
	if _, ok := registrars[name]; ok {
		panic("discovery: registrar already registered: " + name)
	}
	registrars[name] = r
}

// GetRegistrar returns the Registrar backend registered under name.
func GetRegistrar(name string) (Registrar, bool) {
	mu.RLock()
	defer mu.RUnlock()
	r, ok := registrars[name]
	return r, ok
}

// MustGetRegistrar returns the Registrar backend registered under name, or an
// error that lists the available backends when none matches.
func MustGetRegistrar(name string) (Registrar, error) {
	if r, ok := GetRegistrar(name); ok {
		return r, nil
	}
	mu.RLock()
	names := make([]string, 0, len(registrars))
	for k := range registrars {
		names = append(names, k)
	}
	mu.RUnlock()
	sort.Strings(names)
	return nil, fmt.Errorf("discovery: no registrar registered as %q (registered: %v)", name, names)
}
