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
// bigcache instances are injected as, its lifecycle (NewCache/Close), the
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
// identity and emit the operation's metrics (see [ObsCache] — the observability
// lives in the chain, not in this struct). bigcache is an in-process heap cache
// with no network, so the durations are sub-microsecond - the value is per-key
// access visibility and a uniform signal vocabulary with the other client
// starters.
//
// No executor sits under these calls: nothing here reaches out, so there is no
// protection to apply and no access log worth writing. See [NewCache].
//
// The type holds exactly two things, both exported and both deliberate:
// [NewCache] is the only way to build one, so a cache can never exist without
// its identity — the embedded [InnerCache] the command surface runs through
// (reorganized by wrapping the current head in a layer of your own; a plain
// assignment in reviewable sight) and [Cache.Client], the raw instance as a
// read-only handle — not a quiet bypass of the chain, which the delegations
// below and the observation layer already cover.
//
// The type is exported because bigcache (unlike go-redis or gorm) offers no
// hook/plugin extension point, so the only way to observe per-operation traffic
// is to hold the wrapper itself. Apps therefore inject *Cache rather than
// *bigcache.BigCache; [Driver.CreateClient] returns this type too, so a custom
// driver works with the same type the ecosystem sees.
type Cache struct {
	// The embedded InnerCache is the chain the three business operations run
	// through, exposed so a driver's post-processing (or the app, right after
	// wiring) can reorganize it: wrap the current head in its own layer, or
	// replace the chain outright. By default it is the observability layer over
	// the raw cache, so the default path is always observed; a custom layer
	// keeps that by wrapping rather than discarding the head it found.
	// Rebuilding is a build-time move — write it before the cache takes
	// traffic, not racing it.
	InnerCache

	// Client is the raw bigcache instance this cache bottoms out in — the
	// original object, not a wrapper. A handle to READ (and to hand to the
	// chain constructors when a driver builds its own assembly), never to
	// reassign or to run commands through — that would bypass the observation
	// layer.
	Client *bigcache.BigCache
}

// InnerCache is the three-operation seam the cache's internals are hollowed
// into: the business traffic bigcache carries, each call carrying its context.
// bigcache offers no hook or plugin point, so this interface is the ONLY way to
// modify what happens under [Cache.Get]/[Cache.Set]/[Cache.Delete].
//
// The default chain is the observability layer over a raw adapter, and
// the embedded InnerCache is where a custom layer goes: implement this
// interface (embed the head you found to inherit the methods you do not care
// about), then assign your layer over it. The chain under the layer keeps
// doing its job — the keys the layer rewrites are the keys the observation
// layer counts — which is usually the point of adding it.
//
// Release tears the chain down, layer by layer, and releaseRaw picks how far
// down: every layer takes away its OWN resources and passes the flag to the
// layer under it unchanged, and only the instance layer at the bottom acts on
// the flag — closing the raw cache when it is set. Release(false) is the
// shallow release — every layer's own cleanup, the instance left running;
// Release(true) is the full teardown, and [Cache.Close] is nothing but the
// head's Release(true). A custom layer therefore owns its cleanup — embedding
// the head you found inherits a Release that already covers the rest of the
// chain.
type InnerCache interface {
	// Get returns the value stored under key, or the raw cache's miss sentinel.
	Get(ctx context.Context, key string) ([]byte, error)
	// Set stores entry under key.
	Set(ctx context.Context, key string, entry []byte) error
	// Delete removes key from the cache.
	Delete(ctx context.Context, key string) error
	// Release releases the layer's own resources, then hands releaseRaw to the
	// layer under it.
	Release(releaseRaw bool) error
}

// RawCache adapts the raw bigcache instance to [InnerCache]: it is the tail of
// the default chain, discarding the context — bigcache's own API has none — and
// returning its errors verbatim. It is a plain adapter: no observability of its
// own lives here, and no resource but the instance itself. [NewRawCache] builds
// it.
type RawCache struct{ raw *bigcache.BigCache }

// NewRawCache wraps a raw bigcache instance as the [InnerCache] tail of a chain.
func NewRawCache(raw *bigcache.BigCache) *RawCache { return &RawCache{raw: raw} }

// Release closes the raw cache — and only with releaseRaw: the instance layer
// has no resource of its own, so the shallow release is a no-op and the
// instance keeps running. It is where a chain's teardown bottoms out; like
// bigcache's own close it is not idempotent (see [Cache.Close]).
func (r *RawCache) Release(releaseRaw bool) error {
	if !releaseRaw {
		return nil
	}
	return r.raw.Close()
}

func (r *RawCache) Get(_ context.Context, key string) ([]byte, error) { return r.raw.Get(key) }

func (r *RawCache) Set(_ context.Context, key string, entry []byte) error {
	return r.raw.Set(key, entry)
}

func (r *RawCache) Delete(_ context.Context, key string) error { return r.raw.Delete(key) }

// ObsCache is the observability layer of the default chain: it names each call,
// records the outcome against the process-wide counter, and reports the
// instance's statistics as gauges — all through the [statObserver] it holds,
// which is the whole of this cache's observability in one object. The layers
// below it know nothing of it. The next layer is whatever was under it when
// the chain was built; [NewObsCache] sets both.
type ObsCache struct {
	obs  *statObserver
	next InnerCache
}

// NewObsCache builds the observability layer over next, naming the instance
// instanceName — the label every signal it emits carries — and reporting the
// statistics of client, the instance layer the chain bottoms out in (gauges
// read its Len()/Stats()/Capacity(), which [InnerCache] does not carry, so it
// is passed explicitly).
//
// Building one registers, so an assembly that replaces a still-live head over
// the same instance MUST release that head first — [ObsCache.Release] with
// releaseRaw false is exactly that — or the instance is reported twice. The
// standard reorganization does not hit this: wrapping the current head adds a
// layer without building another ObsCache.
//
// The error is a rejected argument (an empty instanceName) or the process-wide
// instrument set failing to build (see [newStatObserver]) — never the layers,
// which are only held, never touched here.
func NewObsCache(next InnerCache, raw *bigcache.BigCache, instanceName string) (*ObsCache, error) {
	if err := errutil.RequireField("bigcache", "instance-name", instanceName); err != nil {
		return nil, err
	}
	obs, err := newStatObserver(raw, instanceName)
	if err != nil {
		return nil, err
	}
	return &ObsCache{obs: obs, next: next}, nil
}

// Release takes away this layer's gauge registration — its own resource,
// released at either depth, because a replacement head registers its own —
// then hands the same releaseRaw to the layer under it: every layer
// self-cleans, and only the instance layer at the bottom acts on it to close
// the instance.
func (o *ObsCache) Release(releaseRaw bool) error {
	o.obs.close()
	return o.next.Release(releaseRaw)
}

func (o *ObsCache) Get(ctx context.Context, key string) ([]byte, error) {
	var b []byte
	err := o.obs.observe(ctx, opGet, func(context.Context) error {
		var err error
		b, err = o.next.Get(ctx, key)
		return err
	})
	return b, err
}

func (o *ObsCache) Set(ctx context.Context, key string, entry []byte) error {
	return o.obs.observe(ctx, opSet, func(context.Context) error {
		return o.next.Set(ctx, key, entry)
	})
}

func (o *ObsCache) Delete(ctx context.Context, key string) error {
	return o.obs.observe(ctx, opDelete, func(context.Context) error {
		return o.next.Delete(ctx, key)
	})
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
	// The chain owns everything: the observability layer counts the calls and
	// reports the instance's statistics, the instance layer closes well, and
	// Close is nothing but the head's. The Cache itself holds no observability
	// of its own.
	tail := NewRawCache(client)
	obs, err := NewObsCache(tail, client, instanceName)
	if err != nil {
		return nil, err
	}
	return &Cache{Client: client, InnerCache: obs}, nil
}

// Close tears the cache down through its chain — the head's Release(true),
// layer by layer, observability registration first, raw cache last — because a
// close that skipped the chain would leave the gauge registration pinning a
// dead cache. It is also the gs destroy method, registered on the bean.
// bigcache's close is not idempotent (it closes a channel), so calling it
// twice panics.
func (c *Cache) Close() error { return c.InnerCache.Release(true) }

// The methods below delegate to the raw cache. They exist because the raw cache
// is an unexported field, so nothing is promoted: every method the raw
// *bigcache.BigCache exposed is re-exposed here unchanged, except the three the
// command surface above overwrites with instrumented operations (Get/Set/Delete)
// and Close, which runs the chain. They are plain pass-throughs on purpose -
// only Get/Set/Delete are business traffic worth observing; these are
// introspection helpers.

// Len returns the number of entries in the cache.
func (c *Cache) Len() int { return c.Client.Len() }

// Capacity returns the bytes allocated for the entries queues, summed over
// shards — the room the cache has, not what it is using (see [Cache.Len]).
func (c *Cache) Capacity() int { return c.Client.Capacity() }

// Stats returns the cache's statistics.
func (c *Cache) Stats() bigcache.Stats { return c.Client.Stats() }

// KeyMetadata returns how many times a cached resource was requested.
func (c *Cache) KeyMetadata(key string) bigcache.Metadata { return c.Client.KeyMetadata(key) }

// Iterator returns an iterator over every EntryInfo in the cache.
func (c *Cache) Iterator() *bigcache.EntryInfoIterator { return c.Client.Iterator() }
