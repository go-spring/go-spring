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

// This file is the CONSUMER half of ZooKeeper service discovery: the
// cloud/discovery Discovery backend serving snapshots of the instances the
// registrar (registrar.go) publishes. It is derived from the
// ${spring.registry.zookeeper} center (center.go) under the fixed label
// "zookeeper" — there is no separate discovery config block. Freshness is
// internal: each resolved service gets a background watcher that keeps the
// cached snapshot current, so Resolve is a cheap read after the first call.
//
// Health is derived from znode liveness: an instance is an ephemeral znode
// owned by the registrar's session (process death lets ZooKeeper remove it),
// so every node found under the service path is a live instance.

package StarterRegistryZookeeper

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-zookeeper/zk"
	"go-spring.org/cloud/discovery"
	"go-spring.org/log"
	"go-spring.org/stdlib/errutil"
)

// normalizeBasePath trims trailing slashes so the service path has exactly one
// separator per level — the same normalization zkRegistrar applies at
// construction.
func normalizeBasePath(p string) string {
	return strings.TrimRight(p, "/")
}

// zkDiscovery serves snapshots of service instances stored under one
// ZooKeeper base path. It implements [discovery.Discovery]. The first Resolve
// of a service seeds a per-service cache and starts a background watcher that
// keeps it current; later calls are in-memory reads.
type zkDiscovery struct {
	conn     *zk.Conn
	basePath string

	// done stops the background watchers; closed by Close (standalone
	// backends). Inherited backends have no destructor, so their watchers exit
	// when the center closes the shared connection instead.
	done     chan struct{}
	doneOnce sync.Once

	mu      sync.Mutex // guards entries
	entries map[string]*serviceEntry
}

// serviceEntry is the cached snapshot for one service name. eps holds the FULL
// (unfiltered) set; scheme narrowing happens per Resolve call.
type serviceEntry struct {
	mu     sync.Mutex // guards eps; held across the seed fetch so it runs once
	eps    []discovery.Endpoint
	seeded bool
}

// Close stops the background watchers and closes the backend's own
// connection. It is idempotent.
func (d *zkDiscovery) Close() error {
	d.doneOnce.Do(func() { close(d.done) })
	if d.conn != nil {
		d.conn.Close()
	}
	return nil
}

// servicePath returns the znode path holding name's instances:
// basePath + "/" + name — the layout zkRegistrar.pathFor writes beneath.
func (d *zkDiscovery) servicePath(name string) string {
	return d.basePath + "/" + name
}

// entry returns (creating if needed) the cache entry for name.
func (d *zkDiscovery) entry(name string) *serviceEntry {
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.entries[name]
	if !ok {
		e = &serviceEntry{}
		d.entries[name] = e
	}
	return e
}

// Resolve returns the current instance set for name. An ephemeral znode's
// existence is the health signal: ZooKeeper deletes it when the registrar's
// session dies, so everything found is live. The first call pays the seed
// fetch (bounded by ctx) and starts the background watcher; later calls read
// the cache.
func (d *zkDiscovery) Resolve(ctx context.Context, name string, opts ...discovery.Option) ([]discovery.Endpoint, error) {
	e := d.entry(name)
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.seeded {
		eps, err := d.fetch(ctx, d.servicePath(name))
		if err != nil {
			discovery.Synced(obsSystem, name, err)
			return nil, errutil.Explain(err, "registry-zookeeper: list %q failed", name)
		}
		e.eps, e.seeded = eps, true
		// The seed is this service's first confirmed snapshot; reporting it
		// starts the freshness clock before any watch event arrives.
		discovery.Synced(obsSystem, name, nil)
		go d.watchLoop(name, e)
	}
	eps := discovery.FilterByScheme(append([]discovery.Endpoint(nil), e.eps...), discovery.NewQuery("", opts...).Scheme)
	return eps, nil
}

// fetch reads the current instance set for path: the ephemeral children of the
// service directory, each carrying the instanceValue JSON the registrar
// wrote. A missing directory (no instance registered yet) is an empty
// snapshot, not an error.
func (d *zkDiscovery) fetch(ctx context.Context, path string) ([]discovery.Endpoint, error) {
	_ = ctx // per-call ops below are unbounded reads against a live session
	children, _, err := d.conn.Children(path)
	if err != nil {
		if errors.Is(err, zk.ErrNoNode) {
			return []discovery.Endpoint{}, nil
		}
		return nil, err
	}
	vals := make(map[string][]byte, len(children))
	for _, child := range children {
		data, _, err := d.conn.Get(path + "/" + child)
		if err != nil {
			// A vanishing ephemeral node (its session just died) is one bad
			// child, not a broken snapshot.
			if errors.Is(err, zk.ErrNoNode) {
				continue
			}
			return nil, err
		}
		vals[child] = data
	}
	return valuesToEndpoints(vals), nil
}

// watchLoop refreshes a service's cache on every ZooKeeper change below path
// until the backend closes. ChildrenW covers join/leave (including the
// ephemeral deletion on session expiry); a GetW armed on every child covers
// in-place payload rewrites (weight updates). A failed refresh keeps the stale
// snapshot — stale addresses are safer than none — and is retried with
// backoff.
func (d *zkDiscovery) watchLoop(name string, e *serviceEntry) {
	path := d.servicePath(name)
	for {
		// Arm the children watch first, then snapshot with GetW on every
		// child: events arriving between the two are re-read by the snapshot,
		// events after it fire a watch — no gap either way.
		_, _, childEv, err := d.conn.ChildrenW(path)
		if err != nil {
			if errConnectionClosed(err) {
				return
			}
			// Ensemble unreachable (or the directory gone): keep the stale
			// snapshot and retry arming later. Reported so the stale window is
			// visible rather than only living inside this retry loop.
			discovery.Synced(obsSystem, name, err)
			select {
			case <-d.done:
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		getEvs := d.fetchWatches(name, e)

		select {
		case <-d.done:
			return
		case <-childEv:
		case <-firstEvent(getEvs):
		}
	}
}

// fetchWatches snapshots path's instances into e (armed with GetW so in-place
// payload rewrites fire too) and returns the armed watch channels.
func (d *zkDiscovery) fetchWatches(name string, e *serviceEntry) []<-chan zk.Event {
	path := d.servicePath(name)
	children, _, err := d.conn.Children(path)
	if err != nil {
		discovery.Synced(obsSystem, name, err)
		return nil
	}
	vals := make(map[string][]byte, len(children))
	var evs []<-chan zk.Event
	for _, child := range children {
		data, _, ev, err := d.conn.GetW(path + "/" + child)
		if err != nil {
			continue
		}
		vals[child] = data
		evs = append(evs, ev)
	}
	eps := valuesToEndpoints(vals)
	e.mu.Lock()
	e.eps = eps
	e.mu.Unlock()
	discovery.Synced(obsSystem, name, nil)
	return evs
}

// firstEvent folds n armed watch channels into one that yields whichever
// fires first. The send is non-blocking into a capacity-1 buffer, so exactly
// one delivery wins and the losing goroutines exit as soon as their own watch
// fires (or the channel closes) instead of blocking forever. A nil return
// blocks in select — the correct "nothing armed" case when no child exists.
func firstEvent(evs []<-chan zk.Event) <-chan zk.Event {
	if len(evs) == 0 {
		return nil
	}
	out := make(chan zk.Event, 1)
	for _, ev := range evs {
		go func(src <-chan zk.Event) {
			if e, ok := <-src; ok {
				select {
				case out <- e:
				default:
				}
			}
		}(ev)
	}
	return out
}

// valuesToEndpoints maps one snapshot's znode payloads to discovery endpoints,
// decoding the instanceValue JSON the registrar writes. Every node found is
// healthy (ephemeral liveness) and enabled (the registrar has no disabled
// state); an optional "scheme" metadata key carries transport selection,
// mirroring the etcd adapter.
func valuesToEndpoints(vals map[string][]byte) []discovery.Endpoint {
	eps := make([]discovery.Endpoint, 0, len(vals))
	for name, data := range vals {
		var v instanceValue
		if err := json.Unmarshal(data, &v); err != nil {
			// A malformed payload is one bad instance, not a broken snapshot.
			log.Warnf(context.Background(), starterTag, "registry-zookeeper: skip malformed instance %q: %v", name, err)
			continue
		}
		eps = append(eps, discovery.Endpoint{
			Addr:     v.Addr,
			Scheme:   v.Metadata["scheme"],
			Weight:   v.Weight,
			Healthy:  true,
			Metadata: v.Metadata,
		})
	}
	sort.Slice(eps, func(i, j int) bool { return eps[i].Addr < eps[j].Addr })
	return eps
}
