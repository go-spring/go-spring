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
	"errors"
	"testing"

	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"go-spring.org/stdlib/testing/assert"
)

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
	// Bad timeout-ms is rejected at parse time.
	_, err = parseSource("127.0.0.1:8848/app?timeout-ms=abc")
	assert.That(t, err).NotNil()
}

func TestSplitHostPort(t *testing.T) {
	h, p, err := splitHostPort("127.0.0.1:8848")
	assert.That(t, err).Nil()
	assert.That(t, h).Equal("127.0.0.1")
	assert.That(t, p).Equal(uint64(8848))
	_, _, err = splitHostPort("noport")
	assert.That(t, err).NotNil()
	_, _, err = splitHostPort("127.0.0.1:x")
	assert.That(t, err).NotNil()
}

// newCtrlWithFake pre-seeds the controller's client cache for source so
// clientFor returns the fake without any network.
func newCtrlWithFake(source string, fake *fakeConfigClient) (*nacosCtrl, error) {
	cs, err := parseSource(source)
	if err != nil {
		return nil, err
	}
	c := &nacosCtrl{clients: map[string]config_client.IConfigClient{clientKey(cs): fake}}
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

func TestTriggerRefreshNilRefresherIsNoop(t *testing.T) {
	// Before the IoC container autowires the PropertiesRefresher, a Nacos push
	// must be a harmless no-op rather than a nil dereference.
	c := &nacosCtrl{}
	c.TriggerRefresh()
}
