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

// config.go is the config concept: the per-instance Config bound under
// ${spring.memcached}.* and the Driver selection key.
package StarterMemcached

import "time"

// Config defines Memcached client connection configuration.
type Config struct {
	// Servers is the list of Memcached server addresses to connect to,
	// e.g., "127.0.0.1:11211". Requests are sharded across the servers.
	// Either Servers or ServiceName must be set.
	Servers []string `value:"${servers:=}"`

	// ServiceName is the service discovery name for the Memcached cluster. When
	// set (and mesh mode is not enabled), Servers is ignored and the server list
	// is resolved once at startup through the registered discovery backend.
	//
	// Note: gomemcache shards keys across a static server set chosen at client
	// creation, so — unlike the dialer-based redis starters — the address list
	// is resolved a single time at boot (fail-fast) rather than kept live. A
	// changing cluster membership requires a restart to pick up. In mesh mode
	// discovery is skipped entirely and Servers is used as-is.
	ServiceName string `value:"${service-name:=}"`

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string `value:"${scheme:=}"`

	// Discovery selects which registered discovery backend resolves ServiceName.
	// It is only consulted when ServiceName is set; the default backend name is
	// "default".
	Discovery string `value:"${discovery:=default}"`

	// Timeout is the socket read/write timeout for each request,
	// 0 uses the driver default (100ms), e.g., "100ms".
	Timeout time.Duration `value:"${timeout:=0}"`

	// MaxIdleConns is the maximum number of idle connections kept per server,
	// 0 uses the driver default (2).
	MaxIdleConns int `value:"${max-idle-conns:=0}"`
}

// Resilience is not a field of Config: it comes from the governance center
// (starter-govern) and is consumed by Client.Init (the gs InitMethod).
