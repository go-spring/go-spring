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
	"context"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/stdlib/discovery"
	"go-spring.org/stdlib/errutil"
)

var driverRegistry = map[string]Driver{}

func init() {
	RegisterDriver("DefaultDriver", DefaultDriver{})
}

// Config defines Memcached client connection configuration.
type Config struct {
	// Servers is the list of Memcached server addresses to connect to,
	// e.g., "127.0.0.1:11211". Requests are sharded across the servers.
	// Either Servers or ServiceName must be set.
	Servers []string `value:"${servers:=}"`

	// ServiceName is the service discovery name for the Memcached cluster. When
	// set, Servers is ignored and the server list is resolved once at startup
	// through the registered discovery backend.
	//
	// Note: gomemcache shards keys across a static server set chosen at client
	// creation, so — unlike the dialer-based redis starters — the address list
	// is resolved a single time at boot (fail-fast) rather than kept live. A
	// changing cluster membership requires a restart to pick up.
	ServiceName string `value:"${service-name:=}"`

	// Discovery selects which registered discovery backend resolves ServiceName.
	// It is only consulted when ServiceName is set; the default backend name is
	// "default".
	Discovery string `value:"${discovery:=default}"`

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
//
// When c.ServiceName is set, the server list is resolved once through the
// registered discovery backend (c.Discovery) instead of using c.Servers. This
// is a one-shot resolve at startup: gomemcache hashes keys onto a fixed server
// set, so the membership is fixed for the client's lifetime.
func (DefaultDriver) CreateClient(c Config) (*memcache.Client, error) {
	servers := c.Servers
	if c.ServiceName != "" {
		d, err := discovery.MustGet(c.Discovery)
		if err != nil {
			return nil, err
		}
		eps, err := d.Resolve(context.Background(), c.ServiceName)
		if err != nil {
			return nil, errutil.Explain(err, "memcached: discovery resolve %q failed", c.ServiceName)
		}
		if len(eps) == 0 {
			return nil, errutil.Explain(nil, "memcached: discovery returned no endpoints for %q", c.ServiceName)
		}
		servers = make([]string, 0, len(eps))
		for _, ep := range eps {
			servers = append(servers, ep.Addr)
		}
	}
	client := memcache.New(servers...)
	if c.Timeout > 0 {
		client.Timeout = c.Timeout
	}
	if c.MaxIdleConns > 0 {
		client.MaxIdleConns = c.MaxIdleConns
	}
	return client, nil
}
