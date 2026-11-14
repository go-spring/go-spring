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

// observe.go wires bigcache's calls and statistics to its [Observer] seam. The
// signals themselves — the operation counter and the cache-statistics gauges —
// live in observer.go behind the interface.
//
// bigcache emits its own signals rather than declaring an operation for the
// framework's emitter: this is an in-process cache with no external dependency,
// so neither the protection stages nor the access log an executor chain would
// produce have anything to act on. There is no span either: trace topology is
// made of edges, and a microsecond in-process call adds none.

package StarterBigCache

import (
	"context"
	"errors"

	"github.com/allegro/bigcache/v3"
)

// The operation names and the status axis the per-call metric carries.
const (
	opGet    = "get"
	opSet    = "set"
	opDelete = "delete"

	statusOK    = "ok"
	statusError = "error"
)

// statObserver is one cache's observability: its instance identity, the reader
// it registered for the shared statistics, and the one place its calls become
// signals. One per [Cache], built with it — which is what keeps the instance's
// name and its registration from being threaded through every call.
type statObserver struct {
	obs  Observer
	name string

	// off removes this cache's statistics reader. Its lifetime is the cache's —
	// [statObserver.close] takes it away — because the values the gauges report
	// come from this cache. Leaving it would report a destroyed cache and pin it.
	off func()
}

// newStatObserverWith is the injection point: obs may be nil, which disables
// observation for this cache.
func newStatObserverWith(client *bigcache.BigCache, name string, obs Observer) *statObserver {
	o := &statObserver{obs: obs, name: name}
	if obs == nil {
		return o
	}
	if off, err := obs.ObserveStats(name, func() Stats {
		s := client.Stats()
		return Stats{
			Hits:         s.Hits,
			Misses:       s.Misses,
			DeleteHits:   s.DelHits,
			DeleteMisses: s.DelMisses,
			Collisions:   s.Collisions,
			Entries:      int64(client.Len()),
			Capacity:     int64(client.Capacity()),
		}
	}); err == nil {
		o.off = off
	}
	return o
}

// close takes away this cache's statistics registration. It comes first on the
// way down: once the cache is closed its statistics are meaningless, and a
// registration left behind would both report a dead cache and pin it.
func (o *statObserver) close() {
	if o.off != nil {
		o.off()
		o.off = nil
	}
}

// statusOf maps an outcome to the metric's status axis. A miss is not a failure:
// the cache answered, the key was simply absent, and the hit rate is the gauges'
// to report — folding it in here would hide the distinction.
func statusOf(err error) string {
	if err != nil && !errors.Is(err, bigcache.ErrEntryNotFound) {
		return statusError
	}
	return statusOK
}

// observe runs one cache operation and records the outcome under the status it
// maps to. It is the one place a bigcache call becomes a signal, and the
// instance's name rides every one of them, so several caches in a process stay
// distinguishable. The key never enters the metric — cache keys are drawn from
// an open set, and one as a label would multiply the series without bound.
func (o *statObserver) observe(ctx context.Context, op string, fn func(context.Context) error) error {
	err := fn(ctx)
	if o.obs != nil {
		o.obs.Operation(ctx, o.name, op, statusOf(err))
	}
	return err
}
