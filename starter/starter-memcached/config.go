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

package StarterMemcached

import (
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines Memcached client connection configuration.
type Config struct {
	// Servers is the list of Memcached server addresses to connect to,
	// e.g., "127.0.0.1:11211". Requests are sharded across the servers.
	Servers []string `value:"${servers}"`

	// Timeout is the socket read/write timeout for each request,
	// 0 uses the driver default (100ms), e.g., "100ms".
	Timeout time.Duration `value:"${timeout:=0}"`

	// MaxIdleConns is the maximum number of idle connections kept per server,
	// 0 uses the driver default (2).
	MaxIdleConns int `value:"${max-idle-conns:=0}"`

	// Driver specifies which Memcached driver to use, defaults to DefaultDriver.
	Driver string `value:"${driver:=DefaultDriver}"`
}

// Driver interface defines how to create a Memcached client.
type Driver interface {
	CreateClient(c Config) (*memcache.Client, error)
}

// RegisterDriver registers a Memcached driver with the given name.
// It panics if the driver name has already been registered.
func RegisterDriver(name string, driver Driver) {
	if _, ok := driverRegistry[name]; ok {
		panic("memcached driver already registered: " + name)
	}
	driverRegistry[name] = driver
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new Memcached client based on the provided configuration.
func (DefaultDriver) CreateClient(c Config) (*memcache.Client, error) {
	client := memcache.New(c.Servers...)
	if c.Timeout > 0 {
		client.Timeout = c.Timeout
	}
	if c.MaxIdleConns > 0 {
		client.MaxIdleConns = c.MaxIdleConns
	}
	return client, nil
}
