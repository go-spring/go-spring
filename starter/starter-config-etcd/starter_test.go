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
	"testing"
	"time"

	"go-spring.org/stdlib/testing/assert"
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
