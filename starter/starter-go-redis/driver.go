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
	"go-spring.org/cloud"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
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
// (with the pick pool it built, if any, and params) to [NewClient], which fixes
// the identity, installs the redisotel pool-metrics + declaration layers, and
// applies the governance executor and pool binding. Nothing reaches into the
// client afterwards.
//
// params carries the container's facilities (see [cloud.ClientParams]): a
// discovery-routed driver builds its [loadbalance.Pool], passes it — along with
// params — to [NewClient], and forgets about it. It is a single struct rather
// than a parameter per capability so this interface — which every company driver
// implements — stays stable as capabilities are added.
//
// params.Discovery is the discovery backend the entry's ${discovery} label
// resolved to, already looked up by the starter wiring; it is nil when the entry
// does not use service discovery (an unknown label fails at wiring, before the
// driver is called). It rides on the params struct rather than Config so a custom
// driver can actually reach it — Config stays a pure bound value.
type Driver interface {
	CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error)
}

// ClusterDriver is an optional interface a Driver may also implement to support
// cluster mode. It is kept separate from Driver so a Driver bean that only
// builds single/sentinel clients stays valid: the starter type-asserts to
// ClusterDriver only for instances with Mode=cluster.
//
// Cluster mode self-discovers through the cluster's own topology and the client
// seeds dialing with c.Addrs — there is no per-endpoint pick pool to hand back —
// so the pool argument to [NewClient] is nil on this path, and only params'
// resilience + fault capabilities apply.
type ClusterDriver interface {
	CreateClusterClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error)
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
// discovery backend (params.Discovery, wired from the ${discovery} label) instead
// of c.Addr: a Resolver keeps the endpoint set fresh and the client dials a live
// instance on each new connection. Combined with c.ConnMaxLifetime, connections
// recycle onto updated addresses without rebuilding the client. When
// c.ServiceName is empty this is a plain Addr dial.
//
// In sentinel mode the client connects to the master resolved by c.MasterName
// through c.SentinelAddrs; service discovery is not used.
//
// A pick pool is built only for a discovery-routed single-mode entry; it is
// handed to [NewClient], which binds it through params. See the [Driver]
// interface for the contract.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error) {
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
		return NewClient(client, c, nil, params)
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

	resolver, err := discovery.NewResolver(ctx, params.Discovery, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, errutil.Explain(err, "redis: discovery resolve %q failed", c.ServiceName)
	}
	if resolver != nil {
		nd := &net.Dialer{Timeout: c.DialTimeout}
		// Addr becomes a label for the pool; the dialer picks a live endpoint
		// through the shared loadbalance machinery (per the balancer config, per connection).
		// Freshness lives inside the discovery backend, so the resolver owns no
		// resources. The tracker makes outlier suspension possible; the pool is
		// handed to [NewClient], which binds it to governance so the service's
		// endpoint selection follows the same rule that drives its protection
		// executor.
		bal := loadbalance.NewRoundRobin()
		lb := loadbalance.NewPool(resolver, bal)
		opts.Addr = c.ServiceName
		opts.Dialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
			ep, err := lb.Pick(loadbalance.PickInfo{})
			if err != nil {
				return nil, err
			}
			conn, derr := nd.DialContext(ctx, network, ep.Addr)
			// The dial outcome is the only signal this picker has; feeding it
			// makes outlier suspension evict an instance that keeps refusing
			// connections.
			lb.Complete(ctx, ep, derr)
			return conn, derr
		}
		return NewClient(redis.NewClient(opts), c, lb, params)
	}

	return NewClient(redis.NewClient(opts), c, nil, params)
}

// CreateClusterClient creates a cluster Redis client seeded by c.Addrs. It
// returns the starter's wrapped [Client]. Cluster mode self-discovers its nodes,
// so c.ServiceName / the discovery resolver is not used and no pick pool is
// built; only params' resilience + fault capabilities apply.
func (DefaultDriver) CreateClusterClient(ctx context.Context, c Config, params cloud.ClientParams) (*Client, error) {
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
	return NewClient(client, c, nil, params)
}
