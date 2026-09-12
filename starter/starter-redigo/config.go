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

package StarterRedigo

import (
	"time"

	"go-spring.org/cloud/tlsconf"
)

// Config defines Redis connection configuration.
type Config struct {
	// Addr is the Redis server address, e.g., "127.0.0.1:6379".
	// Either Addr or ServiceName must be set.
	Addr string `value:"${addr:=}"`

	// Password is the Redis server password, default is empty.
	Password string `value:"${password:=}"`

	// DB is the Redis database number, default is 0.
	DB int `value:"${db:=0}"`

	// Username is the Redis ACL username, default is empty.
	Username string `value:"${username:=}"`

	// PoolSize is the maximum number of connections allocated by the pool at a given time.
	// When zero, there is no limit on the number of connections in the pool.
	PoolSize int `value:"${pool-size:=10}"`

	// MaxIdle is the maximum number of idle connections in the pool.
	MaxIdle int `value:"${max-idle:=5}"`

	// DialTimeout is the timeout for dialing the Redis server, e.g., "5s".
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// ReadTimeout is the timeout for reading from Redis, e.g., "3s".
	ReadTimeout time.Duration `value:"${read-timeout:=3s}"`

	// WriteTimeout is the timeout for writing to Redis, e.g., "3s".
	WriteTimeout time.Duration `value:"${write-timeout:=3s}"`

	// ConnMaxLifetime is the maximum amount of time a connection can be reused, e.g., "2m".
	// Shorter values facilitate smoother traffic switching during service discovery updates.
	ConnMaxLifetime time.Duration `value:"${conn-max-lifetime:=2m}"`

	// ServiceName is the service discovery name for Redis cluster.
	// When set, Addr is ignored and the actual address is resolved via service discovery.
	ServiceName string `value:"${service-name:=}"`

	// Scheme narrows discovery to endpoints of one transport scheme (e.g. "tls",
	// "https"). Empty (the default) returns every scheme; set it when a service
	// exposes both plain and secure instances and this client should reach only
	// one. Only consulted when ServiceName is set. Field layout matches
	// starter-go-redis.
	Scheme string `value:"${scheme:=}"`

	// Discovery names the discovery backend bean that resolves ServiceName
	// (bean name = label). It is only consulted when ServiceName is set; the
	// starter wiring resolves this label to the backend bean and passes it to
	// the driver as the backend argument of CreateClient / the backend
	// argument of NewPool — it is never written back into Config. Field layout
	// matches starter-go-redis.
	// Empty means unset: no discovery backend is wired and the entry must not
	// route by service-name alone.
	Discovery string `value:"${discovery:=}"`

	// TLS configures an optional TLS connection to Redis. When TLS.Enabled is
	// false (the default) the client dials in plaintext. Field layout matches
	// starter-go-redis so the two starters stay interchangeable.
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// StartupPing, when true, dials one connection at boot and PINGs it so a
	// misconfigured address or unreachable server surfaces during startup
	// rather than on the first request. Defaults to false: the redigo pool
	// dials lazily, so without this flag a bad address is only discovered once
	// a command actually runs. (starter-go-redis performs this probe
	// unconditionally; here it is opt-in.)
	StartupPing bool `value:"${startup-ping:=false}"`

	// ObserveEnabled turns this instance's instrumentation layer (trace span +
	// metric + access log) on/off. Defaults to true. It is distinct from the
	// GLOBAL observability.* keys (which configure the access-log level shared
	// across ALL client starters): this is a hard per-instance kill switch.
	// When false, no observer is built and the Conn wrapper becomes a
	// near-zero-cost pass-through (it still wraps, to stay transparent for
	// DoContext / DoWithTimeout, but emits no span/metric/log).
	ObserveEnabled bool `value:"${observe.enabled:=true}"`

	// HealthEnabled controls whether the starter contributes a health.Indicator
	// (named redigo:<name>) for this instance. Defaults to true; set false for a
	// pool that should not roll into aggregate health (e.g. a non-critical
	// cache). When false, no indicator bean is registered for this instance.
	HealthEnabled bool `value:"${health.enabled:=true}"`
}

// Resilience and Observability policy are not fields of Config: the Pool
// wrapper resolves its executor at Init through the neutral
// [resilience.ExecutorFor] seam (see pool.go setupResilience), so policy comes
// from the governance document rather than from a bound property here. Config
// carries only the per-instance on/off switch HealthEnabled; instrumentation
// itself is unconditional (a no-op without starter-otel).
