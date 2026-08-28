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
	"context"
	"testing"

	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/cloud/tlsconf"
	"go-spring.org/stdlib/testing/assert"
)

// The governance resource label must be STABLE across addressing modes: an
// entry that keeps service-name set must resolve to http:<service-name> whether
// it addresses directly (addr pinned, service-name a pure label) or through
// discovery — govern.rules scoped to the label keep matching either way.
func TestResourceLabelStableAcrossAddressingModes(t *testing.T) {
	assert.That(t, httpResourceLabel(Config{ServiceName: "user-svc", Discovery: "nacos"})).
		Equal("http:user-svc")
	assert.That(t, httpResourceLabel(Config{Addr: "10.0.0.1:8080", ServiceName: "user-svc"})).
		Equal("http:user-svc")
	// Only an entry with no service-name at all falls back to its address.
	assert.That(t, httpResourceLabel(Config{Addr: "10.0.0.1:8080"})).Equal("http:10.0.0.1:8080")
}

func TestValidateAddressingModes(t *testing.T) {
	assert.That(t, Config{}.validate() != nil).True()
	assert.That(t, Config{ServiceName: "svc"}.validate() != nil).True() // no addr, no discovery
	assert.That(t, Config{Addr: "10.0.0.1:8080"}.validate() == nil).True()
	assert.That(t, Config{Addr: "10.0.0.1:8080", ServiceName: "svc"}.validate() == nil).True()
	assert.That(t, Config{ServiceName: "svc", Discovery: "nacos"}.validate() == nil).True()
}

// In direct mode the service-name must NOT be handed to httpx as a discovery
// target: it is a pure governance label there.
func TestDirectModeBlanksServiceNameForTransport(t *testing.T) {
	c := Config{Addr: "10.0.0.1:8080", ServiceName: "svc"}
	cfg := c.toTransportConfig(nil, nil)
	assert.That(t, cfg.Addr).Equal("10.0.0.1:8080")
	assert.That(t, cfg.ServiceName).Equal("")

	c2 := Config{ServiceName: "svc", Discovery: "nacos"}
	cfg2 := c2.toTransportConfig(nil, nil)
	assert.That(t, cfg2.ServiceName).Equal("svc")
}

func TestFloorMinRequests(t *testing.T) {
	// error-rate policy with unset MinRequests → floored.
	p := floorMinRequests(resilience.Policy{BreakerStrategy: resilience.BreakerErrorRate, ErrorRateThreshold: 0.5})
	assert.That(t, p.MinRequests).Equal(minRequestsFloor)

	// an explicit higher value wins; a lower explicit value is raised too.
	assert.That(t, floorMinRequests(resilience.Policy{BreakerStrategy: resilience.BreakerErrorRate, ErrorRateThreshold: 0.5, MinRequests: 10}).MinRequests).Equal(10)
	assert.That(t, floorMinRequests(resilience.Policy{BreakerStrategy: resilience.BreakerErrorRate, ErrorRateThreshold: 0.5, MinRequests: 2}).MinRequests).Equal(minRequestsFloor)

	// consecutive and zero policies are untouched.
	assert.That(t, floorMinRequests(resilience.Policy{ErrorThreshold: 5}).MinRequests).Equal(0)
	assert.That(t, floorMinRequests(resilience.Policy{RateLimit: 10}).MinRequests).Equal(0)
	// error-rate strategy with the breaker disabled (no threshold) is untouched.
	assert.That(t, floorMinRequests(resilience.Policy{BreakerStrategy: resilience.BreakerErrorRate}).MinRequests).Equal(0)
}

// With governance not armed, governedExecutor still yields a working (no-op)
// executor — the same contract resilience.ExecutorFor gives.
func TestGovernedExecutorWithoutGovernance(t *testing.T) {
	exec, err := governedExecutor("http:test-svc")
	assert.That(t, err == nil).True()
	assert.That(t, exec != nil).True()
	err = exec.Execute(t.Context(), "host:1", func(ctx context.Context) error { return nil })
	assert.That(t, err == nil).True()
}

// The tls.* block binds under the entry prefix and turns into a real *tls.Config.
func TestTLSConfigBuild(t *testing.T) {
	c := Config{Addr: "10.0.0.1:8080", TLS: tlsconf.TLSConfig{Enabled: true, ServerName: "svc.internal"}}
	cfg, err := c.TLS.Build()
	assert.That(t, err == nil).True()
	assert.That(t, cfg != nil).True()
	assert.That(t, cfg.ServerName).Equal("svc.internal")

	// Off by default: plain http routes stay on system defaults.
	off, err := Config{}.TLS.Build()
	assert.That(t, err == nil).True()
	assert.That(t, off == nil).True()
}
