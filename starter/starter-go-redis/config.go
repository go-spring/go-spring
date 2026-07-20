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
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go-spring.org/spring/cloud/discovery"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/spring/cloud/resilience"
	"go-spring.org/spring/cloud/tlsconf"
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
	TLS tlsconf.TLSConfig `value:"${tls}"`

	// Resilience optionally protects Redis commands with rate limiting and
	// circuit breaking. It is disabled by default; when enabled a redis.Hook is
	// attached so every command flows through the selected resilience driver.
	// Retry is best left to go-redis's own MaxRetries, so leave resilience
	// max-retries at 0 to avoid re-sending non-idempotent commands.
	Resilience ResilienceConfig `value:"${resilience:=}"`

	// Driver specifies which Redis driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// ResilienceConfig binds the backend-neutral resilience knobs exposed by
// stdlib/resilience. Driver selects which registered backend enforces them:
// "default" (bundled, zero-dependency) or "sentinel" (recommended, enabled by
// blank-importing starter-resilience). Switching backends is a one-line config
// change — no code touches the hook seam.
type ResilienceConfig struct {
	// Enabled attaches the resilience hook. When false the client is unchanged.
	Enabled bool `value:"${enabled:=false}"`

	// Driver names the registered resilience backend to use.
	Driver string `value:"${driver:=default}"`

	// RateLimit caps sustained throughput in commands per second (0 disables).
	RateLimit float64 `value:"${rate-limit:=0}"`

	// Burst is the momentary allowance above RateLimit (0 = driver default).
	Burst int `value:"${burst:=0}"`

	// ErrorThreshold is the consecutive-failure count that trips the breaker
	// open (0 disables circuit breaking). redis.Nil (cache miss) never counts.
	ErrorThreshold int `value:"${error-threshold:=0}"`

	// OpenDuration is how long the breaker stays open before a trial command.
	OpenDuration time.Duration `value:"${open-duration:=0}"`

	// MaxRetries is the number of extra attempts after a failure. Keep 0 for
	// Redis; go-redis already retries via its own MaxRetries.
	MaxRetries int `value:"${max-retries:=0}"`

	// AttemptTimeout bounds each individual attempt (0 = no per-attempt bound).
	AttemptTimeout time.Duration `value:"${attempt-timeout:=0}"`
}

// policy maps the bound config onto the backend-neutral resilience.Policy.
func (r ResilienceConfig) policy() resilience.Policy {
	return resilience.Policy{
		RateLimit:      r.RateLimit,
		Burst:          r.Burst,
		ErrorThreshold: r.ErrorThreshold,
		OpenDuration:   r.OpenDuration,
		MaxRetries:     r.MaxRetries,
		Timeout:        r.AttemptTimeout,
	}
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
	tlsConfig, err := c.TLS.Build()
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
		// NewClientDialer centralizes the discovery/mesh decision: normally it
		// resolves c.Discovery and keeps the endpoint set fresh; in mesh mode it
		// skips the backend and dials the stable Service address for the sidecar.
		ld, err = discovery.NewClientDialer(context.Background(), c.Discovery, c.ServiceName)
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
	tlsConfig, err := c.TLS.Build()
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
	return client, nil
}
