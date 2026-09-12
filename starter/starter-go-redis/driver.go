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
	"io"
	"net"

	"github.com/redis/go-redis/v9"
	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/goutil"
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
// The returned io.Closer is the teardown for anything the driver built beyond
// the client itself — for DefaultDriver that is the discovery resolver's
// background watch; a driver with nothing to clean up returns goutil.NopCloser().
// (*Client).Close calls it on shutdown.
//
// backend is the discovery backend the entry's ${discovery} label resolved to,
// already looked up by the starter wiring; it is nil when the entry does not use
// service discovery (an unknown label fails at wiring, before the driver is
// called). It is passed as an argument rather than carried on Config so a custom
// driver can actually reach it — Config stays a pure bound value.
type Driver interface {
	CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*redis.Client, io.Closer, error)
}

// ClusterDriver is an optional interface a Driver may also implement to support
// cluster mode, whose bean type is *redis.ClusterClient. It is kept separate
// from Driver so a Driver bean that only builds *redis.Client stays valid: the
// starter type-asserts to ClusterDriver only for instances with Mode=cluster.
type ClusterDriver interface {
	CreateClusterClient(ctx context.Context, c Config) (*redis.ClusterClient, io.Closer, error)
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
func (DefaultDriver) CreateClient(ctx context.Context, c Config, backend discovery.Discovery) (*redis.Client, io.Closer, error) {
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
		// Sentinel self-discovers its master; no background resolver to stop.
		return client, goutil.NopCloser(), nil
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
		// through the shared loadbalance machinery (round-robin, per connection).
		// Freshness lives inside the discovery backend, so the resolver has no
		// resources to release.
		bal, err := loadbalance.New(loadbalance.RoundRobin)
		if err != nil {
			return nil, nil, err
		}
		lb := loadbalance.NewPool(loadbalance.SourceFunc(resolver), bal)
		opts.Addr = c.ServiceName
		opts.Dialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
			ep, err := lb.Pick(loadbalance.PickInfo{})
			if err != nil {
				return nil, err
			}
			return nd.DialContext(ctx, network, ep.Addr)
		}
	}

	client := redis.NewClient(opts)
	return client, goutil.NopCloser(), nil
}

// CreateClusterClient creates a cluster Redis client seeded by c.Addrs. The bean
// type is *redis.ClusterClient. Cluster mode self-discovers its nodes, so
// c.ServiceName / the discovery resolver is not used here.
func (DefaultDriver) CreateClusterClient(ctx context.Context, c Config) (*redis.ClusterClient, io.Closer, error) {
	tlsConfig, err := c.TLS.BuildClient()
	if err != nil {
		return nil, nil, errutil.Explain(err, "redis: build TLS")
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
	// Cluster self-discovers its nodes; no background resolver to stop.
	return client, goutil.NopCloser(), nil
}
