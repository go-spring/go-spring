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
	"sync"
)

// Instance describes a single service instance a provider publishes to a
// registry — the write-side counterpart to the read-side [Endpoint]. The two
// are the same entity seen from opposite sides: a provider Registers an
// Instance; once it lands in the registry, discovery clients observe it as an
// Endpoint. Scheme, Weight, Disabled and Metadata carry straight over; the
// Endpoint adds the probe-driven Healthy flag (which the publisher does not
// set) and drops the publisher-only ID.
type Instance struct {
	// ServiceName is the logical name the instance is published under — the same
	// name discovery clients later pass to Discovery.Resolve / Discovery.Watch.
	ServiceName string

	// ID uniquely identifies this instance within the service. When empty a
	// Registrar derives a stable one (e.g. from ServiceName and Addr) so a
	// restart replaces the previous entry instead of accumulating duplicates.
	ID string

	// Addr is the connectable "host:port" advertised to clients.
	Addr string

	// Scheme selects the transport for this instance, using the same vocabulary
	// as [Endpoint.Scheme]: "" or "tcp" for plain TCP, "tls"/"https" when it
	// requires TLS, "http"/"https" for HTTP-level routing. It lets one service
	// publish both plain and secure instances under the same name; readers learn
	// which to dial. Leave empty when the instance does not distinguish.
	Scheme string

	// Weight is the load-balancing weight advertised to clients; 0 means the
	// default.
	Weight int

	// Disabled is the instance's own on/off toggle: set true to announce that
	// this instance must stop receiving new traffic WHILE STAYING REGISTERED —
	// the graceful-shutdown / drain signal. Readers see it as [Endpoint.Disabled]
	// and exclude the instance from the eligible set (never even as a fallback).
	// The canonical sequence is Register → re-Register with Disabled=true to
	// drain → Deregister (Register is idempotent on ID; see [Registrar.Register]).
	// The zero value (false) means "serving".
	//
	// It is the instance's decree, distinct from the registry's probe result:
	// Disabled is set by the publisher (or an operator), Healthy is set by probes
	// and is read-only here.
	Disabled bool

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
// CRASH SAFETY (read this): Register MUST use a self-renewing registration —
// heartbeat, TTL, or an "ephemeral" instance — so that an instance whose
// process dies (SIGKILL, panic, OOM) WITHOUT ever calling Deregister is still
// eventually removed by the registry once its keep-alive goes silent. Deregister
// is only the prompt, graceful path for clean shutdown; CORRECTNESS NEVER
// DEPENDS ON IT BEING CALLED. A "permanent" registration that lingers forever
// after a crash violates this contract — backends that support persistent
// instances must not expose them here.
//
// RPC-framework provider registration is deliberately out of scope — that stays
// bound to each framework (kitex, kratos, dubbo, ...).
//
// Implementations must be safe for concurrent use.
type Registrar interface {
	// Register publishes inst and starts the keep-alive that holds the instance
	// in the registry. Per the crash-safety rule above, the keep-alive renews
	// itself until Deregister — or the process dying — ends it.
	//
	// Register is IDEMPOTENT on identity: registering again with the same ID
	// refreshes the existing entry instead of creating a duplicate. That refresh
	// IS the update mechanism — re-publish with changed fields to mutate a live
	// instance, e.g. Register again with Disabled=true to drain traffic, then
	// Deregister once in-flight work completes. There is deliberately NO separate
	// Update method: Consul and Nacos both implement "modify an instance" as a
	// re-registration with the same ID, and kratos/kitex expose only
	// Register/Deregister.
	Register(ctx context.Context, inst Instance) error

	// Deregister removes inst and stops the keep-alive started by Register, for a
	// prompt clean-shutdown removal. It is safe to call for an instance that is
	// not currently registered.
	//
	// It is NOT guaranteed to be called: the process may crash or be killed
	// first. So the registry must already be prepared to expire the instance on
	// its own once the keep-alive stops; Deregister just makes that happen at
	// once instead of after the TTL. Code that wires Deregister into a shutdown
	// hook should treat a missing call as the normal failure mode, not an error.
	Deregister(ctx context.Context, inst Instance) error
}

// registrarsMu guards registrars. It is independent of discoveriesMu (in
// discovery.go): the two registries never need a single atomic operation across
// both, so a per-registry lock lets the read side and write side scale without
// serializing against each other.
var (
	registrarsMu sync.RWMutex
	registrars   = map[string]Registrar{}
)

// RegisterRegistrar makes a Registrar available under name. It panics on empty
// name, nil Registrar, or a duplicate name — mirroring the driver-registry
// idiom used for the read side (see [RegisterDiscovery]) — so mis-wiring fails
// loudly at init rather than silently.
func RegisterRegistrar(name string, r Registrar) {
	if name == "" {
		panic("discovery: register with empty name")
	}
	if r == nil {
		panic("discovery: register nil Registrar for " + name)
	}
	registrarsMu.Lock()
	defer registrarsMu.Unlock()
	if _, ok := registrars[name]; ok {
		panic("discovery: Registrar already registered: " + name)
	}
	registrars[name] = r
}

// GetRegistrar returns the Registrar registered under name, or an error that
// lists every registered registrar when none matches — so a misconfigured name
// surfaces readably at construction time rather than as a silent no-op. Callers
// that only need a presence check test err == nil.
func GetRegistrar(name string) (Registrar, error) {
	registrarsMu.RLock()
	defer registrarsMu.RUnlock()
	if r, ok := registrars[name]; ok {
		return r, nil
	}
	names := make([]string, 0, len(registrars))
	for k := range registrars {
		names = append(names, k)
	}
	sort.Strings(names)
	return nil, fmt.Errorf("discovery: no Registrar registered as %q (registered: %v)", name, names)
}
