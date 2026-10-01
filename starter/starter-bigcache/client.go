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

// client.go is the "resource entity" concept of this starter: the Cache
// wrapper bigcache instances are injected as, its lifecycle (NewCache/Destroy),
// and the delegations back to the raw cache. The per-operation command surface
// lives in command.go.
package StarterBigCache

import (
	"github.com/allegro/bigcache/v3"
	"go-spring.org/cloud"
	"go-spring.org/cloud/resilience"
	"go.opentelemetry.io/otel/metric"
)

// Cache wraps a *bigcache.BigCache so Get/Set/Delete carry their semantic
// identity and flow through the governance executor, which is also where the
// operation's span, metrics and access log are emitted (see observe.go). It also
// owns the cache-statistics gauges. bigcache is an in-process heap cache with no
// network, so the spans are root spans (no caller context to link) and the
// durations are sub-microsecond - the value is per-key access visibility and a
// uniform signal vocabulary with the other client starters.
//
// The raw cache is an unexported field, not an embedded one: [NewCache] is the
// only way to build a Cache, so a cache can never exist without its identity, and
// the command surface (command.go) plus the delegations below are the whole API.
// There is deliberately no exported accessor for the raw cache: that would let a
// caller bypass the declaration and governance layers without it showing up in
// review.
//
// The type is exported because bigcache (unlike go-redis or gorm) offers no
// hook/plugin extension point, so the only way to observe per-operation traffic
// is to hold the wrapper itself. Apps therefore inject *Cache rather than
// *bigcache.BigCache; [Driver.CreateClient] returns this type too, so a custom
// driver works with the same type the ecosystem sees.
type Cache struct {
	// client is the raw bigcache instance. Unexported so [NewCache] is the only
	// constructor — see the type doc.
	client *bigcache.BigCache

	// stats is this starter's shared gauge set (observe.go): the cache-statistics
	// gauges. One per process - every Cache holds the same one. The per-call
	// signals are not held here: they are emitted by the resilience layer from the
	// operation each command declares (command.go).
	stats *statObserver

	// gaugeRegs is this cache's own registration of its statistics against the
	// shared gauges. It is held here because the values those gauges report come
	// from this cache: the registration's lifetime is the cache's lifetime, and
	// Destroy takes it away. Dropping it would leave the instrument reporting a
	// destroyed cache, and holding the registration would pin it.
	gaugeRegs []metric.Registration

	// instanceName is the instance name (the spring.bigcache.instances.<instanceName>
	// map key), used both for the resilience service label and as the gauge label
	// that keeps several caches in one process distinguishable. Fixed by [NewCache].
	instanceName string

	// exec is the resilience executor protecting Get/Set/Delete, applied by
	// [NewCache] in the constructor; it is the observed-only
	// resilience.Unmanaged executor when the container is absent.
	exec resilience.ClientExecutor
	// serviceLabel is the resilience service key ("bigcache:<instance-name>")
	// exec scopes limiter/breaker state by.
	serviceLabel string
}

// NewCache builds a complete Cache — identity, governance and all — over a raw
// (already created) bigcache instance, fixing its identity, installing the
// cache-statistics gauges and building the resilience executor. client is normally
// the Driver's product; instanceName is the config entry's key and both the
// resilience service label and the gauge label that keeps several caches in one
// process distinguishable.
//
// params carries the container's facilities (see [cloud.ClientParams]), and is
// applied HERE so a Cache cannot exist half-assembled: there is no Init step, no
// later patching, and nothing the container has to remember to call. A hand-built
// cache passes the zero [cloud.ClientParams]; its executor then degrades to
// resilience.Unmanaged — observed, with a one-time warning that no protection
// applies — rather than silently running bare.
//
// The manager's ClientExecutorFor resolves its backing executor lazily, on each
// Execute, so the call order relative to the center's wiring is
// irrelevant.
func NewCache(client *bigcache.BigCache, instanceName string, params cloud.ClientParams) *Cache {
	c := &Cache{
		client:       client,
		stats:        instruments(),
		instanceName: instanceName,
	}
	// Register this cache's statistics against the shared gauges, labeled with
	// this instance's name so several caches in one process stay distinguishable.
	// A registration error is dropped rather than failing construction: with no
	// OTel SDK installed the meter is a no-op and there is nothing to report,
	// which is not a reason to refuse to serve.
	if reg, err := c.stats.observeGauges(client, instanceName); err == nil {
		c.gaugeRegs = append(c.gaugeRegs, reg)
	}
	c.serviceLabel = resilience.ServiceLabel("bigcache", c.instanceName)
	c.exec = params.ExecutorFor("bigcache", c.serviceLabel)
	return c
}

// Destroy takes away this cache's gauge registration, releases the resilience
// executor (if governance was applied), and closes the underlying BigCache. It is
// the gs destroy method.
//
// The unregistration comes first: once the cache is closed its statistics are
// meaningless, and a registration left behind would both report a dead cache and
// pin it. It is also why the registration lives on the cache rather than in the
// shared gauge set - the same reason the gauges are not registered by a
// creation-time callback.
func (c *Cache) Destroy() error {
	for _, reg := range c.gaugeRegs {
		_ = reg.Unregister()
	}
	c.gaugeRegs = nil
	if c.exec != nil {
		_ = c.exec.Close()
	}
	return c.client.Close()
}

// The methods below delegate to the raw cache. They exist because the raw cache
// is an unexported field, so nothing is promoted: every method the raw
// *bigcache.BigCache exposed is re-exposed here unchanged, except the three the
// command surface overwrites with instrumented operations (Get/Set/Delete,
// command.go). They are plain pass-throughs on purpose - only Get/Set/Delete are
// business traffic worth observing; these are lifecycle/introspection helpers.

// Close signals a shutdown of the cache, letting its cleaning goroutines exit.
// Note bigcache's Close is not idempotent (it closes a channel), so calling it
// and then Destroy — or twice — panics; Destroy is the normal path.
func (c *Cache) Close() error { return c.client.Close() }

// GetWithInfo reads the entry for key together with Response info. It returns
// bigcache.ErrEntryNotFound when no entry exists for the key.
func (c *Cache) GetWithInfo(key string) ([]byte, bigcache.Response, error) {
	return c.client.GetWithInfo(key)
}

// Append appends entry under key, or sets it when the key does not exist.
func (c *Cache) Append(key string, entry []byte) error { return c.client.Append(key, entry) }

// Reset empties every cache shard.
func (c *Cache) Reset() error { return c.client.Reset() }

// ResetStats resets the cache statistics.
func (c *Cache) ResetStats() error { return c.client.ResetStats() }

// Len returns the number of entries in the cache.
func (c *Cache) Len() int { return c.client.Len() }

// Capacity returns the amount of bytes stored in the cache.
func (c *Cache) Capacity() int { return c.client.Capacity() }

// Stats returns the cache's statistics.
func (c *Cache) Stats() bigcache.Stats { return c.client.Stats() }

// KeyMetadata returns how many times a cached resource was requested.
func (c *Cache) KeyMetadata(key string) bigcache.Metadata { return c.client.KeyMetadata(key) }

// Iterator returns an iterator over every EntryInfo in the cache.
func (c *Cache) Iterator() *bigcache.EntryInfoIterator { return c.client.Iterator() }
