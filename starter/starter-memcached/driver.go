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
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/governance/capability/discovery"
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
// the driver sets both on the wrapper it builds; center is the container's
// runtime — carrying the resilience/fault/loadbalance authorities and
// the discovery directory — and [NewClient] applies it while building. The
// only thing that may touch the client afterwards is post-processing that
// reorganizes the embedded InnerClient — its operation chain — before the
// client takes traffic.
//
// center is one object rather than a parameter per capability so this interface
// — which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its facilities simply ignores it.
//
// name is passed as an argument rather than carried on Config: Config stays a
// pure projection of the entry's properties, and the instance name is the map
// key, not a value in it.
//
// At most one Driver bean is expected per process; every client under
// ${spring.memcached} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, name string, c Config, center *governance.Runtime) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Memcached client based on the provided configuration.
//
// When in-process discovery applies (a service name is set and the process is not in mesh mode), the server set
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
// The entry's ${discovery} label rides on the config (c.Discovery, already
// carrying the family default): the driver resolves it against
// center.Discovery() itself, failing construction when an entry routes by
// service-name and the label names no bean.
//
// In mesh mode discovery is skipped entirely (the runtime reports in-process discovery as off): a sidecar owns
// discovery+LB, so the client connects straight to the configured static
// Servers list (the service's stable DNS address).
func (DefaultDriver) CreateClient(ctx context.Context, name string, c Config, center *governance.Runtime) (*Client, error) {
	servers := c.Servers
	var client *memcache.Client
	// Mesh mode wins over a configured service-name: the sidecar owns
	// discovery+LB, so the client uses the configured Servers list (the stable
	// addresses) and no resolver is built.
	if !center.UsesDiscovery(c.ServiceName) {
		client = memcache.New(servers...)
	} else {
		// Resolve the discovery backend this entry cites. The label rides on the
		// config (c.Discovery, already carrying the family default); an entry
		// routed by service-name that cites no existing backend fails here, at
		// construction.
		backend, _ := center.Discovery().Get(c.Discovery)
		if backend == nil {
			return nil, errutil.Explain(nil, "memcached: instance %q routes by service-name but cites discovery backend %q, which names no bean", name, c.Discovery)
		}
		// Fail fast on an unusable cluster: the live selector tolerates a later
		// empty snapshot per operation, but booting against one is a
		// misconfiguration the operator should see now.
		// Resolver freshness lives inside the backend, so there is nothing to release.
		resolver, err := discovery.NewResolver(ctx, backend, c.ServiceName, c.Scheme)
		if err != nil {
			return nil, errutil.Explain(err, "memcached: discovery resolve %q failed", c.ServiceName)
		}
		eps, err := resolver(ctx)
		if err != nil {
			return nil, errutil.Explain(err, "memcached: discovery resolve %q failed", c.ServiceName)
		}
		if len(eps) == 0 {
			return nil, errutil.Explain(nil, "memcached: discovery returned no endpoints for %q", c.ServiceName)
		}
		client = memcache.NewFromSelector(newLiveServers(resolver))
	}
	if c.Timeout > 0 {
		client.Timeout = c.Timeout
	}
	if c.MaxIdleConns > 0 {
		client.MaxIdleConns = c.MaxIdleConns
	}
	// NewClient is the only way to build a Client: identity and governance are
	// both applied here, so the driver returns a client that is complete.
	return NewClient(client, name, c.ServiceName, center), nil
}
