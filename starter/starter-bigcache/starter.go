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

package StarterBigCache

import (
	"context"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud/actuator/health"
	"go-spring.org/cloud/cache"
	"go-spring.org/log"
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/starter-bigcache/bytecache"
	health2 "go-spring.org/starter-bigcache/health"
	StarterCache "go-spring.org/starter-cache"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func init() {
	// Register multiple BigCache instances as a group, one per entry under
	// "${spring.bigcache}". A gs.Module (rather than gs.Group) is used so each
	// instance's name is available to label its OTel metrics - and to attach
	// the file:line of this registration to the bean for diagnostics.
	//
	// BigCache spawns a background eviction goroutine, so Close must be called
	// on shutdown to release it - the destroy callback handles that.
	gs.Module(gs.OnProperty("spring.bigcache"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.bigcache}", func(name string, c Config) error {
			// IndexArg places name (index 1) and c (index 2) explicitly, leaving
			// index 0 (*gs.ContextProvider) to be autowired - the documented
			// pattern for a ctor whose first param is ContextProvider.
			r.Provide(newClient,
				gs.IndexArg(1, gs.ValueArg(name)),
				gs.IndexArg(2, gs.ValueArg(c)),
			).Name(name).Init((*Cache).Init).Destroy((*Cache).Destroy).Caller(1)
			// Contribute a health indicator for this instance, injecting the
			// client just registered above by name.
			r.Provide(func(c *Cache) *health.Indicator { return health2.NewBigCacheHealth(name, c.BigCache) }, gs.TagArg(name)).Name("bigcache:" + name).Caller(1)
			return nil
		})
	})

	// init registers the "bigcache" cache driver so a *bigcache.BigCache registered
	// under ${spring.bigcache} can be exposed as a cache.Cache via:
	//
	//	spring.cache.<name>.driver = bigcache:<bigcache-instance-name>
	//
	// The beanID selects which BigCache bean to wrap; the implementation lives in
	// starter-bigcache/bytecache.
	StarterCache.RegisterDriver("bigcache", func(beanID string) gs.ModuleFunc {
		return func(r gs.BeanProvider, p flatten.Storage) error {
			r.Provide(func(c *Cache) *cache.Cache {
				return cache.New(bytecache.NewByteCache(c.BigCache))
			}, gs.TagArg(beanID)).Name(beanID)
			return nil
		}
	})
}

// newClient creates a new BigCache instance based on the provided configuration,
// wrapped so Get/Set/Delete flow through the module-local observe layer, and registers OTel
// gauges for its statistics, labeled by the instance name.
func newClient(ctx *gs.ContextProvider, name string, c Config) (*Cache, error) {
	log.Debugf(ctx.Context, log.TagAppDef, "creating bigcache instance, name=%s shards=%d max-size=%d", name, c.Shards, c.MaxEntrySize)

	d, ok := driverRegistry[c.Driver]
	if !ok {
		log.Errorf(ctx.Context, log.TagAppDef, "bigcache driver not found: %s", c.Driver)
		return nil, errutil.Explain(nil, "bigcache driver not found: %s", c.Driver)
	}
	client, err := d.CreateClient(ctx.Context, c)
	if err != nil {
		log.Errorf(ctx.Context, log.TagAppDef, "bigcache: create instance failed: %v", err)
		return nil, errutil.Explain(err, "failed to create bigcache instance")
	}
	// Surface hits/misses/collisions/capacity as OTel gauges. Safe no-op when
	// starter-otel is absent (the OTel globals are no-ops then).
	registerMetrics(name, client)
	log.Infof(ctx.Context, log.TagAppDef, "bigcache instance initialized, name=%s shards=%d", name, c.Shards)
	// Return the wrapper; gs calls Init (InitMethod) after this returns to
	// build the observer + executor.
	return &Cache{BigCache: client, name: name}, nil
}

// metricsMeter is the OTel meter all bigcache instruments register under. It
// follows the starter's module path, matching the convention used by other
// starters (e.g. starter-gin's "go-spring.org/starter-gin").
const metricsMeter = "go-spring.org/starter-bigcache"

// statReader reads one snapshot value from a BigCache instance.
type statReader func(*bigcache.BigCache) int64

// statInstrument describes one gauge to register.
type statInstrument struct {
	name string
	desc string
	read statReader
}

// statInstruments lists the bigcache statistics surfaced as gauges. The five
// counters come from Stats() (cumulative); entries/capacity come from Len()/
// Capacity() (current). All are gauges rather than counters because
// [bigcache.BigCache.ResetStats] can reset the counters, breaking monotonicity.
var statInstruments = []statInstrument{
	{name: "bigcache.hits", desc: "Number of successfully found keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().Hits }},
	{name: "bigcache.misses", desc: "Number of not found keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().Misses }},
	{name: "bigcache.delete_hits", desc: "Number of successfully deleted keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().DelHits }},
	{name: "bigcache.delete_misses", desc: "Number of not deleted keys", read: func(c *bigcache.BigCache) int64 { return c.Stats().DelMisses }},
	{name: "bigcache.collisions", desc: "Number of key hash collisions", read: func(c *bigcache.BigCache) int64 { return c.Stats().Collisions }},
	{name: "bigcache.entries", desc: "Current number of stored entries", read: func(c *bigcache.BigCache) int64 { return int64(c.Len()) }},
	{name: "bigcache.capacity", desc: "Maximum number of entries the cache can hold", read: func(c *bigcache.BigCache) int64 { return int64(c.Capacity()) }},
}

// registerMetrics registers OTel observable gauges that surface a BigCache
// instance's statistics (hits, misses, collisions) and capacity, labeled with
// the instance name so multiple instances (e.g. "hot", "cold") are
// distinguishable in a metrics backend. Each gauge pulls its value on
// collection via a callback - no per-operation overhead and no background
// goroutine.
//
// When starter-otel is imported the gauges are exported through the global
// meter provider it installs (prometheus pull, OTLP, ...); when it is absent
// the OTel globals are no-ops, so this call is safe and cheap to always make.
func registerMetrics(name string, c *bigcache.BigCache) {
	meter := otel.Meter(metricsMeter)
	attrs := metric.WithAttributes(attribute.String("cache.name", name))
	for _, inst := range statInstruments {
		_, _ = meter.Int64ObservableGauge(inst.name,
			metric.WithDescription(inst.desc),
			metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
				o.Observe(inst.read(c), attrs)
				return nil
			}),
		)
	}
}
