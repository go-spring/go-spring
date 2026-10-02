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
	"go-spring.org/stdlib/errutil"
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
// (spring.bigcache.instances.<name>) and becomes the label every signal carries,
// so the driver sets it on the wrapper it builds. Nothing patches the cache
// afterwards.
//
// A driver attributes its own construction failures: the error it returns is
// passed through to the container unwrapped, so it must name the stage that
// failed and the instance it was building — `errutil.Explain(err, "bigcache:
// create instance %q", name)` in [DefaultDriver]. The container adds the bean
// name and the registration's file:line, but only the driver can say which step
// inside the assembly went wrong.
//
// Unlike the network client starters, this signature carries no
// `cloud.ClientParams`: those bundle the container's governance authorities
// (resilience, fault, loadbalance, discovery), and bigcache has no external
// dependency for any of them to act on — it does not route through the executor
// chain at all, emitting its own signals instead (see [statObserver]). Handing a
// driver facilities it must never use would be a seam with nothing behind it.
//
// name is passed as an argument rather than carried on Config: Config stays a
// pure projection of the entry's properties, and the instance name is the map
// key, not a value in it.
//
// At most one Driver bean is expected per process; every instance under
// ${spring.bigcache} is built through it, and per-instance differences are
// expressed through [Config].
type Driver interface {
	CreateClient(ctx context.Context, name string, c Config) (*Cache, error)
}

// DefaultDriver is the default implementation of the Driver interface.
type DefaultDriver struct{}

// CreateClient creates a new BigCache instance based on the provided configuration.
//
// The raw cache is built here and immediately wrapped by [NewCache], which fixes
// its identity and its observability: the returned cache is complete in one step.
// If the wrapping fails the raw cache is closed before returning, so a failed
// construction never leaves the eviction goroutine behind.
func (DefaultDriver) CreateClient(ctx context.Context, name string, c Config) (*Cache, error) {
	conf := bigcache.DefaultConfig(c.LifeWindow)
	conf.Shards = c.Shards
	conf.CleanWindow = c.CleanWindow
	conf.MaxEntriesInWindow = c.MaxEntriesInWindow
	conf.MaxEntrySize = c.MaxEntrySize
	conf.HardMaxCacheSize = c.HardMaxCacheSize
	conf.StatsEnabled = c.StatsEnabled
	client, err := bigcache.New(ctx, conf)
	if err != nil {
		return nil, errutil.Explain(err, "bigcache: create instance %q", name)
	}
	// NewCache is the only way to build a Cache: identity and observation are
	// both applied here (see [Driver]), so the driver returns a cache that is
	// complete.
	cache, err := NewCache(client, name)
	if err != nil {
		// The raw cache was built and is running - with its background eviction
		// goroutine. Nothing else holds it, so it is this branch's to close.
		_ = client.Close()
		return nil, err
	}
	return cache, nil
}
