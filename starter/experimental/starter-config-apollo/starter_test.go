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
	agstorage "github.com/apolloconfig/agollo/v4/storage"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

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
}

func (f *fakeApolloClient) GetConfigContent(namespace string) string {
	f.ns = namespace
	return f.content
}

func (f *fakeApolloClient) AddChangeListener(agstorage.ChangeListener) {
	f.listens++
}

// newCtrlWithFake pre-seeds the controller's client cache for source so
// clientFor returns the fake without any network.
func newCtrlWithFake(source string, fake *fakeApolloClient) (*apolloCtrl, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}
	return &apolloCtrl{clients: map[string]apolloClient{clientKey(cs): fake}}, nil
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
// OnNewestChange reaches TriggerRefresh, which before the container wires the
// PropertiesRefresher must be a harmless no-op rather than a nil dereference.
func TestListenerChangeFiresRefresh(t *testing.T) {
	l := &apolloListener{ctrl: &apolloCtrl{}}
	l.OnChange(&agolloChangeEvent{})
	l.OnNewestChange(&agolloFullChangeEvent{})
	c := &apolloCtrl{}
	c.TriggerRefresh()
}
