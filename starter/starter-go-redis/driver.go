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
	"go-spring.org/stdlib/goutil"
	"go-spring.org/stdlib/errutil"
)

// Driver interface defines how to create a single/sentinel Redis client, whose
// bean type is *redis.Client. A company (or the bundled DefaultDriver)
// implements it once and registers via RegisterDriver; callers select one
// through Config.Driver, which defaults to "DefaultDriver".
//
// The returned io.Closer is the teardown for anything the driver built beyond
// the client itself — for DefaultDriver that is the discovery resolver's
// background watch; a driver with nothing to clean up returns goutil.NopCloser().
// (*Client).Close calls it on shutdown.
type Driver interface {
	CreateClient(ctx context.Context, c Config) (*redis.Client, io.Closer, error)
}

// ClusterDriver is an optional interface a Driver may also implement to support
// cluster mode, whose bean type is *redis.ClusterClient. It is kept separate
// from Driver so existing custom drivers that only build *redis.Client continue
// to compile unchanged. The starter type-asserts to ClusterDriver only when
// Mode=cluster.
type ClusterDriver interface {
	CreateClusterClient(ctx context.Context, c Config) (*redis.ClusterClient, io.Closer, error)
}

// driverRegistry maps driver names to their implementations. The bundled
// DefaultDriver is registered at init; custom drivers add themselves via
// RegisterDriver (e.g. from an init in the application).
var driverRegistry = map[string]Driver{}

// RegisterDriver registers a Redis driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("redis driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
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
// registered discovery backend (c.Discovery) instead of c.Addr: a LiveDialer
// keeps the endpoint set fresh and the client dials a live instance on each new
// connection. Combined with c.ConnMaxLifetime, connections recycle onto updated
// addresses without rebuilding the client. When c.ServiceName is empty this is a
// plain Addr dial.
//
// In sentinel mode the client connects to the master resolved by c.MasterName
// through c.SentinelAddrs; service discovery is not used.
func (DefaultDriver) CreateClient(ctx context.Context, c Config) (*redis.Client, io.Closer, error) {
	tlsConfig, err := c.TLS.Build()
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

	resolver, err := discovery.NewResolver(ctx, c.Discovery, c.ServiceName, discovery.WithScheme(c.Scheme))
	if err != nil {
		return nil, nil, err
	}
	if resolver != nil {
		nd := &net.Dialer{Timeout: c.DialTimeout}
		// Addr becomes a label for the pool; the dialer picks a live endpoint.
		opts.Addr = c.ServiceName
		opts.Dialer = func(ctx context.Context, network, _ string) (net.Conn, error) {
			ep, err := resolver.Pick()
			if err != nil {
				return nil, err
			}
			return nd.DialContext(ctx, network, ep.Addr)
		}
	}

	// The driver owns the resolver's lifecycle: return its Stop as the teardown
	// closer so (*Client).Close can stop the background watch
	// without the starter keeping a client->resolver registry. No resolver
	// (plain Addr / mesh / sentinel) → no-op.
	client := redis.NewClient(opts)
	stop := goutil.NopCloser()
	if resolver != nil {
		stop = goutil.CloserFunc(resolver.Stop)
	}
	return client, stop, nil
}

// CreateClusterClient creates a cluster Redis client seeded by c.Addrs. The bean
// type is *redis.ClusterClient. Cluster mode self-discovers its nodes, so
// c.ServiceName / LiveDialer is not used here.
func (DefaultDriver) CreateClusterClient(ctx context.Context, c Config) (*redis.ClusterClient, io.Closer, error) {
	tlsConfig, err := c.TLS.Build()
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
