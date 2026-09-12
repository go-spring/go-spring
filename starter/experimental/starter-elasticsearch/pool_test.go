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

package StarterElasticsearch

import (
	"errors"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/elastic/elastic-transport-go/v8/elastictransport"
	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// fakeResolver is a mutable discovery.Resolver: a test sets the endpoint set it
// should report, which is what a naming service does when membership changes.
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

// seedConns builds the connections the transport hands a ConnectionPoolFunc from
// its configured addresses.
func seedConns(addrs ...string) []*elastictransport.Connection {
	conns := make([]*elastictransport.Connection, 0, len(addrs))
	for _, a := range addrs {
		u, err := url.Parse("http://" + a)
		if err != nil {
			panic(err)
		}
		conns = append(conns, &elastictransport.Connection{URL: u})
	}
	return conns
}

// poolURLs returns the pool's node set as sorted "host:port" strings, which is
// what the transport's own URLs() reports to the caller.
func poolURLs(p *livePool) []string {
	urls := p.URLs()
	hosts := make([]string, 0, len(urls))
	for _, u := range urls {
		hosts = append(hosts, u.Host)
	}
	sort.Strings(hosts)
	return hosts
}

// withoutThrottle removes the propagation budget so a test can observe a
// membership change on the very next call.
func withoutThrottle(t *testing.T) {
	t.Helper()
	old := refreshInterval
	refreshInterval = 0
	t.Cleanup(func() { refreshInterval = old })
}

// TestLivePoolFollowsMembership covers the point of the live pool: a node joining
// or leaving the cluster is visible after the propagation budget, instead of only
// after a process restart, while node selection stays the transport's own.
func TestLivePoolFollowsMembership(t *testing.T) {
	withoutThrottle(t)
	f := &fakeResolver{}
	f.set("10.0.0.1:9200", "10.0.0.2:9200")
	p := newLivePool(f.resolve, "http", nil, seedConns("10.0.0.1:9200", "10.0.0.2:9200"))
	assert.That(t, poolURLs(p)).Equal([]string{"10.0.0.1:9200", "10.0.0.2:9200"})

	f.set("10.0.0.1:9200", "10.0.0.2:9200", "10.0.0.3:9200")
	// The next request pays for the refresh: every pick funnels through Next.
	_, _ = p.Next()
	assert.That(t, poolURLs(p)).Equal([]string{"10.0.0.1:9200", "10.0.0.2:9200", "10.0.0.3:9200"})

	f.set("10.0.0.3:9200")
	_, _ = p.Next()
	assert.That(t, poolURLs(p)).Equal([]string{"10.0.0.3:9200"})
}

// TestLivePoolOrderInsensitive pins the stabilizing rule: the naming service may
// report the same membership in any order, and a reorder must not look like a
// change (it would rebuild the pool and reset the transport's per-node state).
func TestLivePoolOrderInsensitive(t *testing.T) {
	withoutThrottle(t)
	f := &fakeResolver{}
	f.set("10.0.0.1:9200", "10.0.0.2:9200")
	p := newLivePool(f.resolve, "http", nil, seedConns("10.0.0.1:9200"))
	_, _ = p.Next()

	before := p.pool
	f.set("10.0.0.2:9200", "10.0.0.1:9200") // same set, other order
	_, _ = p.Next()
	assert.That(t, p.pool == before).True()
}

// TestLivePoolKeepsLastGoodOnUnreadableSnapshot pins the failure mode: a registry
// hiccup must not black-hole a cluster that a moment ago worked.
func TestLivePoolKeepsLastGoodOnUnreadableSnapshot(t *testing.T) {
	withoutThrottle(t)
	f := &fakeResolver{}
	f.set("10.0.0.1:9200", "10.0.0.2:9200")
	p := newLivePool(f.resolve, "http", nil, seedConns("10.0.0.1:9200"))
	_, _ = p.Next()

	f.fail(errors.New("registry down"))
	_, _ = p.Next()
	assert.That(t, poolURLs(p)).Equal([]string{"10.0.0.1:9200", "10.0.0.2:9200"})

	f.set() // membership reported as empty
	_, _ = p.Next()
	assert.That(t, poolURLs(p)).Equal([]string{"10.0.0.1:9200", "10.0.0.2:9200"})
}

// TestLivePoolStillServes pins that the live pool is a working pool, not just a
// bookkeeper: Next hands out one of the discovered nodes and reports success to
// the transport's own bookkeeping.
func TestLivePoolStillServes(t *testing.T) {
	withoutThrottle(t)
	f := &fakeResolver{}
	f.set("10.0.0.1:9200", "10.0.0.2:9200")
	p := newLivePool(f.resolve, "http", nil, seedConns("10.0.0.1:9200"))

	got := map[string]bool{}
	for range 4 {
		c, err := p.Next()
		assert.Error(t, err).Nil()
		got[c.URL.Host] = true
		assert.Error(t, p.OnSuccess(c)).Nil()
	}
	assert.Number(t, len(got)).Equal(2)

	if err := p.Close(t.Context()); err != nil {
		t.Fatalf("close: %v", err)
	}
	assert.Error(t, p.Close(t.Context())).Nil() // idempotent
	_, err := p.Next()
	assert.Error(t, err).NotNil() // a closed pool serves nothing
}

// TestLivePoolHonoursPropagationBudget pins the throttle: the snapshot is not
// re-read on every request, so a hot path pays one time check and nothing more.
func TestLivePoolHonoursPropagationBudget(t *testing.T) {
	old := refreshInterval
	refreshInterval = time.Hour
	t.Cleanup(func() { refreshInterval = old })

	f := &fakeResolver{}
	f.set("10.0.0.1:9200")
	p := newLivePool(f.resolve, "http", nil, seedConns("10.0.0.1:9200"))
	_, _ = p.Next()

	f.set("10.0.0.1:9200", "10.0.0.2:9200")
	for range 5 {
		_, _ = p.Next()
	}
	assert.That(t, poolURLs(p)).Equal([]string{"10.0.0.1:9200"})
}
