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

package StarterRegistryNacos

// NacosConfig binds the Nacos naming-server connection under
// ${spring.registry.nacos}.
type NacosConfig struct {
	// Server is the Nacos server address, e.g. "127.0.0.1:8848". Setting it is
	// what activates this starter (fail-loud opt-in; no silent localhost).
	Server string `value:"${server}"`

	// Namespace is the Nacos namespace id to register into; empty uses "public".
	Namespace string `value:"${namespace:=}"`

	// Group is the service group the instance is published under. Discovery
	// clients must resolve within the same group.
	Group string `value:"${group:=DEFAULT_GROUP}"`

	// Cluster is the Nacos cluster name the instance belongs to.
	Cluster string `value:"${cluster:=DEFAULT}"`

	// Username / Password authenticate against Nacos when auth is enabled.
	// Leave empty for anonymous clusters.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// TimeoutMs bounds each Nacos API call, including the startup connectivity
	// probe used to fail fast on an unreachable server.
	TimeoutMs uint64 `value:"${timeout-ms:=5000}"`

	// Name is the key this Nacos registrar is published under in the
	// stdlib/discovery registrar registry. The register server selects a backend
	// by this name via ${spring.registry.backend}; keep both at "default" for the
	// common single-registry case.
	Name string `value:"${name:=default}"`
}

// RegistrationConfig binds the instance to advertise, under ${spring.registry}.
// These fields are backend-agnostic: switching from Nacos to another registry
// backend is a blank-import swap, not a config migration (starter/DESIGN §3).
type RegistrationConfig struct {
	// ServiceName is the logical name to publish — the same name discovery
	// clients later resolve. Required.
	ServiceName string `value:"${service-name:=}"`

	// Addr is the connectable "host:port" advertised to clients. Required; the
	// starter never guesses it, so a misconfiguration fails at startup.
	Addr string `value:"${addr:=}"`

	// ID is accepted for parity with other registry backends but is unused by
	// Nacos, which identifies an instance by its ip:port within a service and
	// cluster. Restarting the same address therefore replaces the same entry.
	ID string `value:"${id:=}"`

	// Weight is the load-balancing weight advertised to clients; 0 falls back to
	// Nacos's default weight of 1 (0 in Nacos means "receive no traffic").
	Weight int `value:"${weight:=0}"`

	// Metadata is arbitrary key/value attributes stored with the instance
	// (zone, unit, version, ...), bound from ${spring.registry.metadata.*}.
	Metadata map[string]string `value:"${metadata:=}"`

	// Backend selects which registrar backend to publish to, by the name it was
	// registered under in the stdlib/discovery registrar registry. Defaults to
	// "default", matching NacosConfig.Name's default.
	Backend string `value:"${backend:=default}"`
}
