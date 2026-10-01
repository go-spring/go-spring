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

// driver.go is the "construction seam" concept of this starter: the Driver
// interface + DefaultDriver owns full client assembly. The Driver is an OPTIONAL
// container bean: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterBigCache.Driver); when none is present,
// starter-bigcache falls back to the bundled [DefaultDriver] inside client
// assembly. Because a custom driver is a bean, it may inject the configuration
// it needs at wiring time.
package StarterBigCache

import (
	"context"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud"
)

// Driver interface defines how to create a BigCache instance. It is an OPTIONAL
// CONTAINER BEAN: a company or umbrella starter may provide its own Driver bean
// (its constructor returns StarterBigCache.Driver); when none is present,
// starter-bigcache falls back to the bundled [DefaultDriver] inside client
// assembly.
//
// CreateClient returns the module's exported [Cache] — the wrapper apps inject —
// not the raw *bigcache.BigCache, so a driver takes part in the type the rest of
// the ecosystem sees and future wrapper capabilities are reachable from it. It
// returns the cache COMPLETE: name is the config entry's key
// (spring.bigcache.instances.<name>) and becomes both the resilience service
// label and the gauge label, so the driver sets it on the wrapper it builds; and
// params carries the container's facilities (see [cloud.ClientParams]), which
// [NewCache] applies while building. Nothing patches the cache afterwards.
//
// params is one struct rather than a parameter per capability so this interface —
// which every company driver implements — stays stable as capabilities are
// added. A driver that has no use for one of its fields simply ignores it.
//
// name is passed as an argument rather than carried on Config: Config stays a
// pure projection of the entry's properties, and the instance name is the map
// key, not a value in it.
//
// At most one Driver bean is expected per process; every instance under
// ${spring.bigcache} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, name string, c Config, params cloud.ClientParams) (*Cache, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new BigCache instance based on the provided configuration.
//
// The raw cache is built here and immediately wrapped by [NewCache], which also
// applies governance: the returned cache is complete in one step.
func (DefaultDriver) CreateClient(ctx context.Context, name string, c Config, params cloud.ClientParams) (*Cache, error) {
	conf := bigcache.DefaultConfig(c.LifeWindow)
	conf.Shards = c.Shards
	conf.CleanWindow = c.CleanWindow
	conf.MaxEntriesInWindow = c.MaxEntriesInWindow
	conf.MaxEntrySize = c.MaxEntrySize
	conf.HardMaxCacheSize = c.HardMaxCacheSize
	conf.StatsEnabled = c.StatsEnabled
	client, err := bigcache.New(ctx, conf)
	if err != nil {
		return nil, err
	}
	// NewCache is the only way to build a Cache: identity and governance are
	// both applied here (see [Driver]), so the driver returns a cache that is
	// complete.
	return NewCache(client, name, params), nil
}
