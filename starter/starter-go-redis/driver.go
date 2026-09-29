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
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a single/sentinel Redis client, whose
// bean type is *redis.Client. It is an OPTIONAL CONTAINER BEAN: a company or
// umbrella starter may provide its own Driver bean (its constructor returns
// StarterGoRedis.Driver); when none is present, starter-go-redis falls back to
// the bundled [DefaultDriver] inside client assembly. A custom driver is a bean,
// so it may inject the configuration/beans it needs — e.g. company config bound
// from a properties file at wiring time.
//
// At most one Driver bean is expected per process. Because single-Driver-bean
// selection is process-wide, every ${spring.go-redis} instance is built through
// it regardless of Mode; a service that also uses cluster mode must provide a
// Driver that implements [ClusterDriver] too.
//
// backend is the discovery backend the entry's ${discovery} label resolved to,
// already looked up by the starter wiring; it is nil when the entry does not use
// service discovery (an unknown label fails at wiring, before the driver is
// called). It is passed as an argument rather than carried on Config so a custom
// driver can actually reach it — Config stays a pure bound value.
//
// The returned *loadbalance.Pool is the endpoint-selection pool the driver built
// for a discovery-routed entry, or nil when it built none. The driver does NOT
// bind it to governance — it hands it back so the caller can, which keeps a
// company Driver entirely unaware of governance: build a pool if your topology
// needs one, return it, forget about it. The starter binds it to the injected
// [loadbalance.Manager] and folds the detach into [Client.Destroy].
type Driver interface {
	CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*redis.Client, *loadbalance.Pool, error)
}

// ClusterDriver is an optional interface a Driver may also implement to support
// cluster mode, whose bean type is *redis.ClusterClient. It is kept separate
// from Driver so a Driver bean that only builds *redis.Client stays valid: the
// starter type-asserts to ClusterDriver only for instances with Mode=cluster.
//
// It has no pool return because cluster mode self-discovers through the
// cluster's own topology and the client seeds dialing with c.Addrs — there is no
// per-endpoint pick pool to hand back.
type ClusterDriver interface {
	CreateClusterClient(ctx context.Context, c Config) (*redis.ClusterClient, error)
}

// DefaultDriver is the default implementation of the Driver interface. It also
// implements ClusterDriver, so it can build all three topologies.
type DefaultDriver struct{}

var (
	_ Driver        = DefaultDriver{}
	_ ClusterDriver = DefaultDriver{}
)

// CreateClient creates a single or sentinel Redis client based on c.Mode. Both
// topologies return *redis.Client.
//
// In single mode, when c.ServiceName is set the address is resolved through the
// discovery backend (backend, wired from the ${discovery} label) instead of
// c.Addr: a Resolver keeps the endpoint set fresh and the client dials a live
// instance on each new connection. Combined with c.ConnMaxLifetime, connections
// recycle onto updated addresses without rebuilding the client. When
// c.ServiceName is empty this is a plain Addr dial.
//
// In sentinel mode the client connects to the master resolved by c.MasterName
// through c.SentinelAddrs; service discovery is not used.
//
// The returned pool is non-nil only for a discovery-routed single-mode entry;
// see the Driver interface for the contract.
func (DefaultDriver) CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*redis.Client, *loadbalance.Pool, error) {
	tlsConfig, err := c.TLS.BuildClient()
	if err != nil {
		return nil, nil, errutil.Explain(err, "redis: build TLS")
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
		return client, nil, nil
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

	resolver, err := discovery.NewResolver(ctx, backend, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, nil, err
	}
	if resolver != nil {
		nd := &net.Dialer{Timeout: c.DialTimeout}
		// Addr becomes a label for the pool; the dialer picks a live endpoint
		// through the shared loadbalance machinery (per the balancer config, per connection).
		// Freshness lives inside the discovery backend, so the resolver owns no
		// resources. The tracker makes outlier suspension possible; the pool is
		// handed back to the caller, which binds it to governance so the
		// service's endpoint selection follows the same rule that drives its
		// protection executor.
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
			lb.Complete(ep, derr)
			return conn, derr
		}
		client := redis.NewClient(opts)
		return client, lb, nil
	}

	client := redis.NewClient(opts)
	return client, nil, nil
}

// CreateClusterClient creates a cluster Redis client seeded by c.Addrs. The bean
// type is *redis.ClusterClient. Cluster mode self-discovers its nodes, so
// c.ServiceName / the discovery resolver is not used here.
func (DefaultDriver) CreateClusterClient(ctx context.Context, c Config) (*redis.ClusterClient, error) {
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
	return client, nil
}
