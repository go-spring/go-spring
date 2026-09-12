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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + the bundled DefaultDriver, which owns full client assembly
// (including service-discovery resolution). It mirrors starter-redigo's
// driver.go.
package StarterMemcached

import (
	"context"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a Memcached client. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterMemcached.Driver); when none is present,
// starter-memcached falls back to the bundled [DefaultDriver] inside client
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// At most one Driver bean is expected per process; every client under
// ${spring.memcached} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*memcache.Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Memcached client based on the provided configuration.
//
// When c.ServiceName is set (and mesh mode is not enabled), the server set
// follows the discovery backend (backend, wired from the ${discovery} label)
// instead of c.Servers: the client is built over a live
// [memcache.ServerSelector] that re-reads the endpoint snapshot on every key
// lookup, so an instance joining or leaving is visible on the next operation.
// The initial snapshot is still read here, so a service that resolves to nothing
// fails at boot rather than on first use. Key affinity is preserved — the same
// CRC32-of-key hashing the built-in ServerList uses, over an address-sorted
// snapshot so an unchanged cluster keeps an unchanged key→server mapping.
// Resolver freshness lives inside the backend, so there is nothing to release.
//
// backend is the discovery backend the entry's ${discovery} label resolved to,
// already looked up by the starter wiring; it is nil when the entry cites no
// label (an unknown label fails at wiring, before the driver is called). It is
// passed as an argument rather than carried on Config so a custom driver can
// actually reach it — Config stays a pure bound value.
//
// In mesh mode (mesh.Enabled) discovery is skipped entirely: a sidecar owns
// discovery+LB, so the client connects straight to the configured static
// Servers list (the service's stable DNS address).
func (DefaultDriver) CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*memcache.Client, error) {
	servers := c.Servers
	resolver, err := newLiveResolver(ctx, c, backend)
	if err != nil {
		return nil, errutil.Explain(err, "memcached: discovery resolve %q failed", c.ServiceName)
	}
	var client *memcache.Client
	if resolver != nil {
		// Fail fast on an unusable cluster: the live selector tolerates a later
		// empty snapshot per operation, but booting against one is a
		// misconfiguration the operator should see now.
		eps, err := resolver()
		if err != nil {
			return nil, errutil.Explain(err, "memcached: discovery resolve %q failed", c.ServiceName)
		}
		if len(eps) == 0 {
			return nil, errutil.Explain(nil, "memcached: discovery returned no endpoints for %q", c.ServiceName)
		}
		client = memcache.NewFromSelector(newLiveServers(resolver))
	} else {
		client = memcache.New(servers...)
	}
	if c.Timeout > 0 {
		client.Timeout = c.Timeout
	}
	if c.MaxIdleConns > 0 {
		client.MaxIdleConns = c.MaxIdleConns
	}
	return client, nil
}

// newLiveResolver resolves the discovery backend for c into a by-name
// Resolver that re-reads the service's live endpoint snapshot. It returns
// (nil, nil) when service-name is unset or mesh mode is enabled (a sidecar owns
// discovery+LB), in which case the caller uses the configured Servers list.
// Resolver freshness lives inside the backend, so there is nothing to release.
func newLiveResolver(ctx context.Context, c Config, backend discovery.Discovery) (discovery.Resolver, error) {
	return discovery.NewResolver(ctx, backend, c.ServiceName, discovery.WithScheme(c.Scheme))
}
