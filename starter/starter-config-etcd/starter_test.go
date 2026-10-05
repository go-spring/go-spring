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

package StarterConfigEtcd

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
	pb "go.etcd.io/etcd/api/v3/etcdserverpb"
	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func TestParseSource(t *testing.T) {
	// Full form: every query knob spelled out.
	cs, err := parseSource("127.0.0.1:2379/config/app.yaml?format=yaml&username=u&password=p&dial-timeout=2s")
	assert.That(t, err).Nil()
	assert.That(t, cs.endpoint).Equal("127.0.0.1:2379")
	assert.That(t, cs.key).Equal("config/app.yaml")
	assert.That(t, cs.username).Equal("u")
	assert.That(t, cs.password).Equal("p")
	assert.That(t, cs.format).Equal("yaml")
	assert.That(t, cs.dialTimeout).Equal(2 * time.Second)

	// Defaults: format inferred from the key extension, 5s dial timeout.
	cs, err = parseSource("127.0.0.1:2379/config/app.json")
	assert.That(t, err).Nil()
	assert.That(t, cs.format).Equal("json")
	assert.That(t, cs.dialTimeout).Equal(5 * time.Second)

	// No extension falls back to properties.
	cs, err = parseSource("127.0.0.1:2379/config/app")
	assert.That(t, err).Nil()
	assert.That(t, cs.format).Equal("properties")

	// Missing server or key fails loudly.
	_, err = parseSource("onlykey")
	assert.That(t, err).NotNil()
	_, err = parseSource("127.0.0.1:2379")
	assert.That(t, err).NotNil()

	// An invalid dial-timeout is rejected instead of silently coerced.
	_, err = parseSource("127.0.0.1:2379/config/app?dial-timeout=not-a-duration")
	assert.That(t, err).NotNil()
}

func TestClientKey(t *testing.T) {
	// The cache key separates clients only by credentials-bearing fields:
	// two sources differing in key or format share one client.
	cs := configSource{endpoint: "h:2379", username: "u", password: "p", key: "a", format: "yaml"}
	same, _ := parseSource("h:2379/a?format=yaml&username=u&password=p")
	assert.That(t, clientKey(cs)).Equal(clientKey(same))
	same.key, same.format = "b", "properties"
	assert.That(t, clientKey(cs)).Equal(clientKey(same))
	other, _ := parseSource("h:2379/a?username=u2")
	assert.That(t, clientKey(cs)).NotEqual(clientKey(other))
}

func TestLifecycleRearmAndClose(t *testing.T) {
	c := newEtcdCtrl()
	ctx1 := c.watch.ctx

	// Close cancels the outgoing generation, mints the next one right away,
	// and empties the caches.
	c.Close()
	assert.That(t, ctx1.Err()).NotNil()
	assert.That(t, c.watch.ctx.Err()).Nil()
	assert.That(t, len(c.clients)).Equal(0)
	assert.That(t, len(c.watch.listened)).Equal(0)

	// Close twice is safe: the second cancel targets the live generation.
	c.Close()

	// A registration after Close runs on the minted generation. clientv3.New
	// is lazy and dials with retry backoff, so registering against an
	// unreachable endpoint spawns a watcher that harmlessly keeps retrying.
	cs := configSource{endpoint: "127.0.0.1:1", key: "k", dialTimeout: time.Second}
	cli, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()
	c.watch.registerWatcher(cli, cs, false, 0)
	assert.That(t, c.watch.ctx == ctx1).Equal(false)
	assert.That(t, c.watch.ctx.Err()).Nil()
	assert.That(t, len(c.watch.listened)).Equal(1)

	// A second registration on the live generation is deduplicated and keeps it.
	c.watch.registerWatcher(cli, cs, false, 0)
	assert.That(t, len(c.watch.listened)).Equal(1)
	assert.That(t, c.watch.ctx.Err()).Nil()
}

func TestClientForCacheAndClose(t *testing.T) {
	// clientv3.New is lazy: it succeeds instantly against an unreachable
	// endpoint, so no live etcd is needed here.
	c := newEtcdCtrl()
	cs := configSource{endpoint: "127.0.0.1:1", key: "k", dialTimeout: time.Second}

	cli1, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()
	cli2, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()
	assert.That(t, cli2 == cli1).Equal(true)
	assert.That(t, len(c.clients)).Equal(1)

	// Close closes the cached client and drops the cache.
	c.Close()
	assert.That(t, len(c.clients)).Equal(0)

	// The next clientFor builds a fresh client.
	cli3, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()
	assert.That(t, cli3 == cli1).Equal(false)
	c.Close()
}

func TestLoadRejectsBadSources(t *testing.T) {
	c := newEtcdCtrl()
	defer c.Close()

	// Malformed URL, missing host, missing key, bad dial-timeout: each fails
	// before any client is built, regardless of optional.
	for _, src := range []string{"%zz", "127.0.0.1:2379", "127.0.0.1:2379/k?dial-timeout=bogus"} {
		_, err := c.Load(context.Background(), false, src)
		assert.That(t, err).NotNil()
		_, err = c.Load(context.Background(), true, src)
		assert.That(t, err).NotNil()
	}
	assert.That(t, len(c.clients)).Equal(0)
}

func TestLoadGetFailure(t *testing.T) {
	c := newEtcdCtrl()
	defer c.Close()

	// Nothing listens on 127.0.0.1:1: the client is built lazily, but the Get
	// fails. Required -> error; optional -> skipped without error.
	_, err := c.Load(context.Background(), false, "127.0.0.1:1/k?dial-timeout=100ms")
	assert.That(t, err).NotNil()

	m, err := c.Load(context.Background(), true, "127.0.0.1:1/k2?dial-timeout=100ms")
	assert.That(t, err).Nil()
	assert.That(t, m).Nil()
}

func TestRegisterWatcherDedup(t *testing.T) {
	c := newEtcdCtrl()
	defer c.Close()

	cs, err := parseSource("127.0.0.1:1/k?dial-timeout=100ms")
	assert.That(t, err).Nil()
	cli, err := c.clientFor(context.Background(), cs)
	assert.That(t, err).Nil()

	// Repeated Load calls install exactly one watcher per client+key.
	c.watch.registerWatcher(cli, cs, false, 0)
	c.watch.registerWatcher(cli, cs, false, 0)
	assert.That(t, len(c.watch.listened)).Equal(1)

	// A different key gets its own watcher entry.
	cs2 := cs
	cs2.key = "k2"
	c.watch.registerWatcher(cli, cs2, true, 0)
	assert.That(t, len(c.watch.listened)).Equal(2)
}

// fakeEtcd fakes the etcd client surface (see etcdAPI) so the load and watch
// success paths can be driven without a live server.
type fakeEtcd struct {
	mu      sync.Mutex
	gets    []fakeGet
	served  int
	watches int
	events  []clientv3.WatchResponse
	sent    int
}

type fakeGet struct {
	resp *clientv3.GetResponse
	err  error
}

func newFakeEtcd() *fakeEtcd { return &fakeEtcd{} }

// get scripts one Get result; scripted results are served in order and the
// last one repeats, standing in for a re-read.
func (f *fakeEtcd) get(resp *clientv3.GetResponse, err error) *fakeEtcd {
	f.gets = append(f.gets, fakeGet{resp, err})
	return f
}

// watch scripts the responses the next Watch subscription delivers.
func (f *fakeEtcd) watch(evs ...clientv3.WatchResponse) *fakeEtcd {
	f.events = append(f.events, evs...)
	return f
}

func (f *fakeEtcd) Get(context.Context, string, ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.gets) == 0 {
		return nil, errors.New("empty get script")
	}
	i := f.served
	if i >= len(f.gets) {
		i = len(f.gets) - 1
	} else {
		f.served++
	}
	return f.gets[i].resp, f.gets[i].err
}

func (f *fakeEtcd) Watch(ctx context.Context, _ string, _ ...clientv3.OpOption) clientv3.WatchChan {
	f.mu.Lock()
	f.watches++
	evs := f.events
	f.events = nil
	f.mu.Unlock()

	ch := make(chan clientv3.WatchResponse, len(evs)+1)
	for _, ev := range evs {
		ch <- ev
		f.mu.Lock()
		f.sent++
		f.mu.Unlock()
	}
	// Stay open like a healthy idle watch, and close when the watch generation
	// is cancelled so the loop exits instead of parking forever.
	go func() { <-ctx.Done(); close(ch) }()
	return ch
}

func (f *fakeEtcd) Close() error { return nil }

func (f *fakeEtcd) watchCalls() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watches
}

func (f *fakeEtcd) delivered() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.sent
}

// newCtrlWithFake pre-seeds the controller's client cache for source so
// clientFor returns the fake without any network.
func newCtrlWithFake(source string, fake etcdAPI) (*etcdCtrl, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}
	c := newEtcdCtrl()
	c.clients[clientKey(cs)] = fake
	return c, nil
}

func kvResponse(rev int64, value string) *clientv3.GetResponse {
	return &clientv3.GetResponse{
		Header: &pb.ResponseHeader{Revision: rev},
		Kvs:    []*mvccpb.KeyValue{{Key: []byte("app"), Value: []byte(value), ModRevision: rev}},
	}
}

func TestLoadPropertiesFromClient(t *testing.T) {
	// The Get -> parse path, driven through the fake client (no live etcd).
	fake := newFakeEtcd().get(kvResponse(7, "greeting=hello\nnum=42\n"), nil)
	c, err := newCtrlWithFake("127.0.0.1:2379/app.properties", fake)
	assert.That(t, err).Nil()
	defer c.Close()

	m, err := c.Load(context.Background(), false, "127.0.0.1:2379/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, m["greeting"]).Equal("hello")
	assert.That(t, m["num"]).Equal("42")
}

func TestLoadOptionalSkipsOnGetFailureAndEmptyKey(t *testing.T) {
	// optional:true turns a fetch failure and an empty key into a skip.
	fail := newFakeEtcd().get(nil, errors.New("down"))
	c, err := newCtrlWithFake("127.0.0.1:2379/app.properties", fail)
	assert.That(t, err).Nil()
	defer c.Close()
	m, err := c.Load(context.Background(), true, "127.0.0.1:2379/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, len(m)).Equal(0)

	empty := newFakeEtcd().get(&clientv3.GetResponse{Header: &pb.ResponseHeader{Revision: 3}}, nil)
	c2, err := newCtrlWithFake("127.0.0.1:2379/app.properties", empty)
	assert.That(t, err).Nil()
	defer c2.Close()
	m, err = c2.Load(context.Background(), true, "127.0.0.1:2379/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, len(m)).Equal(0)
}

func TestWatchLoopConsumesEvents(t *testing.T) {
	// A watch response carrying an event must reach the loop (that event is what
	// fires the refresh). The test observes the fake being drained, then Close
	// cancels the generation so the goroutine exits.
	fake := newFakeEtcd().
		get(kvResponse(7, "a=1\n"), nil).
		watch(clientv3.WatchResponse{
			Header: pb.ResponseHeader{Revision: 9},
			Events: []*clientv3.Event{{
				Type: clientv3.EventTypePut,
				Kv:   &mvccpb.KeyValue{Key: []byte("app"), Value: []byte("a=2\n"), ModRevision: 9},
			}},
		})
	c, err := newCtrlWithFake("127.0.0.1:2379/app.properties", fake)
	assert.That(t, err).Nil()
	_, err = c.Load(context.Background(), false, "127.0.0.1:2379/app.properties")
	assert.That(t, err).Nil()
	defer c.Close()

	deadline := time.Now().Add(5 * time.Second)
	for fake.delivered() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	assert.That(t, fake.delivered()).Equal(1)
	assert.That(t, fake.watchCalls()).Equal(1)
}
