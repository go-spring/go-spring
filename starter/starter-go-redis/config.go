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
	"crypto/tls"
	"crypto/x509"
	"os"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

// liveDialers tracks the discovery-backed dialer behind each client built by
// DefaultDriver, so the destructors can stop the background watch on shutdown.
// The key is the client value (*redis.Client for single/sentinel), the value is
// the *discovery.LiveDialer. Cluster/sentinel topologies self-discover their
// nodes and never use a LiveDialer, so only single-mode clients appear here.
var liveDialers sync.Map // redis client -> *discovery.LiveDialer

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines Redis connection configuration.
type Config struct {
	// Mode selects the Redis topology: "single" (default), "sentinel", or
	// "cluster". It stays "single" by default so existing single-node
	// configurations keep working unchanged.
	//   - single:   dials Addr (or ServiceName via service discovery).
	//   - sentinel:  connects to the master resolved by MasterName through
	//                SentinelAddrs; the bean type is still *redis.Client.
	//   - cluster:   connects to the cluster seeded by Addrs; the bean type is
	//                *redis.ClusterClient (a distinct type — see README).
	Mode string `value:"${mode:=single}"`

	// Addr is the Redis server address, e.g., "127.0.0.1:6379".
	// Used only in single mode; either Addr or ServiceName must be set.
	Addr string `value:"${addr:=}"`

	// MasterName is the sentinel master group name. Required in sentinel mode.
	MasterName string `value:"${master-name:=}"`

	// SentinelAddrs are the sentinel node addresses, e.g.,
	// ["127.0.0.1:26379", "127.0.0.1:26380"]. Required in sentinel mode.
	SentinelAddrs []string `value:"${sentinel-addrs:=}"`

	// SentinelPassword is the password used to authenticate with the sentinels
	// themselves (distinct from Password, which authenticates with the master).
	SentinelPassword string `value:"${sentinel-password:=}"`

	// Addrs are the cluster seed node addresses, e.g.,
	// ["127.0.0.1:7000", "127.0.0.1:7001"]. Required in cluster mode.
	Addrs []string `value:"${addrs:=}"`

	// MaxRedirects is the maximum number of MOVED/ASK redirects to follow in
	// cluster mode, default is 0 (go-redis default of 3 applies).
	MaxRedirects int `value:"${max-redirects:=0}"`

	// RouteByLatency routes read-only commands to the lowest-latency node in
	// cluster mode. Default is false.
	RouteByLatency bool `value:"${route-by-latency:=false}"`

	// RouteRandomly routes read-only commands to a random node in cluster mode.
	// Default is false.
	RouteRandomly bool `value:"${route-randomly:=false}"`

	// Password is the Redis server password, default is empty.
	Password string `value:"${password:=}"`

	// DB is the Redis database number, default is 0.
	DB int `value:"${db:=0}"`

	// Username is the Redis ACL username, default is empty.
	Username string `value:"${username:=}"`

	// PoolSize is the maximum number of socket connections, default is 10.
	PoolSize int `value:"${pool-size:=10}"`

	// MaxIdle is the maximum number of idle connections in the pool, default is 5.
	MaxIdle int `value:"${max-idle:=5}"`

	// MaxRetries is the maximum number of retries for failed commands, default is 0.
	MaxRetries int `value:"${max-retries:=0}"`

	// DialTimeout is the timeout for dialing the Redis server, e.g., "5s".
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// ReadTimeout is the timeout for reading from Redis, e.g., "3s".
	ReadTimeout time.Duration `value:"${read-timeout:=3s}"`

	// WriteTimeout is the timeout for writing to Redis, e.g., "3s".
	WriteTimeout time.Duration `value:"${write-timeout:=3s}"`

	// ConnMaxLifetime is the maximum amount of time a connection can be reused, e.g., "2m".
	// Shorter values facilitate smoother traffic switching during service discovery updates.
	ConnMaxLifetime time.Duration `value:"${conn-max-lifetime:=2m}"`

	// ServiceName is the service discovery name for a single Redis instance.
	// When set, Addr is ignored and the actual address is resolved via service
	// discovery. It applies to single mode only: sentinel and cluster topologies
	// self-discover their nodes, so combining ServiceName with those modes is
	// rejected at startup.
	ServiceName string `value:"${service-name:=}"`

	// Discovery selects which registered discovery backend resolves ServiceName.
	// It is only consulted when ServiceName is set. A company registers its
	// naming service once via discovery.Register; the default backend name is
	// "default".
	Discovery string `value:"${discovery:=default}"`

	// TLS configures an optional TLS connection to Redis. When TLS.Enabled is
	// false (the default) the client dials in plaintext.
	TLS TLSConfig `value:"${tls}"`

	// Driver specifies which Redis driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// TLSConfig configures a TLS connection to Redis. It is only applied when
// Enabled is true.
type TLSConfig struct {
	// Enabled turns on TLS for the connection.
	Enabled bool `value:"${enabled:=false}"`

	// CertFile and KeyFile are the client certificate/key pair, used for mutual
	// TLS. Leave both empty when the server does not require a client cert.
	CertFile string `value:"${cert-file:=}"`
	KeyFile  string `value:"${key-file:=}"`

	// CAFile is a PEM bundle of root CAs used to verify the server certificate.
	// When empty the host's default root set is used.
	CAFile string `value:"${ca-file:=}"`

	// ServerName overrides the name checked against the server certificate,
	// useful when dialing by IP or through a service-discovery label.
	ServerName string `value:"${server-name:=}"`

	// InsecureSkipVerify disables server certificate verification. Intended for
	// local testing only — never enable it in production.
	InsecureSkipVerify bool `value:"${insecure-skip-verify:=false}"`
}

// buildTLSConfig turns a TLSConfig into a *tls.Config, or nil when TLS is
// disabled. It loads the client key pair and CA bundle from disk when provided.
func buildTLSConfig(c TLSConfig) (*tls.Config, error) {
	if !c.Enabled {
		return nil, nil
	}
	cfg := &tls.Config{
		ServerName:         c.ServerName,
		InsecureSkipVerify: c.InsecureSkipVerify,
	}
	if c.CertFile != "" || c.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.CertFile, c.KeyFile)
		if err != nil {
			return nil, errutil.Explain(err, "redis: failed to load TLS key pair")
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	if c.CAFile != "" {
		pem, err := os.ReadFile(c.CAFile)
		if err != nil {
			return nil, errutil.Explain(err, "redis: failed to read TLS CA file")
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, errutil.Explain(nil, "redis: no certificates found in CA file %s", c.CAFile)
		}
		cfg.RootCAs = pool
	}
	return cfg, nil
}

// Driver interface defines how to create a single/sentinel Redis client, whose
// bean type is *redis.Client.
type Driver interface {
	CreateClient(c Config) (*redis.Client, error)
}

// ClusterDriver is an optional interface a Driver may also implement to support
// cluster mode, whose bean type is *redis.ClusterClient. It is kept separate
// from Driver so existing custom drivers that only build *redis.Client continue
// to compile unchanged. The starter type-asserts to ClusterDriver only when
// Mode=cluster.
type ClusterDriver interface {
	CreateClusterClient(c Config) (*redis.ClusterClient, error)
}

// RegisterDriver registers a Redis driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("redis driver already registered: " + name)
	}
	driverRegistry[name] = driver
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
func (DefaultDriver) CreateClient(c Config) (*redis.Client, error) {
	tlsConfig, err := buildTLSConfig(c.TLS)
	if err != nil {
		return nil, err
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
		return client, nil
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

	var ld *discovery.LiveDialer
	if c.ServiceName != "" {
		d, err := discovery.MustGet(c.Discovery)
		if err != nil {
			return nil, err
		}
		ld, err = discovery.NewLiveDialer(context.Background(), d, c.ServiceName)
		if err != nil {
			return nil, err
		}
		// Addr becomes a label for the pool; the dialer picks a live endpoint.
		opts.Addr = c.ServiceName
		opts.Dialer = ld.DialContext
	}

	client := redis.NewClient(opts)
	if ld != nil {
		liveDialers.Store(client, ld)
	}
	return client, nil
}

// CreateClusterClient creates a cluster Redis client seeded by c.Addrs. The bean
// type is *redis.ClusterClient. Cluster mode self-discovers its nodes, so
// c.ServiceName / LiveDialer is not used here.
func (DefaultDriver) CreateClusterClient(c Config) (*redis.ClusterClient, error) {
	tlsConfig, err := buildTLSConfig(c.TLS)
	if err != nil {
		return nil, err
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
	return client, nil
}
