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

// This file adds the CONSUMER half of ZooKeeper service discovery to the
// registry starter: a cloud/discovery Discovery backend that serves snapshots
// of the instances this starter's own registrar (registrar.go) publishes —
// closing the loop so one starter serves both sides of the naming idiom.
// Freshness is internal: each resolved service gets a background watcher that
// keeps the cached snapshot current, so Resolve is a cheap read after the
// first call.
//
// Each backend is a NAMED BEAN in the IoC container — the container is the
// discovery directory: configure one block per ensemble under
// ${spring.discovery.zookeeper.<name>} and a client starter injects the
// backend its config cites by that name.
//
//	spring.discovery.zookeeper.prod.servers=127.0.0.1:2181
//	spring.discovery.zookeeper.prod.base-path=/services
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
	"go-spring.org/spring/conf"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/errutil"
	"go-spring.org/stdlib/flatten"
)

// DiscoveryConfig binds one ZooKeeper discovery adapter under
// ${spring.discovery.zookeeper.<name>}. It mirrors the registry-side
// ZookeeperConfig connection fields (same client idiom, same auth knobs) — the
// consumer side needs no session-timeout tuning because it never writes.
type DiscoveryConfig struct {
	// Servers lists the ZooKeeper ensemble members to dial. Empty INHERITS the
	// ${spring.registry.zookeeper} center connection (one ensemble, one shared
	// session with the registrar); set it to point this backend at a different
	// ensemble than the one this process registers into.
	Servers []string `value:"${servers:=}"`

	// SessionTimeout is the ZooKeeper session timeout; it bounds the startup
	// probe of a standalone block.
	SessionTimeout time.Duration `value:"${session-timeout:=10s}"`

	// BasePath is the same persistent parent znode the registrar writes
	// under; the adapter only lists below it. It must match
	// ${spring.registry.zookeeper.base-path} of the registering applications
	// or nothing resolves.
	BasePath string `value:"${base-path:=/services}"`

	// Username / Password enable ZooKeeper digest authentication when set.
	// Leave both empty for an open ensemble.
	Username string `value:"${username:=}"`
	Password string `value:"${password:=}"`
}

func init() {
	// One NAMED bean per block under ${spring.discovery.zookeeper.<name>}: the
	// bean name is the label a client starter cites to pick this backend. The
	// backend is constructed at injection time (only when something cites it —
	// or collects all Discovery beans), and a standalone block's connection is
	// closed by the bean destructor on shutdown. A label colliding with
	// another bean name fails loudly in the container.
	gs.Module(gs.OnProperty("spring.discovery.zookeeper"), func(r gs.BeanProvider, p flatten.Storage) error {
		return conf.BindEach(p, "${spring.discovery.zookeeper}", func(name string, c DiscoveryConfig) error {
			if len(c.Servers) == 0 {
				// No connection of its own: inherit the center connection. The
				// bean keeps the block's base-path (still defaults to the
				// registrar's), so an inheriting block is usually just a label.
				r.Provide(newInheritedDiscoveryBackend,
					gs.IndexArg(0, gs.ValueArg(c)),
					gs.IndexArg(1, gs.TagArg("?")),
				).Name(name).Caller(1)
			} else {
				r.Provide(newDiscoveryBackend,
					gs.IndexArg(0, gs.ValueArg(c)),
				).Name(name).Destroy(destroyDiscoveryBackend).Caller(1)
			}
			log.Debugf(context.Background(), log.TagAppDef, "declared zookeeper discovery backend bean name=%s servers=%v", name, c.Servers)
			return nil
		})
	})
}

// newDiscoveryBackend builds one ZooKeeper-backed Discovery bean with its OWN
// connection (probing the ensemble, same fail-fast as the registrar). It runs
// at injection time; the bean destructor closes the connection.
func newDiscoveryBackend(c DiscoveryConfig) (discovery.Discovery, error) {
	conn, err := connectZookeeper(ZookeeperConfig{
		Servers:        c.Servers,
		SessionTimeout: c.SessionTimeout,
		Username:       c.Username,
		Password:       c.Password,
	})
	if err != nil {
		return nil, err
	}
	return &zkDiscovery{
		conn:     conn,
		basePath: normalizeBasePath(c.BasePath),
		done:     make(chan struct{}),
		entries:  map[string]*serviceEntry{},
	}, nil
}

// newInheritedDiscoveryBackend builds one ZooKeeper-backed Discovery bean
// reusing the shared ${spring.registry.zookeeper} center connection — the
// consumer half of the "one center config, one connection" convergence. It
// carries no destructor: the center bean owns the connection's lifetime.
func newInheritedDiscoveryBackend(c DiscoveryConfig, zc *zkCenter) (discovery.Discovery, error) {
	if zc == nil {
		return nil, errutil.Explain(nil, "registry-zookeeper: discovery block cites no servers and no ${spring.registry.zookeeper} center is configured")
	}
	return &zkDiscovery{
		conn:     zc.conn,
		basePath: normalizeBasePath(c.BasePath),
		done:     make(chan struct{}),
		entries:  map[string]*serviceEntry{},
	}, nil
}

// destroyDiscoveryBackend closes the backend's own ZooKeeper connection on
// shutdown. It is the bean destructor; the background watchers stop with the
// connection.
func destroyDiscoveryBackend(d discovery.Discovery) error {
	b, ok := d.(*zkDiscovery)
	if !ok || b == nil {
		return nil
	}
	return b.Close()
}

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
			return nil, errutil.Explain(err, "registry-zookeeper: list %q failed", name)
		}
		e.eps, e.seeded = eps, true
		go d.watchLoop(d.servicePath(name), e)
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
func (d *zkDiscovery) watchLoop(path string, e *serviceEntry) {
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
			// snapshot and retry arming later.
			select {
			case <-d.done:
				return
			case <-time.After(5 * time.Second):
			}
			continue
		}
		getEvs := d.fetchWatches(path, e)

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
func (d *zkDiscovery) fetchWatches(path string, e *serviceEntry) []<-chan zk.Event {
	children, _, err := d.conn.Children(path)
	if err != nil {
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
			log.Warnf(context.Background(), log.TagAppDef, "registry-zookeeper: skip malformed instance %q: %v", name, err)
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
