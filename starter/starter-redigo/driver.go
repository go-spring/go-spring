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
	"context"
)

// Driver interface defines how to create a Redis client (a connection pool) —
// THE extension point for customizing pool assembly. It is an OPTIONAL CONTAINER
// BEAN: a company or umbrella starter may provide its own Driver bean (exported
// via [go-spring.org/spring/gs.As]); when none is present, starter-redigo falls
// back to the bundled [DefaultDriver] inside pool assembly. Because a custom
// driver is a bean, it may inject the configuration/beans it needs — e.g.
// company config bound from a properties file at wiring time, which an
// init-time seam could not see.
//
// At most one Driver bean is expected per process; every pool under
// ${spring.redigo} is built through it, and per-instance differences are
// expressed through [Config].
//
// The driver owns the FULL assembly and returns the starter's wrapped [Pool]
// (embeds the concrete *redis.Pool), fully armed. The bundled DefaultDriver
// simply delegates to [NewPool] — the one-shot standard assembly (raw dial +
// discovery/TLS + observer + resilience + instrumented dial). Two
// customization shapes:
//
//   - ADD to the default: call [NewPool] (or embed DefaultDriver), then
//     customize the returned Pool via the public API (e.g.
//     [Pool.UseCommandInterceptor]).
//   - REPLACE: build and arm the Pool entirely your own way — the public
//     primitive is [NewConn] (variadic interceptors); you simply own what the
//     standard assembly would have done, including any teardown of resources
//     you built.
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*Pool, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Redis pool based on the provided configuration.
//
// When c.ServiceName is set (and mesh mode is not enabled), the address is
// resolved through the registered discovery backend (c.Discovery) instead of
// c.Addr: a discovery loader keeps the endpoint set fresh via the backend's
// watch and the pool dials a live instance (Pick) for each new connection.
// Combined with c.ConnMaxLifetime, pooled connections recycle onto updated
// addresses without rebuilding the pool. When c.ServiceName is empty this is a
// plain Addr dial, unchanged from before.
//
// In mesh mode (mesh.Enabled) discovery is skipped entirely: a sidecar owns
// discovery+LB, so the pool connects straight to the configured static Addr
// (the service's stable DNS address).
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*Pool, error) {
	return NewPool(ctx, c)
}
