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

package StarterConfigConsul

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"go-spring.org/stdlib/testing/assert"
)

func TestParseSource(t *testing.T) {
	// Full form: every query knob spelled out.
	cs, err := parseSource("127.0.0.1:8500/config/app.yaml?format=yaml&token=tk&datacenter=dc1&scheme=https")
	assert.That(t, err).Nil()
	assert.That(t, cs.address).Equal("127.0.0.1:8500")
	assert.That(t, cs.scheme).Equal("https")
	assert.That(t, cs.kvPath).Equal("config/app.yaml")
	assert.That(t, cs.token).Equal("tk")
	assert.That(t, cs.datacenter).Equal("dc1")
	assert.That(t, cs.format).Equal("yaml")

	// Defaults: http scheme, format inferred from the key extension.
	cs, err = parseSource("127.0.0.1:8500/config/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, cs.scheme).Equal("http")
	assert.That(t, cs.format).Equal("properties")

	// No extension falls back to properties.
	cs, err = parseSource("127.0.0.1:8500/config/app")
	assert.That(t, err).Nil()
	assert.That(t, cs.format).Equal("properties")

	// Missing server or kv path fails loudly.
	_, err = parseSource("onlypath")
	assert.That(t, err).NotNil()
	_, err = parseSource("127.0.0.1:8500")
	assert.That(t, err).NotNil()
}

// fakeKV fakes the Consul KV surface. Scripted results are served in order;
// when the script runs dry the last result repeats forever, standing in for a
// blocking query that keeps reporting the same index.
type fakeKV struct {
	mu     sync.Mutex
	served int // scripted results consumed so far
	script []fakeResult
}

type fakeResult struct {
	pair *api.KVPair
	idx  uint64
	err  error
}

func newFakeKV() *fakeKV {
	return &fakeKV{}
}

func (f *fakeKV) result(pair *api.KVPair, idx uint64, err error) *fakeKV {
	f.script = append(f.script, fakeResult{pair, idx, err})
	return f
}

func (f *fakeKV) Get(string, *api.QueryOptions) (*api.KVPair, *api.QueryMeta, error) {
	f.mu.Lock()
	if len(f.script) == 0 {
		f.mu.Unlock()
		return nil, nil, errors.New("empty script")
	}
	i := f.served
	repeat := false
	if i >= len(f.script) {
		i = len(f.script) - 1
		repeat = true
	} else {
		f.served++
	}
	r := f.script[i]
	f.mu.Unlock()
	if repeat {
		// Stand in for the blocking-query wait so the watcher goroutines left
		// behind by Load tests do not spin hot on the unchanged index.
		time.Sleep(10 * time.Millisecond)
	}
	return r.pair, &api.QueryMeta{LastIndex: r.idx}, r.err
}

func (f *fakeKV) done() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.served
}

// newCtrlWithFake pre-seeds the controller's client cache for source so
// clientFor returns the fake without any network.
func newCtrlWithFake(source string, fake kvAPI) (*consulCtrl, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}
	c := newConsulCtrl()
	c.clients[clientKey(cs)] = fake
	return c, nil
}

func TestLoadProperties(t *testing.T) {
	fake := newFakeKV().result(&api.KVPair{Value: []byte("greeting=hello\nnum=42\n")}, 1, nil)
	c, err := newCtrlWithFake("127.0.0.1:8500/app.properties", fake)
	assert.That(t, err).Nil()
	m, err := c.Load(false, "127.0.0.1:8500/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, m["greeting"]).Equal("hello")
	assert.That(t, m["num"]).Equal("42")
}

func TestLoadOptionalSkipsOnFetchFailureMissingAndEmpty(t *testing.T) {
	// optional:true turns a fetch error into a skip, not a startup failure.
	fakeErr := newFakeKV().result(nil, 0, errors.New("down"))
	c, err := newCtrlWithFake("127.0.0.1:8500/app.properties", fakeErr)
	assert.That(t, err).Nil()
	m, err := c.Load(true, "127.0.0.1:8500/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, len(m)).Equal(0)

	// Same for a missing key (nil pair) and an empty-but-present value.
	fakeMissing := newFakeKV().result(nil, 0, nil)
	c2, err := newCtrlWithFake("127.0.0.1:8500/app.properties", fakeMissing)
	assert.That(t, err).Nil()
	m2, err := c2.Load(true, "127.0.0.1:8500/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, len(m2)).Equal(0)

	fakeEmpty := newFakeKV().result(&api.KVPair{}, 0, nil)
	c3, err := newCtrlWithFake("127.0.0.1:8500/app.properties", fakeEmpty)
	assert.That(t, err).Nil()
	m3, err := c3.Load(true, "127.0.0.1:8500/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, len(m3)).Equal(0)

	// Non-optional propagates all three.
	_, err = c.Load(false, "127.0.0.1:8500/app.properties")
	assert.That(t, err).NotNil()
	_, err = c2.Load(false, "127.0.0.1:8500/app.properties")
	assert.That(t, err).NotNil()
	_, err = c3.Load(false, "127.0.0.1:8500/app.properties")
	assert.That(t, err).NotNil()
}

func TestLoadParseErrorPropagates(t *testing.T) {
	// Content that does not parse as the declared format must fail the load.
	fake := newFakeKV().result(&api.KVPair{Value: []byte("{not-json")}, 1, nil)
	c, err := newCtrlWithFake("127.0.0.1:8500/app.json", fake)
	assert.That(t, err).Nil()
	_, err = c.Load(false, "127.0.0.1:8500/app.json")
	assert.That(t, err).NotNil()
}

func TestWatchRegisteredOncePerSource(t *testing.T) {
	// Two Loads of the same source (refresh re-loads every import): the watch
	// goroutine is registered exactly once — the dedup key is client+kvPath
	// and a second blocking-query loop would double-fire refreshes.
	fake := newFakeKV().result(&api.KVPair{Value: []byte("a=1\n")}, 1, nil)
	c, err := newCtrlWithFake("127.0.0.1:8500/app.properties", fake)
	assert.That(t, err).Nil()
	for i := 0; i < 2; i++ {
		_, err = c.Load(false, "127.0.0.1:8500/app.properties")
		assert.That(t, err).Nil()
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	assert.That(t, len(c.listened)).Equal(1)
}

func TestWatchLoopAdvancesIndex(t *testing.T) {
	// The blocking-query loop must swallow the initial index, then observe the
	// index bump. Script: initial (idx 5), unchanged (idx 5), changed (idx 9);
	// afterwards the fake repeats the final index, standing in for a blocking
	// query that keeps reporting the same (unchanged) index. The loop keeps
	// running by design, so the test just observes it consuming the script.
	fake := newFakeKV().
		result(&api.KVPair{Value: []byte("a=1\n")}, 5, nil).
		result(&api.KVPair{Value: []byte("a=1\n")}, 5, nil).
		result(&api.KVPair{Value: []byte("a=2\n")}, 9, nil)

	c := newConsulCtrl()
	go c.watchLoop(context.Background(), fake, configSource{kvPath: "app.properties"}, false)

	deadline := time.Now().Add(5 * time.Second)
	for fake.done() < 3 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.That(t, fake.done()).Equal(3)
}

func TestTriggerRefreshNilRefresherIsNoop(t *testing.T) {
	// Before the app has started, the gs.RefreshProperties facade returns an
	// error and the change is dropped — a Consul index bump must be harmless.
	c := newConsulCtrl()
	c.TriggerRefresh(context.Background())
}

func TestCloseStopsWatchersAndRearms(t *testing.T) {
	// Close cancels the watch generation and clears the dedup set; the next
	// Load re-arms (fresh generation) and registers the watch again.
	fake := newFakeKV().result(&api.KVPair{Value: []byte("a=1\n")}, 1, nil)
	c, err := newCtrlWithFake("127.0.0.1:8500/app.properties", fake)
	assert.That(t, err).Nil()
	_, err = c.Load(false, "127.0.0.1:8500/app.properties")
	assert.That(t, err).Nil()

	ctx := c.currentCtx()

	assert.That(t, c.Close(context.Background())).Nil()
	assert.That(t, ctx.Err()).NotNil() // generation canceled: loops see it and exit

	c.mu.Lock()
	assert.That(t, len(c.listened)).Equal(0)
	c.mu.Unlock()

	// A Load after Close re-arms and re-watches.
	_, err = c.Load(false, "127.0.0.1:8500/app.properties")
	assert.That(t, err).Nil()
	c.mu.Lock()
	defer c.mu.Unlock()
	assert.That(t, len(c.listened)).Equal(1)
	assert.That(t, c.ctx.Err()).Nil()
}

func TestWatchLoopExitsOnCancelAndRetriesThroughFailures(t *testing.T) {
	// Script: failure, failure, recovery with a changed index, then a failure
	// that repeats — the loop must retry through all of them (not exit), then
	// return once the generation context is canceled.
	fake := newFakeKV().
		result(nil, 0, errors.New("consul down")).
		result(nil, 0, errors.New("consul down")).
		result(&api.KVPair{Value: []byte("a=2\n")}, 7, nil).
		result(nil, 0, errors.New("consul down"))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	c := newConsulCtrl()
	go func() { c.watchLoop(ctx, fake, configSource{kvPath: "app.properties"}, false); close(done) }()

	deadline := time.Now().Add(5 * time.Second)
	for fake.done() < 4 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	assert.That(t, fake.done()).Equal(4) // retried through every failure

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("watchLoop did not exit after cancel")
	}
}

func TestClientForCachesPerConnectionTuple(t *testing.T) {
	// Same source twice → one cached handle; a different datacenter is a
	// different cache key.
	fake := newFakeKV().result(&api.KVPair{Value: []byte("a=1\n")}, 1, nil)
	c, err := newCtrlWithFake("127.0.0.1:8500/app.properties?datacenter=dc1", fake)
	assert.That(t, err).Nil()

	cs, err := parseSource("127.0.0.1:8500/app.properties?datacenter=dc1")
	assert.That(t, err).Nil()
	got, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()
	assert.That(t, got).Equal(fake)

	again, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()
	assert.That(t, again).Equal(fake)

	c.mu.Lock()
	defer c.mu.Unlock()
	assert.That(t, len(c.clients)).Equal(1)
}

// currentCtx exposes the watch generation for lifecycle tests.
func (c *consulCtrl) currentCtx() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ctx
}

// Compile-time guard: the real Consul KV handle satisfies the fake-able
// interface the controller depends on.
var _ kvAPI = (*api.KV)(nil)
