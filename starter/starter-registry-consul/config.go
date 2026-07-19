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

package StarterRegistryConsul

import "time"

// ConsulConfig binds the Consul agent connection under ${spring.registry.consul}.
type ConsulConfig struct {
	// Address is the Consul HTTP API address, e.g. "127.0.0.1:8500". Setting it
	// is what activates this starter (fail-loud opt-in; no silent localhost).
	Address string `value:"${address}"`

	// Scheme is the URI scheme for the Consul server, "http" or "https".
	Scheme string `value:"${scheme:=http}"`

	// Datacenter is the datacenter to register into; empty uses the agent's.
	Datacenter string `value:"${datacenter:=}"`

	// Token is the ACL token used for requests, empty for none.
	Token string `value:"${token:=}"`

	// Namespace is the Consul Enterprise namespace, empty for none.
	Namespace string `value:"${namespace:=}"`

	// Name is the key this Consul registrar is published under in the
	// stdlib/discovery registrar registry. The register server selects a backend
	// by this name via ${spring.registry.backend}; keep both at "default" for the
	// common single-registry case.
	Name string `value:"${name:=default}"`

	// TTL is the Consul TTL health check interval. The registrar refreshes the
	// check on a heartbeat at half this interval so the instance stays passing;
	// if the process dies the check goes critical after one TTL.
	TTL time.Duration `value:"${ttl:=15s}"`

	// DeregisterCriticalAfter tells Consul to drop the instance automatically if
	// its check stays critical this long (e.g. after an ungraceful crash that
	// skipped Deregister). Zero disables auto-deregistration.
	DeregisterCriticalAfter time.Duration `value:"${deregister-critical-after:=1m}"`
}

// RegistrationConfig binds the instance to advertise, under ${spring.registry}.
// These fields are backend-agnostic: switching from Consul to another registry
// backend is a blank-import swap, not a config migration (starter/DESIGN §3).
type RegistrationConfig struct {
	// ServiceName is the logical name to publish — the same name discovery
	// clients later resolve. Required.
	ServiceName string `value:"${service-name:=}"`

	// Addr is the connectable "host:port" advertised to clients. Required; the
	// starter never guesses it, so a misconfiguration fails at startup.
	Addr string `value:"${addr:=}"`

	// ID overrides the instance id within the service; empty derives a stable one
	// from ServiceName and Addr so restarts replace the same entry.
	ID string `value:"${id:=}"`

	// Weight is the load-balancing weight advertised to clients; 0 uses the
	// backend default.
	Weight int `value:"${weight:=0}"`

	// Metadata is arbitrary key/value attributes stored with the instance
	// (zone, unit, version, ...), bound from ${spring.registry.metadata.*}.
	Metadata map[string]string `value:"${metadata:=}"`

	// Backend selects which registrar backend to publish to, by the name it was
	// registered under in the stdlib/discovery registrar registry. Defaults to
	// "default", matching ConsulConfig.Name's default.
	Backend string `value:"${backend:=default}"`
}
