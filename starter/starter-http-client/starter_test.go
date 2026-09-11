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

package StarterHTTPClient

import (
	"testing"

	"go-spring.org/cloud/tlsconf"
	"go-spring.org/stdlib/testing/assert"
)

// The governance resource label must be STABLE across addressing modes: an
// entry that keeps service-name set must resolve to http:<service-name> whether
// it addresses directly (addr pinned, service-name a pure label) or through
// discovery — govern.rules scoped to the label keep matching either way.
func TestResourceLabelStableAcrossAddressingModes(t *testing.T) {
	assert.That(t, Config{ServiceName: "user-svc", Discovery: "nacos"}.toTransportConfig().Resource).
		Equal("http:user-svc")
	assert.That(t, Config{Addr: "10.0.0.1:8080", ServiceName: "user-svc"}.toTransportConfig().Resource).
		Equal("http:user-svc")
	// Only an entry with no service-name at all falls back to its address.
	assert.That(t, Config{Addr: "10.0.0.1:8080"}.toTransportConfig().Resource).
		Equal("http:10.0.0.1:8080")
}

func TestValidateAddressingModes(t *testing.T) {
	assert.That(t, Config{}.validate() != nil).True()
	assert.That(t, Config{ServiceName: "svc"}.validate() != nil).True() // no addr, no discovery
	assert.That(t, Config{Addr: "10.0.0.1:8080"}.validate() == nil).True()
	assert.That(t, Config{Addr: "10.0.0.1:8080", ServiceName: "svc"}.validate() == nil).True()
	assert.That(t, Config{ServiceName: "svc", Discovery: "nacos"}.validate() == nil).True()
}

// In direct mode the service-name must NOT be handed to httpx as a discovery
// target: it is a pure governance label there, carried via Resource.
func TestDirectModeBlanksServiceNameForTransport(t *testing.T) {
	c := Config{Addr: "10.0.0.1:8080", ServiceName: "svc"}
	cfg := c.toTransportConfig()
	assert.That(t, cfg.Addr).Equal("10.0.0.1:8080")
	assert.That(t, cfg.ServiceName).Equal("")
	assert.That(t, cfg.Resource).Equal("http:svc")

	c2 := Config{ServiceName: "svc", Discovery: "nacos"}
	cfg2 := c2.toTransportConfig()
	assert.That(t, cfg2.ServiceName).Equal("svc")
}

// The bound tls.* block is handed to httpx verbatim; the TLS surface itself is
// built by starter-http-client/httpx (covered by its own tests).
func TestTLSConfigPassthrough(t *testing.T) {
	c := Config{Addr: "10.0.0.1:8080", TLS: tlsconf.TLSConfig{Enabled: true, ServerName: "svc.internal"}}
	cfg := c.toTransportConfig()
	assert.That(t, cfg.TLS.ServerName).Equal("svc.internal")
}
