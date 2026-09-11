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

	// TTL is the Consul TTL health check interval. The registrar refreshes the
	// check on a heartbeat at half this interval so the instance stays passing;
	// if the process dies the check goes critical after one TTL.
	TTL time.Duration `value:"${ttl:=15s}"`

	// DeregisterCriticalAfter tells Consul to drop the instance automatically if
	// its check stays critical this long (e.g. after an ungraceful crash that
	// skipped Deregister). Zero disables auto-deregistration.
	DeregisterCriticalAfter time.Duration `value:"${deregister-critical-after:=1m}"`
}

