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

// client.go is the "resource entity" concept of this starter: the Cache wrapper
// bigcache instances are injected as, its lifecycle (NewCache/Destroy), the
// command surface (Get/Set/Delete — the three operations that carry business
// traffic, and the only three observed), and the delegations back to the raw
// cache.
package StarterBigCache

import (
	"context"

	"github.com/allegro/bigcache/v3"
	"go-spring.org/stdlib/errutil"
)

// Cache wraps a *bigcache.BigCache so Get/Set/Delete carry their semantic
// identity and emit the operation's span and metrics (see [statObserver]). It
// holds its own observer — the instance's identity, its gauges and the one place
// its calls become signals — over the process-wide instruments. bigcache is an
// in-process heap cache with no network, so the spans are root spans (no caller
// context to link) and the durations are sub-microsecond - the value is per-key
// access visibility and a uniform signal vocabulary with the other client
// starters.
//
// No executor sits under these calls: nothing here reaches out, so there is no
// protection to apply and no access log worth writing. See [NewCache].
//
// The raw cache is an unexported field, not an embedded one: [NewCache] is the
// only way to build a Cache, so a cache can never exist without its identity, and
// the command surface plus the delegations below are the whole API. There is
// deliberately no exported accessor for the raw cache: that would let a caller
// bypass the observation layer without it showing up in review.
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

	// obs is this cache's own observability (observe.go): its instance identity,
	// its registration of the shared gauges, and the one place its calls become
	// signals. One per Cache — the instruments behind it are the process-wide set.
	obs *statObserver
}

// NewCache builds a complete Cache — identity and observation — over a raw
// (already created) bigcache instance. client is normally the Driver's product;
// instanceName is the config entry's key and the label that keeps several caches
// in one process distinguishable.
//
// Everything is applied HERE, so a Cache cannot exist half-assembled: there is no
// Init step and nothing the container has to remember to call. No container
// facility is taken either — an in-process cache has nothing to protect, and
// nothing to declare to an emitter.
//
// The error is a rejected argument (an empty instanceName) or the process-wide
// instrument set failing to build (see [newStatObserver]) — never the raw cache,
// which is already open and healthy
// when this is called. The caller owns that raw cache, so it must close it when
// this returns an error; [Driver.CreateClient] does.
func NewCache(client *bigcache.BigCache, instanceName string) (*Cache, error) {
	if err := errutil.RequireField("bigcache", "instance-name", instanceName); err != nil {
		return nil, err
	}
	obs, err := newStatObserver(client, instanceName)
	if err != nil {
		return nil, err
	}
	return &Cache{client: client, obs: obs}, nil
}

// Destroy takes away this cache's observability and closes the underlying
// BigCache. It is the gs destroy method; the unregistration comes first, because
// once the cache is closed its statistics are meaningless.
func (c *Cache) Destroy() error {
	c.obs.close()
	return c.client.Close()
}

// The command surface: Get/Set/Delete are the three operations bigcache's raw API
// exposes that carry business traffic, and the only ones worth observing. Each
// names itself and its key and runs under [statObserver.observe], which emits the
// signal. bigcache exposes no hook or plugin point, so the surface is
// hand-written - the analog of starter-memcached's command surface.
//
// The error comes back verbatim: a miss is [bigcache.ErrEntryNotFound], which
// [statusOf] folds into the ok status rather than an error, so callers keep
// treating it as the normal outcome it is.
//
// The exported forms take a context even though bigcache's raw API has none: the
// context carries the metric record — the counter the wrapper emits is labeled
// with the operation and its outcome, whatever the caller was doing.

func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	var b []byte
	err := c.obs.observe(ctx, opGet, func(context.Context) error {
		var err error
		b, err = c.client.Get(key)
		return err
	})
	return b, err
}

func (c *Cache) Set(ctx context.Context, key string, entry []byte) error {
	return c.obs.observe(ctx, opSet, func(context.Context) error {
		return c.client.Set(key, entry)
	})
}

func (c *Cache) Delete(ctx context.Context, key string) error {
	return c.obs.observe(ctx, opDelete, func(context.Context) error {
		return c.client.Delete(key)
	})
}

// The methods below delegate to the raw cache. They exist because the raw cache
// is an unexported field, so nothing is promoted: every method the raw
// *bigcache.BigCache exposed is re-exposed here unchanged, except the three the
// command surface above overwrites with instrumented operations (Get/Set/Delete).
// They are plain pass-throughs on purpose - only Get/Set/Delete are business
// traffic worth observing; these are lifecycle/introspection helpers.

// Close signals a shutdown of the cache, letting its cleaning goroutines exit.
// Note bigcache's Close is not idempotent (it closes a channel), so calling it
// and then Destroy — or twice — panics; Destroy is the normal path.
func (c *Cache) Close() error { return c.client.Close() }

// Reset empties every cache shard.
func (c *Cache) Reset() error { return c.client.Reset() }

// Len returns the number of entries in the cache.
func (c *Cache) Len() int { return c.client.Len() }

// Capacity returns the bytes allocated for the entries queues, summed over
// shards — the room the cache has, not what it is using (see [Cache.Len]).
func (c *Cache) Capacity() int { return c.client.Capacity() }

// Stats returns the cache's statistics.
func (c *Cache) Stats() bigcache.Stats { return c.client.Stats() }

// KeyMetadata returns how many times a cached resource was requested.
func (c *Cache) KeyMetadata(key string) bigcache.Metadata { return c.client.KeyMetadata(key) }

// Iterator returns an iterator over every EntryInfo in the cache.
func (c *Cache) Iterator() *bigcache.EntryInfoIterator { return c.client.Iterator() }
