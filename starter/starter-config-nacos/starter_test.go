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

package StarterConfigNacos

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"go-spring.org/stdlib/testing/assert"
)

// fakeConfigClient stands in for a nacos server: it serves one document and
// records the installed listener. Unimplemented methods come from the embedded
// interface (nil panics if wrongly touched). getErr/listenErr inject failures
// for error-path tests; listens counts ListenConfig calls so listener-dedup can
// be asserted; closes counts CloseClient calls for lifecycle tests.
type fakeConfigClient struct {
	config_client.IConfigClient

	mu        sync.Mutex
	data      string
	getErr    error
	listenErr error
	listens   int
	closes    int
	onChange  func(namespace, group, dataId, data string)
}

func (f *fakeConfigClient) GetConfig(vo.ConfigParam) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data, f.getErr
}

func (f *fakeConfigClient) ListenConfig(p vo.ConfigParam) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.listenErr != nil {
		return f.listenErr
	}
	f.listens++
	f.onChange = p.OnChange
	return nil
}

func (f *fakeConfigClient) CancelListenConfig(vo.ConfigParam) error { return nil }

func (f *fakeConfigClient) CloseClient() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closes++
}

func TestParseSource(t *testing.T) {
	// Full form: every query knob spelled out.
	cs, err := parseSource("127.0.0.1:8848/app.yaml?group=G&namespace=ns&username=u&password=p&timeout-ms=1000&format=yaml")
	assert.That(t, err).Nil()
	assert.That(t, cs.server).Equal("127.0.0.1:8848")
	assert.That(t, cs.dataID).Equal("app.yaml")
	assert.That(t, cs.group).Equal("G")
	assert.That(t, cs.namespace).Equal("ns")
	assert.That(t, cs.username).Equal("u")
	assert.That(t, cs.password).Equal("p")
	assert.That(t, cs.timeoutMs).Equal(uint64(1000))
	assert.That(t, cs.format).Equal("yaml")

	// Defaults: DEFAULT_GROUP, format inferred from the data-id extension.
	cs, err = parseSource("127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, cs.group).Equal("DEFAULT_GROUP")
	assert.That(t, cs.format).Equal("properties")
	assert.That(t, cs.timeoutMs).Equal(uint64(5000))

	// No extension falls back to properties.
	cs, err = parseSource("127.0.0.1:8848/app")
	assert.That(t, err).Nil()
	assert.That(t, cs.format).Equal("properties")

	// Missing server or data id fails loudly.
	_, err = parseSource("onlydata")
	assert.That(t, err).NotNil()
	_, err = parseSource("127.0.0.1:8848")
	assert.That(t, err).NotNil()
	// Bad timeout-ms is rejected at parse time — including zero, which the
	// SDK would otherwise read as "no timeout" and stall the refresh cycle.
	_, err = parseSource("127.0.0.1:8848/app?timeout-ms=abc")
	assert.That(t, err).NotNil()
	_, err = parseSource("127.0.0.1:8848/app?timeout-ms=0")
	assert.That(t, err).NotNil()

	// An explicit path-only source has no host: the address check fires.
	_, err = parseSource("/app.properties")
	assert.That(t, err).NotNil()

	// A missing port survives parsing (host-only) — it is rejected later,
	// when the client is built.
	cs, err = parseSource("nohost/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, cs.server).Equal("nohost")
}

// newCtrlWithFake pre-seeds the controller's client cache for source so
// clientFor returns the fake without any network.
func newCtrlWithFake(source string, fake *fakeConfigClient) (*nacosCtrl, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}
	c := newNacosCtrl()
	c.clients[clientKey(cs)] = fake
	return c, nil
}

func TestLoadProperties(t *testing.T) {
	c, err := newCtrlWithFake("127.0.0.1:8848/app.properties", &fakeConfigClient{data: "greeting=hello\nnum=42\n"})
	assert.That(t, err).Nil()
	m, err := c.Load(false, "127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, m["greeting"]).Equal("hello")
	assert.That(t, m["num"]).Equal("42")
}

func TestLoadOptionalSkipsOnFetchFailureAndEmpty(t *testing.T) {
	// optional:true turns a fetch error into a skip, not a startup failure.
	c, err := newCtrlWithFake("127.0.0.1:8848/app.properties", &fakeConfigClient{getErr: errors.New("down")})
	assert.That(t, err).Nil()
	m, err := c.Load(true, "127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, len(m)).Equal(0)

	// Same for an empty-but-present config.
	c2, err := newCtrlWithFake("127.0.0.1:8848/app.properties", &fakeConfigClient{})
	assert.That(t, err).Nil()
	m2, err := c2.Load(true, "127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, len(m2)).Equal(0)

	// Non-optional propagates both.
	_, err = c.Load(false, "127.0.0.1:8848/app.properties")
	assert.That(t, err).NotNil()
	_, err = c2.Load(false, "127.0.0.1:8848/app.properties")
	assert.That(t, err).NotNil()
}

func TestLoadParseErrorPropagates(t *testing.T) {
	// Content that does not parse as the declared format must fail the load.
	c, err := newCtrlWithFake("127.0.0.1:8848/app.json", &fakeConfigClient{data: "{not-json"})
	assert.That(t, err).Nil()
	_, err = c.Load(false, "127.0.0.1:8848/app.json")
	assert.That(t, err).NotNil()
}

func TestLoadInvalidSourceFailsBeforeClient(t *testing.T) {
	c := newNacosCtrl()
	_, err := c.Load(false, "127.0.0.1:8848")
	assert.That(t, err).NotNil()
	// No client was created for the malformed source.
	assert.That(t, len(c.clients)).Equal(0)
}

func TestListenerRegisteredOncePerSource(t *testing.T) {
	fake := &fakeConfigClient{data: "a=1\n"}
	c, err := newCtrlWithFake("127.0.0.1:8848/app.properties", fake)
	assert.That(t, err).Nil()
	// Two Loads of the same source (refresh re-loads every import): the
	// listener is registered exactly once — Nacos dedup keys are per
	// client+group+dataId and a second ListenConfig would double-fire.
	for i := 0; i < 2; i++ {
		_, err = c.Load(false, "127.0.0.1:8848/app.properties")
		assert.That(t, err).Nil()
	}
	assert.That(t, fake.listens).Equal(1)
}

func TestListenerFailureRetriedOnNextLoad(t *testing.T) {
	// A failed ListenConfig must not be remembered as installed: the next
	// Load retries, otherwise the dataId would silently never hot-reload.
	fake := &fakeConfigClient{data: "a=1\n", listenErr: errors.New("listen down")}
	c, err := newCtrlWithFake("127.0.0.1:8848/app.properties", fake)
	assert.That(t, err).Nil()
	_, err = c.Load(false, "127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil() // load itself succeeds; only listening failed
	assert.That(t, fake.listens).Equal(0)

	fake.mu.Lock()
	fake.listenErr = nil
	fake.mu.Unlock()
	_, err = c.Load(false, "127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, fake.listens).Equal(1)
}

func TestListenerSeparatePerGroupAndDataID(t *testing.T) {
	// Dedup is per client+group+dataId: different targets each get a listener.
	fake := &fakeConfigClient{data: "a=1\n"}
	c, err := newCtrlWithFake("127.0.0.1:8848/app.properties", fake)
	assert.That(t, err).Nil()
	for _, src := range []string{
		"127.0.0.1:8848/app.properties",
		"127.0.0.1:8848/app.properties?group=OTHER",
		"127.0.0.1:8848/other.properties",
	} {
		_, err = c.Load(false, src)
		assert.That(t, err).Nil()
	}
	assert.That(t, fake.listens).Equal(3)
}

func TestOnChangeCallbackIsSafeBeforeAppStart(t *testing.T) {
	// A push delivered before the app started must be dropped harmlessly
	// (gs.RefreshProperties errors, TriggerRefresh ignores it).
	fake := &fakeConfigClient{data: "a=1\n"}
	c, err := newCtrlWithFake("127.0.0.1:8848/app.properties", fake)
	assert.That(t, err).Nil()
	_, err = c.Load(false, "127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil()
	assert.That(t, fake.onChange).NotNil()
	fake.onChange("ns", "DEFAULT_GROUP", "app.properties", "a=2\n")
}

func TestCloseClosesClientsAndDropsCaches(t *testing.T) {
	fake := &fakeConfigClient{data: "a=1\n"}
	c, err := newCtrlWithFake("127.0.0.1:8848/app.properties", fake)
	assert.That(t, err).Nil()
	_, err = c.Load(false, "127.0.0.1:8848/app.properties")
	assert.That(t, err).Nil()

	assert.That(t, c.Close(context.Background())).Nil()
	assert.That(t, fake.closes).Equal(1)
	c.mu.Lock()
	assert.That(t, len(c.clients)).Equal(0)
	assert.That(t, len(c.listened)).Equal(0)
	assert.That(t, c.stopped).Equal(true)
	c.mu.Unlock()

	// Close is safe to call twice.
	assert.That(t, c.Close(context.Background())).Nil()
	assert.That(t, fake.closes).Equal(1)
}

func TestRearmOnNextLoadAfterClose(t *testing.T) {
	// After a Close, the next Load must re-arm the listener generation and
	// re-register the listener on the (fresh) client of the new generation.
	fake1 := &fakeConfigClient{data: "a=1\n"}
	src := "127.0.0.1:8848/app.properties"
	c, err := newCtrlWithFake(src, fake1)
	assert.That(t, err).Nil()
	_, err = c.Load(false, src)
	assert.That(t, err).Nil()
	assert.That(t, c.Close(context.Background())).Nil()

	fake2 := &fakeConfigClient{data: "a=2\n"}
	cs, err := parseSource(src)
	assert.That(t, err).Nil()
	c.mu.Lock()
	c.clients[clientKey(cs)] = fake2 // the new generation's client
	c.mu.Unlock()

	m, err := c.Load(false, src)
	assert.That(t, err).Nil()
	assert.That(t, m["a"]).Equal("2")
	assert.That(t, fake2.listens).Equal(1)
	assert.That(t, fake1.listens).Equal(1) // old client untouched

	c.mu.Lock()
	stopped := c.stopped
	c.mu.Unlock()
	assert.That(t, stopped).Equal(false)
}

func TestRegisterListenerSkipsStaleClientAfterClose(t *testing.T) {
	// Close ran between Load's clientFor and registerListener: the listener
	// must NOT be marked on the stale client, so the next generation's Load
	// still registers on its fresh client.
	fake := &fakeConfigClient{data: "a=1\n"}
	src := "127.0.0.1:8848/app.properties"
	c, err := newCtrlWithFake(src, fake)
	assert.That(t, err).Nil()
	cs, err := parseSource(src)
	assert.That(t, err).Nil()

	assert.That(t, c.Close(context.Background())).Nil()
	c.registerListener(context.Background(), fake, cs) // stale call from an in-flight Load

	c.mu.Lock()
	assert.That(t, len(c.listened)).Equal(0)
	c.mu.Unlock()

	// New generation: fresh client, listener must install.
	fake2 := &fakeConfigClient{data: "a=1\n"}
	c.mu.Lock()
	c.clients[clientKey(cs)] = fake2
	c.mu.Unlock()
	_, err = c.Load(false, src)
	assert.That(t, err).Nil()
	assert.That(t, fake2.listens).Equal(1)
}

func TestTriggerRefreshNilRefresherIsNoop(t *testing.T) {
	// Before the app has started, gs.RefreshProperties returns an error and
	// the push is dropped rather than panicking.
	c := newNacosCtrl()
	c.TriggerRefresh(context.Background())
}
