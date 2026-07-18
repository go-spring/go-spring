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

package StarterRedigo

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

// liveDialers tracks the discovery-backed dialer behind each pool built by
// DefaultDriver, so destroyClient can stop the background watch on shutdown.
var liveDialers sync.Map // *redis.Pool -> *discovery.LiveDialer

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines Redis connection configuration.
type Config struct {
	// Addr is the Redis server address, e.g., "127.0.0.1:6379".
	// Either Addr or ServiceName must be set.
	Addr string `value:"${addr:=}"`

	// Password is the Redis server password, default is empty.
	Password string `value:"${password:=}"`

	// DB is the Redis database number, default is 0.
	DB int `value:"${db:=0}"`

	// Username is the Redis ACL username, default is empty.
	Username string `value:"${username:=}"`

	// PoolSize is the maximum number of connections allocated by the pool at a given time.
	// When zero, there is no limit on the number of connections in the pool.
	PoolSize int `value:"${pool-size:=10}"`

	// MaxIdle is the maximum number of idle connections in the pool.
	MaxIdle int `value:"${max-idle:=5}"`

	// DialTimeout is the timeout for dialing the Redis server, e.g., "5s".
	DialTimeout time.Duration `value:"${dial-timeout:=5s}"`

	// ReadTimeout is the timeout for reading from Redis, e.g., "3s".
	ReadTimeout time.Duration `value:"${read-timeout:=3s}"`

	// WriteTimeout is the timeout for writing to Redis, e.g., "3s".
	WriteTimeout time.Duration `value:"${write-timeout:=3s}"`

	// ConnMaxLifetime is the maximum amount of time a connection can be reused, e.g., "2m".
	// Shorter values facilitate smoother traffic switching during service discovery updates.
	ConnMaxLifetime time.Duration `value:"${conn-max-lifetime:=2m}"`

	// ServiceName is the service discovery name for Redis cluster.
	// When set, Addr is ignored and the actual address is resolved via service discovery.
	ServiceName string `value:"${service-name:=}"`

	// Discovery selects which registered discovery backend resolves ServiceName.
	// It is only consulted when ServiceName is set. A company registers its
	// naming service once via discovery.Register; the default backend name is
	// "default". Field layout matches starter-go-redis.
	Discovery string `value:"${discovery:=default}"`

	// TLS configures an optional TLS connection to Redis. When TLS.Enabled is
	// false (the default) the client dials in plaintext. Field layout matches
	// starter-go-redis so the two starters stay interchangeable.
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

// Driver interface defines how to create a Redis client.
type Driver interface {
	CreateClient(c Config) (*redis.Pool, error)
}

// RegisterDriver registers a Redis driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("redis driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Redis client based on the provided configuration.
//
// When c.ServiceName is set, the address is resolved through the registered
// discovery backend (c.Discovery) instead of c.Addr: a LiveDialer keeps the
// endpoint set fresh and the pool dials a live instance for each new connection.
// Combined with c.ConnMaxLifetime, pooled connections recycle onto updated
// addresses without rebuilding the pool. When c.ServiceName is empty this is a
// plain Addr dial, unchanged from before.
func (DefaultDriver) CreateClient(c Config) (*redis.Pool, error) {
	tlsConfig, err := buildTLSConfig(c.TLS)
	if err != nil {
		return nil, err
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
	}

	pool := &redis.Pool{
		MaxActive:       c.PoolSize,
		MaxIdle:         c.MaxIdle,
		MaxConnLifetime: c.ConnMaxLifetime,
		Wait:            true,
		Dial: func() (redis.Conn, error) {
			opts := []redis.DialOption{
				redis.DialPassword(c.Password),
				redis.DialConnectTimeout(c.DialTimeout),
				redis.DialReadTimeout(c.ReadTimeout),
				redis.DialWriteTimeout(c.WriteTimeout),
			}
			if c.Username != "" {
				opts = append(opts, redis.DialUsername(c.Username))
			}
			if tlsConfig != nil {
				opts = append(opts,
					redis.DialUseTLS(true),
					redis.DialTLSConfig(tlsConfig),
					redis.DialTLSSkipVerify(c.TLS.InsecureSkipVerify),
				)
			}
			// With service discovery the LiveDialer picks a live endpoint and
			// ignores the static c.Addr passed below.
			if ld != nil {
				opts = append(opts, redis.DialContextFunc(ld.DialContext))
			}
			conn, err := redis.Dial("tcp", c.Addr, opts...)
			if err != nil {
				return nil, err
			}
			if c.DB != 0 {
				_, err = conn.Do("SELECT", c.DB)
				if err != nil {
					conn.Close()
					return nil, err
				}
			}
			return conn, nil
		},
	}
	if ld != nil {
		liveDialers.Store(pool, ld)
	}
	return pool, nil
}
