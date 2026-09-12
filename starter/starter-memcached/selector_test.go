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

package StarterMemcached

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"testing"

	"github.com/bradfitz/gomemcache/memcache"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// fakeResolver is a mutable discovery.Resolver: a test sets the endpoint set it
// should report, which is exactly what a naming service does when membership
// changes.
type fakeResolver struct {
	mu  sync.Mutex
	eps []discovery.Endpoint
	err error
}

func (f *fakeResolver) set(addrs ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = nil
	f.eps = f.eps[:0]
	for _, a := range addrs {
		f.eps = append(f.eps, discovery.Endpoint{Addr: a, Healthy: true, Weight: 1})
	}
}

func (f *fakeResolver) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

func (f *fakeResolver) resolve() ([]discovery.Endpoint, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return append([]discovery.Endpoint(nil), f.eps...), nil
}

// TestLiveServersMatchesServerListHashing is the key-affinity guarantee: for the
// same server set, the live selector must hash a key onto exactly the server
// gomemcache's own ServerList would — otherwise switching a cluster to discovery
// would move keys between instances and cold-start the cache.
func TestLiveServersMatchesServerListHashing(t *testing.T) {
	addrs := []string{"10.0.0.1:11211", "10.0.0.2:11211", "10.0.0.3:11211"}
	f := &fakeResolver{}
	f.set(addrs...)
	live := newLiveServers(f.resolve)

	want := new(memcache.ServerList)
	assert.Error(t, want.SetServers(addrs...)).Nil()

	for i := range 300 {
		key := fmt.Sprintf("user:%d", i)
		got, err := live.PickServer(key)
		assert.Error(t, err).Nil()
		exp, err := want.PickServer(key)
		assert.Error(t, err).Nil()
		assert.That(t, got.String()).Equal(exp.String())
	}
}

// TestLiveServersFollowsMembership covers the point of the live selector: an
// instance joining or leaving the naming service is visible on the next lookup,
// with no restart — the one-shot behavior it replaces required a restart.
func TestLiveServersFollowsMembership(t *testing.T) {
	f := &fakeResolver{}
	f.set("10.0.0.1:11211", "10.0.0.2:11211")
	live := newLiveServers(f.resolve)

	servers := func(n int) map[string]bool {
		seen := map[string]bool{}
		for i := range n {
			a, err := live.PickServer(fmt.Sprintf("user:%d", i))
			assert.Error(t, err).Nil()
			seen[a.String()] = true
		}
		return seen
	}

	before := servers(200)
	assert.Number(t, len(before)).Equal(2)
	assert.That(t, before["10.0.0.3:11211"]).False()

	// Scale up: the new instance takes its share immediately.
	f.set("10.0.0.1:11211", "10.0.0.2:11211", "10.0.0.3:11211")
	after := servers(400)
	assert.Number(t, len(after)).Equal(3)

	// Scale down: the removed one takes no traffic at all.
	f.set("10.0.0.1:11211", "10.0.0.3:11211")
	drained := servers(400)
	assert.Number(t, len(drained)).Equal(2)
	assert.That(t, drained["10.0.0.2:11211"]).False()
}

// TestLiveServersOrderInsensitive pins the stabilizing rule: the naming service
// may report the same membership in any order (and backends routinely do), so the
// selector hashes over an address-sorted snapshot — a reorder must not shuffle
// keys between instances.
func TestLiveServersOrderInsensitive(t *testing.T) {
	f := &fakeResolver{}
	f.set("10.0.0.1:11211", "10.0.0.2:11211", "10.0.0.3:11211")
	live := newLiveServers(f.resolve)

	pick := func() map[string]string {
		m := map[string]string{}
		for i := range 100 {
			key := fmt.Sprintf("user:%d", i)
			a, err := live.PickServer(key)
			assert.Error(t, err).Nil()
			m[key] = a.String()
		}
		return m
	}

	first := pick()
	f.set("10.0.0.3:11211", "10.0.0.1:11211", "10.0.0.2:11211") // same set, new order
	assert.That(t, pick()).Equal(first)
}

// TestLiveServersSurfaceUnusableCluster pins the failure mode: an unknown cluster
// is reported, not papered over with a stale set the key may no longer live on.
func TestLiveServersSurfaceUnusableCluster(t *testing.T) {
	f := &fakeResolver{}
	f.set("10.0.0.1:11211")
	live := newLiveServers(f.resolve)
	_, err := live.PickServer("k")
	assert.Error(t, err).Nil()
	// Single server: no hashing, and the resolved address is cached.
	a, err := live.PickServer("anything")
	assert.Error(t, err).Nil()
	assert.That(t, a.String()).Equal("10.0.0.1:11211")

	f.set() // membership went to zero
	_, err = live.PickServer("k")
	assert.That(t, errors.Is(err, memcache.ErrNoServers)).True()

	boom := errors.New("registry unreachable")
	f.fail(boom)
	_, err = live.PickServer("k")
	assert.That(t, errors.Is(err, boom)).True()
}

// TestLiveServersEach covers the cluster-wide operations (FlushAll/Stats): they
// walk the current snapshot rather than a boot-time one.
func TestLiveServersEach(t *testing.T) {
	f := &fakeResolver{}
	f.set("10.0.0.2:11211", "10.0.0.1:11211")
	live := newLiveServers(f.resolve)

	got := map[string]bool{}
	err := live.Each(func(a net.Addr) error {
		got[a.String()] = true
		return nil
	})
	assert.Error(t, err).Nil()
	assert.Number(t, len(got)).Equal(2)
	assert.That(t, got["10.0.0.1:11211"]).True()

	f.set("10.0.0.9:11211")
	got = map[string]bool{}
	assert.Error(t, live.Each(func(a net.Addr) error {
		got[a.String()] = true
		return nil
	})).Nil()
	assert.That(t, got["10.0.0.9:11211"]).True()

	// A visitor error stops the walk, like the built-in ServerList.
	f.set("10.0.0.1:11211", "10.0.0.2:11211")
	stop := errors.New("stop")
	assert.That(t, errors.Is(live.Each(func(net.Addr) error { return stop }), stop)).True()

	// An unreadable snapshot is surfaced, not silently treated as "no servers".
	f.fail(errors.New("down"))
	assert.Error(t, live.Each(func(net.Addr) error { return nil })).NotNil()
}
