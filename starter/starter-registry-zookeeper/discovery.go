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
// ${spring.registry.zookeeper} center (starter.go) under the fixed label
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

	// done stops the background watchers and is the ONLY terminal signal they
	// take: the backend closes it before the shared session goes away, so a
	// watcher retires on that rather than inferring shutdown from a connection
	// error it cannot tell apart from a reconnect.
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

// Close stops the background watchers. It is idempotent. It does NOT close the
// session: the registrar reads through the same connection, and the backend
// that created it is what releases it (zkBackend.Close).
func (d *zkDiscovery) Close() {
	d.doneOnce.Do(func() { close(d.done) })
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
			// A connection-closed error means the ensemble dropped the session,
			// NOT that this watcher is done — the client reconnects on its own
			// and shutdown is signalled by done. Treating it as terminal is what
			// let a blip landing on this very call retire the watcher for good
			// while the registrar (which polls Conn.State) recovered and
			// re-registered, leaving the instance published but undiscoverable.
			//
			// Ensemble unreachable (or the directory gone): keep the stale
			// snapshot and retry arming later. Reported so the stale window is
			// visible rather than only living inside this retry loop.
			discovery.Synced(obsSystem, name, err)
			log.Warn(context.Background(), starterTag, append(
				discovery.SyncFailedFields(obsSystem, name, err),
				log.Msgf("registry-zookeeper: arm watch %q failed (keeping stale snapshot)", path),
			)...)
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
//
// A child that vanishes mid-snapshot is one bad child, the same rule the seed
// fetch applies. Any other read failure means the snapshot is missing instances
// that are known to exist, so the previous snapshot is kept and the failure
// reported instead of publishing the short one: reporting it ok would reset the
// freshness clock and hide the loss behind a healthy-looking gauge, which is
// the one signal this side of the instrumentation has.
func (d *zkDiscovery) fetchWatches(name string, e *serviceEntry) []<-chan zk.Event {
	path := d.servicePath(name)
	children, _, err := d.conn.Children(path)
	if err != nil {
		discovery.Synced(obsSystem, name, err)
		log.Warn(context.Background(), starterTag, append(
			discovery.SyncFailedFields(obsSystem, name, err),
			log.Msgf("registry-zookeeper: list %q failed (keeping stale snapshot)", path),
		)...)
		return nil
	}
	vals := make(map[string][]byte, len(children))
	var evs []<-chan zk.Event
	var readErr error
	for _, child := range children {
		data, _, ev, err := d.conn.GetW(path + "/" + child)
		if err != nil {
			if errors.Is(err, zk.ErrNoNode) {
				// The ephemeral node's session died between Children and GetW.
				continue
			}
			readErr = err
			continue
		}
		vals[child] = data
		evs = append(evs, ev)
	}
	if readErr != nil {
		discovery.Synced(obsSystem, name, readErr)
		log.Warn(context.Background(), starterTag, append(
			discovery.SyncFailedFields(obsSystem, name, readErr),
			log.Msgf("registry-zookeeper: refresh %q failed (keeping stale snapshot)", path),
		)...)
		// The watches already armed stay live, so the next change re-runs this.
		return evs
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
// state); the payload's scheme field carries transport selection.
func valuesToEndpoints(vals map[string][]byte) []discovery.Endpoint {
	eps := make([]discovery.Endpoint, 0, len(vals))
	for name, data := range vals {
		var v instanceValue
		if err := json.Unmarshal(data, &v); err != nil {
			// A malformed payload is one bad discovery.Instance, not a broken snapshot.
			log.Warn(context.Background(), starterTag,
				log.String("system", obsSystem),
				log.String("node", name),
				log.Err(err),
				log.Msg("registry-zookeeper: skipping a malformed instance payload"))
			continue
		}
		eps = append(eps, discovery.Endpoint{
			Addr:     v.Addr,
			Scheme:   v.Scheme,
			Weight:   v.Weight,
			Healthy:  true,
			Metadata: withDimensions(v.Metadata, v.Version, v.Zone),
		})
	}
	sort.Slice(eps, func(i, j int) bool { return eps[i].Addr < eps[j].Addr })
	return eps
}

// withDimensions surfaces the payload's version/zone fields through the
// metadata map consumers route on (zone-aware balancing reads the zone key),
// copying only when something must be added.
func withDimensions(md map[string]string, version, zone string) map[string]string {
	if version == "" && zone == "" {
		return md
	}
	out := make(map[string]string, len(md)+2)
	for k, v := range md {
		out[k] = v
	}
	if version != "" {
		out[discovery.MetaKeyVersion] = version
	}
	if zone != "" {
		out[discovery.MetaKeyZone] = zone
	}
	return out
}
