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
	"go-spring.org/cloud"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/resilience"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a Memcached client. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterMemcached.Driver); when none is present,
// starter-memcached falls back to the bundled [DefaultDriver] inside client
// assembly. A custom driver is a bean, so it may inject the configuration/beans
// it needs — e.g. company config bound from a properties file at wiring time.
//
// CreateClient returns the module's exported [Client] — the wrapper apps inject
// — not the raw *memcache.Client, so a driver takes part in the type the rest of
// the ecosystem sees and future wrapper capabilities are reachable from it. It
// returns the client COMPLETE: name is the config entry's key
// (spring.memcached.instances.<name>) and c.ServiceName its discovery name, so
// the driver sets both on the wrapper it builds; params carries the container's
// facilities — the resilience/fault/loadbalance authorities and the discovery
// backend — and [NewClient] applies them while building. The only thing that
// may touch the client afterwards is post-processing that reorganizes the
// embedded InnerClient — its operation chain — before the client takes
// traffic.
//
// params is one struct rather than a parameter per capability so this interface
// — which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its fields simply ignores it.
//
// name is passed as an argument rather than carried on Config: Config stays a
// pure projection of the entry's properties, and the instance name is the map
// key, not a value in it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.memcached} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, name string, c Config, params cloud.ClientParams) (*Client, error)
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
// params.Discovery is the discovery backend the entry's ${discovery} label
// resolved to, already looked up by the starter wiring; it is nil when the entry
// cites no label (an unknown label fails at wiring, before the driver is
// called). It rides on the params struct rather than Config so a custom driver
// can actually reach it — Config stays a pure bound value.
//
// In mesh mode (mesh.Enabled) discovery is skipped entirely: a sidecar owns
// discovery+LB, so the client connects straight to the configured static
// Servers list (the service's stable DNS address).
func (DefaultDriver) CreateClient(ctx context.Context, name string, c Config, params cloud.ClientParams) (*Client, error) {
	servers := c.Servers
	// A nil resolver (service-name unset, or mesh mode where a sidecar owns
	// discovery+LB) means the caller uses the configured Servers list.
	// Resolver freshness lives inside the backend, so there is nothing to release.
	resolver, err := discovery.NewResolver(ctx, params.Discovery, c.ServiceName, discovery.WithScheme(c.Scheme))
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
	// The governance rule may size the pool too — the resource half of isolation,
	// next to the bulkhead's concurrency half — and it wins over the per-instance
	// key, so one place configures both halves. gomemcache caps IDLE connections
	// and dials on demand, so this rule's MaxConns is that cap here, not a limit
	// on open connections.
	if n := params.PolicyFor(resilience.ServiceLabel("memcached", c.ServiceName, name)).MaxConns; n > 0 {
		client.MaxIdleConns = n
	}
	// NewClient is the only way to build a Client: identity and governance are
	// both applied here, so the driver returns a client that is complete.
	return NewClient(client, name, c.ServiceName, params), nil
}
