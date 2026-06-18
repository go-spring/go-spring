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
	"time"

	"github.com/redis/go-redis/v9"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines Redis connection configuration.
type Config struct {
	// Addr is the Redis server address, e.g., "127.0.0.1:6379".
	Addr string `value:"${addr}"`

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

	// ServiceName is the service discovery name for Redis cluster.
	// When set, Addr is ignored and the actual address is resolved via service discovery.
	ServiceName string `value:"${service-name:=}"`

	// Driver specifies which Redis driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// Driver interface defines how to create a Redis client.
type Driver interface {
	CreateClient(c Config) (*redis.Client, error)
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
func (DefaultDriver) CreateClient(c Config) (*redis.Client, error) {
	return redis.NewClient(&redis.Options{
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
	}), nil
}
