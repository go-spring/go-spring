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

package StarterRegistryZookeeper

import "time"

// ZookeeperConfig binds the ZooKeeper ensemble connection under
// ${spring.registry.zookeeper}.
type ZookeeperConfig struct {
	// Servers lists the ZooKeeper ensemble members to connect to, e.g.
	// "127.0.0.1:2181". Required; setting it is what activates this starter
	// (fail-loud opt-in; no silent localhost).
	Servers []string `value:"${servers}"`

	// SessionTimeout is the ZooKeeper session timeout. Ephemeral registration
	// nodes survive as long as the session; if the process dies ZooKeeper removes
	// them roughly one session timeout later. It also bounds the startup probe.
	SessionTimeout time.Duration `value:"${session-timeout:=10s}"`

	// BasePath is the parent znode under which service directories are created,
	// e.g. "/services". Persistent; created on demand.
	BasePath string `value:"${base-path:=/services}"`

	// Username / Password enable ZooKeeper digest authentication when set. Leave
	// both empty for an open ensemble.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`

	// Name is the key this ZooKeeper registrar is published under in the
	// stdlib/discovery registrar registry. The register server selects a backend
	// by this name via ${spring.registry.backend}; keep both at "default" for the
	// common single-registry case.
	Name string `value:"${name:=default}"`
}

// RegistrationConfig binds the instance to advertise, under ${spring.registry}.
// These fields are backend-agnostic: switching from ZooKeeper to another
// registry backend is a blank-import swap, not a config migration
// (starter/DESIGN §3).
type RegistrationConfig struct {
	// ServiceName is the logical name to publish — the same name discovery
	// clients later resolve. Required.
	ServiceName string `value:"${service-name:=}"`

	// Addr is the connectable "host:port" advertised to clients. Required; the
	// starter never guesses it, so a misconfiguration fails at startup.
	Addr string `value:"${addr:=}"`

	// ID overrides the instance id within the service; empty derives a stable one
	// from ServiceName and Addr so restarts replace the same znode.
	ID string `value:"${id:=}"`

	// Weight is the load-balancing weight advertised to clients; 0 means the
	// backend default.
	Weight int `value:"${weight:=0}"`

	// Metadata is arbitrary key/value attributes stored with the instance
	// (zone, unit, version, ...), bound from ${spring.registry.metadata.*}.
	Metadata map[string]string `value:"${metadata:=}"`

	// Backend selects which registrar backend to publish to, by the name it was
	// registered under in the stdlib/discovery registrar registry. Defaults to
	// "default", matching ZookeeperConfig.Name's default.
	Backend string `value:"${backend:=default}"`
}
