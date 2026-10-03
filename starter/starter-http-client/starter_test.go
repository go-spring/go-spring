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

	"go-spring.org/cloud/discovery"
	"go-spring.org/cloud/governance"
	"go-spring.org/cloud/loadbalance"
	"go-spring.org/cloud/resilience"
	"go-spring.org/cloud/security"
	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
)

// TestNewRoute_ResolvesDiscoveryLabel pins the entry's ${discovery} label -> the
// directory the center carries: a cited label that names no backend fails loud
// (naming the label and what IS registered), and a known one assembles.
func TestNewRoute_ResolvesDiscoveryLabel(t *testing.T) {
	disc := discovery.NewManager(map[string]discovery.Discovery{
		"default": discovery.NewStaticDiscovery(discovery.Endpoint{Addr: "10.0.0.1:8080", Healthy: true, Weight: 1}),
	})
	lb, e := loadbalance.NewManager(nil) // no factory bean contributed
	assert.Error(t, e).Nil()
	center := governance.NewCenter(governance.Config{},
		resilience.NewManager(nil), lb, nil, disc, nil)

	_, err := newRoute(nil, "x", Config{ServiceName: "user-svc", Discovery: "nope"}, nil, center, nil)
	assert.Error(t, err).Matches("no such backend")
	assert.String(t, err.Error()).Contains("default") // the registered labels are reported

	rt, err := newRoute(nil, "x", Config{ServiceName: "user-svc", Discovery: "default"}, nil, center, nil)
	assert.That(t, err).Nil()
	assert.That(t, rt).NotNil()
	_ = rt.Close()
}

// The governance service label must be STABLE across addressing modes: an
// entry that keeps service-name set must resolve to http:<service-name> whether
// it addresses directly (addr pinned, service-name a pure label) or through
// discovery — spring.governance.client.rules scoped to the label keep matching either way.
func TestServiceLabelStableAcrossAddressingModes(t *testing.T) {
	assert.That(t, Config{ServiceName: "user-svc", Discovery: "nacos"}.toTransportConfig(nil).Service).
		Equal("http:user-svc")
	assert.That(t, Config{Addr: "10.0.0.1:8080", ServiceName: "user-svc"}.toTransportConfig(nil).Service).
		Equal("http:user-svc")
	// Only an entry with no service-name at all falls back to its address.
	assert.That(t, Config{Addr: "10.0.0.1:8080"}.toTransportConfig(nil).Service).
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
// target: it is a pure governance label there, carried via Service.
func TestDirectModeBlanksServiceNameForTransport(t *testing.T) {
	c := Config{Addr: "10.0.0.1:8080", ServiceName: "svc"}
	cfg := c.toTransportConfig(nil)
	assert.That(t, cfg.Addr).Equal("10.0.0.1:8080")
	assert.That(t, cfg.ServiceName).Equal("")
	assert.That(t, cfg.Service).Equal("http:svc")

	c2 := Config{ServiceName: "svc", Discovery: "nacos"}
	cfg2 := c2.toTransportConfig(nil)
	assert.That(t, cfg2.ServiceName).Equal("svc")
}

// The bound tls.* block is handed to httpx verbatim; the TLS surface itself is
// built by starter-http-client/httpx (covered by its own tests).
func TestTLSConfigPassthrough(t *testing.T) {
	c := Config{Addr: "10.0.0.1:8080", TLS: security.TLSConfig{Enabled: true, ServerName: "svc.internal"}}
	cfg := c.toTransportConfig(nil)
	assert.That(t, cfg.TLS.ServerName).Equal("svc.internal")
}

// An entry inherits every key it does not define from the family-wide
// spring.http-client.default bucket — one value at a time, so an entry that
// tweaks one leaf of a struct still gets the struct's other leaves.
func TestEntriesInheritFamilyDefaults(t *testing.T) {
	p := flatten.WithFallback(flatten.NewPropertiesStorage(flatten.NewProperties(map[string]string{
		"spring.http-client.default.discovery":          "consul.main",
		"spring.http-client.default.tls.ca-file":        "/etc/ca.pem",
		"spring.http-client.instances.svc.service-name": "user-svc",
		"spring.http-client.instances.svc.tls.enabled":  "true",
	})), "spring.http-client.instances", "spring.http-client.default")

	var m map[string]Config
	assert.Error(t, conf.Bind(p, &m, "${spring.http-client.instances}")).Nil()

	c := m["svc"]
	assert.That(t, c.ServiceName).Equal("user-svc")   // the entry's own leaf
	assert.That(t, c.Discovery).Equal("consul.main")  // inherited leaf
	assert.That(t, c.TLS.Enabled).True()              // the entry's own leaf
	assert.That(t, c.TLS.CAFile).Equal("/etc/ca.pem") // inherited sibling leaf
}
