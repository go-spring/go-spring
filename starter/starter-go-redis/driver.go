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
	"context"
	"net"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/runtime"
	"go-spring.org/cloud/runtime/dataplane/kernel/discovery"
	"go-spring.org/cloud/runtime/dataplane/kernel/loadbalance"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a single/sentinel Redis client. It is
// an OPTIONAL CONTAINER BEAN: a company or umbrella starter may provide its own
// Driver bean (its constructor returns StarterGoRedis.Driver); when none is
// present, starter-go-redis falls back to the bundled [DefaultDriver] inside
// client assembly. A custom driver is a bean, so it may inject the
// configuration/beans it needs — e.g. company config bound from a properties
// file at wiring time.
//
// At most one Driver bean is expected per process. Because single-Driver-bean
// selection is process-wide, every ${spring.go-redis} instance is built through
// it regardless of Mode; a service that also uses cluster mode must provide a
// Driver that implements [ClusterDriver] too.
//
// CreateClient returns the module's exported [Client] — the wrapper apps inject
// — not a raw *redis.Client, so a driver takes part in the type the rest of the
// ecosystem sees and future wrapper capabilities are reachable from it. It
// returns the client COMPLETE: the driver builds the raw client, then hands it
// (with the endpoint-selection chain it built, if any, and center) to
// [NewClient], which fixes the identity, installs the redisotel pool-metrics +
// declaration layers, and applies the governance executor. Nothing reaches into
// the client afterwards.
//
// center is the container's runtime — the family's one injection
// point — carrying resilience, fault, loadbalance and the discovery directory.
// A discovery-routed driver resolves the backend its entry's ${discovery} label
// names from center.Discovery() and builds its [loadbalance.Chain] over the
// resolver — which registers the chain under the service's governance label, so
// the strategy and outlier suspension apply from the first dial and are rebuilt
// when a rule moves them — then passes the chain, along with center, to
// [NewClient] and forgets about it. It is one object rather than a parameter per
// capability so this interface — which every company driver implements — stays
// stable as capabilities are added.
//
// The entry's ${discovery} label rides on the config (c.Discovery, already
// carrying the family default): the driver resolves it against
// center.Discovery() itself, failing construction when an entry routes by
// service-name and the label names no bean.
type Driver interface {
	CreateClient(ctx context.Context, c Config, center *runtime.Runtime) (*Client, error)
}

// ClusterDriver is an optional interface a Driver may also implement to support
// cluster mode. It is kept separate from Driver so a Driver bean that only
// builds single/sentinel clients stays valid: the starter type-asserts to
// ClusterDriver only for instances with Mode=cluster.
//
// Cluster mode self-discovers through the cluster's own topology and the client
// seeds dialing with c.Addrs — there is no per-endpoint pick pool to hand back —
// so the pool argument to [NewClient] is nil on this path, and only the center's
// resilience + fault capabilities apply.
type ClusterDriver interface {
	CreateClusterClient(ctx context.Context, c Config, center *runtime.Runtime) (*Client, error)
}

// DefaultDriver is the default implementation of the Driver interface. It also
// implements ClusterDriver, so it can build all three topologies.
type DefaultDriver struct{}

var (
	_ Driver        = DefaultDriver{}
	_ ClusterDriver = DefaultDriver{}
)

// CreateClient creates a single or sentinel Redis client based on c.Mode. Both
// topologies return the starter's wrapped [Client].
//
// In single mode, when c.ServiceName is set the address is resolved through the
// discovery backend (center.Discovery().Get(c.Discovery)) instead of c.Addr: a
// Resolver keeps the endpoint set fresh and the client dials a live
// instance on each new connection. Combined with c.ConnMaxLifetime, connections
// recycle onto updated addresses without rebuilding the client. When
// c.ServiceName is empty this is a plain Addr dial.
//
// In sentinel mode the client connects to the master resolved by c.MasterName
// through c.SentinelAddrs; service discovery is not used.
//
// A pick pool is built only for a discovery-routed single-mode entry; it is
// handed to [NewClient], which binds it through the center. See the [Driver]
// interface for the contract.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, center *runtime.Runtime) (*Client, error) {
	tlsConfig, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "redis: build TLS")
	}

	if c.Mode == "sentinel" {
		client := redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:       c.MasterName,
			SentinelAddrs:    c.SentinelAddrs,
			SentinelPassword: c.SentinelPassword,
			Password:         c.Password,
			DB:               c.DB,
			Username:         c.Username,
			PoolSize:         c.PoolSize,
			MaxIdleConns:     c.MaxIdle,
			ConnMaxLifetime:  c.ConnMaxLifetime,
			MaxRetries:       c.MaxRetries,
			DialTimeout:      c.DialTimeout,
			ReadTimeout:      c.ReadTimeout,
			WriteTimeout:     c.WriteTimeout,
			TLSConfig:        tlsConfig,
		})
		// Sentinel self-discovers its master; no resolver and no pick pool.
		return NewClient(client, c, nil, center)
	}

	opts := &redis.Options{
		Addr:            c.Addr,
		Password:        c.Password,
		DB:              c.DB,
		Username:        c.Username,
		PoolSize:        c.PoolSize,
		MaxIdleConns:    c.MaxIdle,
		ConnMaxLifetime: c.ConnMaxLifetime,
		MaxRetries:      c.MaxRetries,
		DialTimeout:     c.DialTimeout,
		ReadTimeout:     c.ReadTimeout,
		WriteTimeout:    c.WriteTimeout,
		TLSConfig:       tlsConfig,
	}

	// Resolve the discovery backend this entry cites. The label rides on the
	// config (c.Discovery, already carrying the family default); an entry routed
	// by service-name that cites no existing backend fails here, at construction.
	var backend discovery.Discovery
	if c.ServiceName != "" {
		backend, _ = center.Discovery().Get(c.Discovery)
		if backend == nil {
			return nil, errutil.Explain(nil, "redis: instance routes by service-name but cites discovery backend %q, which names no bean", c.Discovery)
		}
	}
	resolver, err := discovery.NewResolver(ctx, backend, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, errutil.Explain(err, "redis: discovery resolve %q failed", c.ServiceName)
	}
	if resolver != nil {
		nd := &net.Dialer{Timeout: c.DialTimeout}
		// Addr becomes a label for the chain; the dialer picks a live endpoint
		// through the shared loadbalance machinery (per the balancer config, per connection).
		// Freshness lives inside the discovery backend, so the resolver owns no
		// resources. The tracker makes outlier suspension possible, and building
		// the chain here (rather than binding a pool in [NewClient]) is what
		// registers it under the service's governance label — the same rule that
		// drives its protection executor — so the dialer below picks under the
		// selection in force.
		lbChain := center.Loadbalance().Chain(ctx, resolver, serviceLabel(c))
		opts.Addr = c.ServiceName
		opts.Dialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
			ep, err := lbChain.Pick(loadbalance.PickInfo{})
			if err != nil {
				return nil, err
			}
			conn, derr := nd.DialContext(ctx, network, ep.Addr)
			// The dial outcome is the only signal this picker has; feeding it
			// makes outlier suspension evict an instance that keeps refusing
			// connections.
			lbChain.Complete(ctx, ep, derr)
			return conn, derr
		}
		return NewClient(redis.NewClient(opts), c, lbChain, center)
	}

	return NewClient(redis.NewClient(opts), c, nil, center)
}

// CreateClusterClient creates a cluster Redis client seeded by c.Addrs. It
// returns the starter's wrapped [Client]. Cluster mode self-discovers its nodes,
// so c.ServiceName / the discovery resolver is not used and no pick pool is
// built; only the center's resilience + fault capabilities apply.
func (DefaultDriver) CreateClusterClient(ctx context.Context, c Config, center *runtime.Runtime) (*Client, error) {
	tlsConfig, err := c.TLS.BuildClient()
	if err != nil {
		return nil, errutil.Explain(err, "redis: build TLS")
	}
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:           c.Addrs,
		Password:        c.Password,
		Username:        c.Username,
		MaxRedirects:    c.MaxRedirects,
		RouteByLatency:  c.RouteByLatency,
		RouteRandomly:   c.RouteRandomly,
		PoolSize:        c.PoolSize,
		MaxIdleConns:    c.MaxIdle,
		ConnMaxLifetime: c.ConnMaxLifetime,
		MaxRetries:      c.MaxRetries,
		DialTimeout:     c.DialTimeout,
		ReadTimeout:     c.ReadTimeout,
		WriteTimeout:    c.WriteTimeout,
		TLSConfig:       tlsConfig,
	})
	// Cluster self-discovers its nodes; no resolver and no pick pool.
	return NewClient(client, c, nil, center)
}
