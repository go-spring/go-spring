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

package StarterDiscoveryZookeeper

import "time"

// ZookeeperConfig binds the ZooKeeper ensemble connection under
// ${spring.discovery.zookeeper}.
type ZookeeperConfig struct {
	// Servers lists the ZooKeeper ensemble members to connect to, e.g.
	// "127.0.0.1:2181". Required; setting it is what activates this starter
	// (fail-loud opt-in; no silent localhost).
	Servers []string `value:"${servers}"`

	// SessionTimeout is the ZooKeeper session timeout. Ephemeral registration
	// nodes survive as long as the session; if the process dies ZooKeeper removes
	// them roughly one session timeout later. It also bounds the startup probe
	// (see Ping).
	SessionTimeout time.Duration `value:"${session-timeout:=10s}"`

	// BasePath is the parent znode under which service directories are created,
	// e.g. "/services". Persistent; created on demand.
	BasePath string `value:"${base-path:=/services}"`

	// Username / Password enable ZooKeeper digest authentication when set. Leave
	// both empty for an open ensemble.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

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
