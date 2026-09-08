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

package StarterGoRedis

import (
	"time"

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/tlsconf"
)

// Config defines Redis connection configuration.
type Config struct {
	// Mode selects the Redis topology: "single" (default), "sentinel", or
	// "cluster". It stays "single" by default so existing single-node
	// configurations keep working unchanged.
	//   - single:   dials Addr (or ServiceName via service discovery).
	//   - sentinel:  connects to the master resolved by MasterName through
	//                SentinelAddrs; the bean type is still *redis.Client.
	//   - cluster:   connects to the cluster seeded by Addrs; the bean type is
	//                *redis.ClusterClient (a distinct type — see README).
	Mode string `value:"${mode:=single}"`

	// Addr is the Redis server address, e.g., "127.0.0.1:6379".
	// Used only in single mode; either Addr or ServiceName must be set.
	Addr string `value:"${addr:=}"`

	// MasterName is the sentinel master group name. Required in sentinel mode.
	MasterName string `value:"${master-name:=}"`

	// SentinelAddrs are the sentinel node addresses, e.g.,
	// ["127.0.0.1:26379", "127.0.0.1:26380"]. Required in sentinel mode.
	SentinelAddrs []string `value:"${sentinel-addrs:=}"`

	// SentinelPassword is the password used to authenticate with the sentinels
	// themselves (distinct from Password, which authenticates with the master).
	SentinelPassword string `value:"${sentinel-password:=}"`

	// Addrs are the cluster seed node addresses, e.g.,
	// ["127.0.0.1:7000", "127.0.0.1:7001"]. Required in cluster mode.
	Addrs []string `value:"${addrs:=}"`

	// MaxRedirects is the maximum number of MOVED/ASK redirects to follow in
	// cluster mode, default is 0 (go-redis default of 3 applies).
	MaxRedirects int `value:"${max-redirects:=0}"`

	// RouteByLatency routes read-only commands to the lowest-latency node in
	// cluster mode. Default is false.
	RouteByLatency bool `value:"${route-by-latency:=false}"`

	// RouteRandomly routes read-only commands to a random node in cluster mode.
	// Default is false.
	RouteRandomly bool `value:"${route-randomly:=false}"`

	// Password is the Redis server password, default is empty.
	Password string `value:"${password:=}"`

	// DB is the Redis database number, default is 0. Redis Cluster exposes no
	// databases, so a non-zero DB combined with Mode=cluster is rejected at
	// startup.
	DB int `value:"${db:=0}"`

	// Username is the Redis ACL username, default is empty.
	Username string `value:"${username:=}"`

	// PoolSize is the maximum number of socket connections, default is 10.
	PoolSize int `value:"${pool-size:=10}"`

	// MaxIdle is the maximum number of idle connections in the pool, default is 5.
	MaxIdle int `value:"${max-idle:=5}"`

	// MaxRetries is the maximum number of retries for failed commands, default is 0.
	MaxRetries int `value:"${max-retries:=0}"`

	// DialTimeout is the timeout for dialing the Redis server, e.g., "5s".
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// ReadTimeout is the timeout for reading from Redis, e.g., "3s".
	ReadTimeout time.Duration `value:"${read-timeout:=3s}"`

	// WriteTimeout is the timeout for writing to Redis, e.g., "3s".
	WriteTimeout time.Duration `value:"${write-timeout:=3s}"`

	// ConnMaxLifetime is the maximum amount of time a connection can be reused, e.g., "2m".
	// Shorter values facilitate smoother traffic switching during service discovery updates.
	ConnMaxLifetime time.Duration `value:"${conn-max-lifetime:=2m}"`

	// ServiceName is the service discovery name for a single Redis instance.
	// When set, Addr is ignored and the actual address is resolved via service
	// discovery. It applies to single mode only: sentinel and cluster topologies
	// self-discover their nodes, so combining ServiceName with those modes is
	// rejected at startup.
	ServiceName string `value:"${service-name:=}"`

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set.
	Scheme string `value:"${scheme:=}"`

	// Discovery names the discovery backend bean that resolves ServiceName
	// (bean name = label). It is only consulted when ServiceName is set; the
	// bean itself is injected by the starter's constructor from this label.
	Discovery string `value:"${discovery:=default}"`

	// backend is the discovery backend instance the label above cites. It is
	// populated by the starter wiring (newClient injects the named backend
	// bean), never bound from configuration.
	backend discovery.Discovery

	// TLS configures an optional TLS connection to Redis. When TLS.Enabled is
	// false (the default) the client dials in plaintext.
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// Otel toggles the redisotel tracing/metrics instrumentation attached to
	// the client at construction. Both default to true — redisotel rides the
	// OTel globals that starter-otel installs and is a no-op without it, so
	// leaving them on is the zero-config opt-in. Set either to false to fully
	// detach that signal's hooks for an instance, e.g. a high-throughput cache
	// where the per-command span overhead is unwanted, or when a custom
	// instrumentation is supplied via a Driver.
	Otel OtelConfig `value:"${otel}"`

	// HealthEnabled controls whether the starter contributes a health.Indicator
	// bean for each instance. It mirrors starter-redigo's switch so operators
	// can turn the readiness probe off uniformly, e.g. for an instance whose
	// server is intentionally short-lived. Default is true.
	HealthEnabled bool `value:"${health.enabled:=true}"`
}

// OtelConfig toggles the built-in redisotel instrumentation per instance and per
// signal. See [Config.Otel].
type OtelConfig struct {
	TracingEnabled bool `value:"${tracing.enabled:=true}"`
	MetricsEnabled bool `value:"${metrics.enabled:=true}"`
}

// Resilience is no longer a field of Config: it is resolved by the Client
// wrapper bean's Init (the gs InitMethod) through the neutral
// resilience.ExecutorFor seam.

// Resilience binds the backend-neutral resilience knobs shared by every client
// starter (see [resilience.Config]). Driver selects which registered backend
// enforces them: "default" (bundled, zero-dependency) or "sentinel"
// (recommended, enabled by blank-importing starter-resilience). Switching
// backends is a one-line config change — no code touches the hook seam.
//
// Keep resilience MaxRetries at 0 for Redis: go-redis already retries via its
// own MaxRetries, and re-sending non-idempotent commands is unsafe.
