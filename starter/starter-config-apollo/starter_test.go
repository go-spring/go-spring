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

package StarterConfigApollo

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agstorage "github.com/apolloconfig/agollo/v4/storage"
	"go-spring.org/spring/gs"
	"go-spring.org/stdlib/testing/assert"
)

// mockApollo serves the endpoints agollo needs: meta service discovery, the
// namespace content, and the long-poll notification endpoint. It mirrors the
// mock in example/main.go — the starter's contract is the provider seam, so a
// mock service covers it end to end without the Apollo stack.
func mockApollo(t *testing.T, content string) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/services/config":
			http.Error(w, "[]", http.StatusOK) // empty: agollo falls back to the source address
		case "/configfiles/json/demo/default/application":
			_, _ = w.Write([]byte(content))
		case "/notifications/v2":
			// Real Apollo holds the poll open; sleeping keeps the loop from
			// spinning while the app instance is up.
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusNotModified)
		default:
			_, _ = w.Write([]byte(`[{"appName":"demo","instanceId":"mock","homepageUrl":"` + srv.URL + `"}]`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestApolloAcrossAppInstances is the end-to-end counterpart of the
// fake-backed tests: it drives the real agollo client against a mock service and
// runs two application instances in one process. The second instance is the
// Close-then-Load path required by provider.Provider — exactly what sequential
// gs.RunTest runs do — so it proves the client can be stopped and rebuilt, not
// just that the controller clears its own cache.
func TestApolloAcrossAppInstances(t *testing.T) {
	srv := mockApollo(t, `{"demo.message":"hello-from-apollo"}`)
	source := "apollo:" + strings.TrimPrefix(srv.URL, "http://") + "/application?appId=demo"

	// The import must be declared in a loaded config file (imports are read from
	// the file's own properties, not from the layered storage), so point the app
	// at a throwaway config directory holding one such file.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.properties"),
		[]byte("spring.config.import="+source+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	loadOnce := func() string {
		var got string
		gs.Web(false).Configure(func(app gs.App) {
			app.Property("spring.app.config.dir", dir)
		}).RunTest(t, func(d *struct {
			Message gs.Dync[string] `value:"${demo.message:=none}"`
		}) {
			got = d.Message.Value()
		})
		return got
	}

	assert.That(t, loadOnce()).Equal("hello-from-apollo")
	// The first instance has shut down and closed the provider by now; a fresh
	// instance must be able to start polling again.
	assert.That(t, loadOnce()).Equal("hello-from-apollo")
}

// TestParseSource pins the source grammar and defaults: cluster "default",
// format inferred from the namespace extension.
func TestParseSource(t *testing.T) {
	cs, err := parseSource("127.0.0.1:8080/application.properties?appId=demo")
	assert.Error(t, err).Nil()
	assert.That(t, cs.server).Equal("127.0.0.1:8080")
	assert.That(t, cs.namespace).Equal("application.properties")
	assert.That(t, cs.appID).Equal("demo")
	assert.That(t, cs.cluster).Equal("default")
	assert.That(t, cs.format).Equal("properties")
}

// TestParseSourceMissingAppID pins the required appId.
func TestParseSourceMissingAppID(t *testing.T) {
	_, err := parseSource("127.0.0.1:8080/application")
	assert.That(t, err != nil).True()
}

// TestParseSourceFormatOverride pins an explicit format query over the
// namespace extension.
func TestParseSourceFormatOverride(t *testing.T) {
	cs, err := parseSource("h:1/ns?appId=demo&format=yaml&cluster=prod")
	assert.Error(t, err).Nil()
	assert.That(t, cs.format).Equal("yaml")
	assert.That(t, cs.cluster).Equal("prod")
}

// fakeApolloClient is an in-memory apolloClient: it hands out a fixed
// namespace content and counts change-listener registrations.
type fakeApolloClient struct {
	content string // namespace content GetConfig returns; empty means "not loaded"
	ns      string // namespace GetConfig is called with
	listens int
	closed  int
}

func (f *fakeApolloClient) GetConfigContent(namespace string) string {
	f.ns = namespace
	return f.content
}

func (f *fakeApolloClient) AddChangeListener(agstorage.ChangeListener) {
	f.listens++
}

func (f *fakeApolloClient) Close() {
	f.closed++
}

// newCtrlWithFake pre-seeds the controller's client cache for source so
// clientFor returns the fake without any network.
func newCtrlWithFake(source string, fake *fakeApolloClient) (*apolloCtrl, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}
	c := newApolloCtrl()
	c.clients[clientKey(cs)] = fake
	return c, nil
}

// TestCloseReleasesClientAndLoadRearms pins the provider lifecycle: Close stops
// the client it created and drops it, so a later Load (a new application
// instance in the same process) cannot reuse a stopped client and re-listens
// from scratch.
func TestCloseReleasesClientAndLoadRearms(t *testing.T) {
	const source = "127.0.0.1:8080/application?appId=demo"
	fake := &fakeApolloClient{content: "greeting=hello\n"}
	c, err := newCtrlWithFake(source, fake)
	assert.That(t, err).Nil()

	if _, err = c.Load(false, source); err != nil {
		t.Fatalf("Load: %v", err)
	}
	assert.Number(t, fake.listens).Equal(1)
	assert.Number(t, len(c.clients)).Equal(1)

	assert.That(t, c.Close()).Nil()
	assert.Number(t, fake.closed).Equal(1)
	assert.Number(t, len(c.clients)).Equal(0)
	assert.Number(t, len(c.listened)).Equal(0)

	// Re-seeding the cache stands in for the fresh client the next Load would
	// build: the controller must have dropped the old one, and it must listen
	// again.
	cs, err := parseSource(source)
	assert.That(t, err).Nil()
	c.clients[clientKey(cs)] = fake

	if _, err = c.Load(false, source); err != nil {
		t.Fatalf("Load after Close: %v", err)
	}
	assert.Number(t, fake.listens).Equal(2)
}

// TestLoadProperties pins the happy path: properties content parsed and
// flattened into the returned map.
func TestLoadProperties(t *testing.T) {
	c, err := newCtrlWithFake("127.0.0.1:8080/application?appId=demo", &fakeApolloClient{content: "greeting=hello\nnum=42\n"})
	assert.That(t, err).Nil()
	m, err := c.Load(false, "127.0.0.1:8080/application?appId=demo")
	assert.That(t, err).Nil()
	assert.That(t, m["greeting"]).Equal("hello")
	assert.That(t, m["num"]).Equal("42")
}

// TestLoadOptionalSkipsOnMissingNamespace pins the optional semantics: an
// absent (never synced) namespace skips instead of failing startup, while a
// non-optional import of the same source fails loudly.
func TestLoadOptionalSkipsOnMissingNamespace(t *testing.T) {
	src := "127.0.0.1:8080/application?appId=demo"
	c, err := newCtrlWithFake(src, &fakeApolloClient{})
	assert.That(t, err).Nil()

	m, err := c.Load(true, src)
	assert.That(t, err).Nil()
	assert.That(t, len(m)).Equal(0)

	_, err = c.Load(false, src)
	assert.That(t, err).NotNil()
}

// TestLoadParseErrorPropagates pins that content not matching the declared
// format fails the load.
func TestLoadParseErrorPropagates(t *testing.T) {
	c, err := newCtrlWithFake("127.0.0.1:8080/app.json?appId=demo", &fakeApolloClient{content: "{not-json"})
	assert.That(t, err).Nil()
	_, err = c.Load(false, "127.0.0.1:8080/app.json?appId=demo")
	assert.That(t, err).NotNil()
}

// TestListenerRegisteredOncePerSource pins the dedup: two Loads of the same
// source (a refresh re-loads every import) register exactly one listener.
func TestListenerRegisteredOncePerSource(t *testing.T) {
	src := "127.0.0.1:8080/application?appId=demo"
	fake := &fakeApolloClient{content: "a=1\n"}
	c, err := newCtrlWithFake(src, fake)
	assert.That(t, err).Nil()
	for i := 0; i < 2; i++ {
		_, err = c.Load(false, src)
		assert.That(t, err).Nil()
	}
	assert.That(t, fake.listens).Equal(1)
}

// TestListenerChangeFiresRefresh pins the listener seam: firing OnChange /
// OnNewestChange reaches TriggerRefresh, which must be a harmless no-op before
// the app has started rather than a nil dereference.
func TestListenerChangeFiresRefresh(t *testing.T) {
	l := &apolloListener{ctrl: newApolloCtrl()}
	l.OnChange(&agolloChangeEvent{})
	l.OnNewestChange(&agolloFullChangeEvent{})
	c := newApolloCtrl()
	c.TriggerRefresh(context.Background())
}
