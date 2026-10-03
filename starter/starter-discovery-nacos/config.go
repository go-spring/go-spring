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

package StarterDiscoveryNacos

import "go-spring.org/cloud/security"

// NacosConfig binds the Nacos naming-server connection under
// ${spring.discovery.nacos}.
type NacosConfig struct {
	// Enabled turns this named block off without deleting it. Default true;
	// a disabled block contributes no beans (no client, no registration,
	// no discovery, no health indicator).
	Enabled bool `value:"${enabled:=true}"`

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
	// probe (see Ping) used to fail fast on an unreachable server.
	TimeoutMs uint64 `value:"${timeout-ms:=5000}"`

	// TLS configures optional transport-layer security for a TLS-enabled
	// Nacos server. Off by default. Uses the shared spring/cloud/security
	// block so every starter exposes the same tls.* keys.
	TLS security.TLSConfig `value:"${tls}"`

	// Ping enables the startup connectivity probe: when true the constructor
	// probes the cluster once and fails startup if it is unreachable, surfacing
	// misconfiguration early. Default is false: a cluster that is not up yet
	// must not block the application from starting; connectivity problems
	// surface on first use instead. Set true to restore fail-fast behaviour.
	Ping bool `value:"${ping:=false}"`

	// Health controls whether the starter contributes a health.Indicator bean
	// for this block (readiness/startup probes via starter-actuator). On by
	// default; set false to keep it out of the aggregated health report.
	Health bool `value:"${health:=true}"`
}
