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

package StarterGrpc

import (
	"testing"
	"time"

	"go-spring.org/spring/conf"
	"go-spring.org/stdlib/flatten"
	"go-spring.org/stdlib/testing/assert"
	"google.golang.org/grpc"
)

// bindConfig binds a Config from flat dotted properties the way the container
// would bind ${spring.grpc.server}, so tests exercise the same value-tag
// defaults and overrides production sees. "addr" is always supplied because the
// tag has no default — the server refuses to start without it by design (the
// port-must-be-configured policy).
func bindConfig(t *testing.T, props map[string]any) Config {
	t.Helper()
	if _, ok := props["addr"]; !ok {
		props["addr"] = ":0"
	}
	p := flatten.NewPropertiesStorage(flatten.MapProperties(props))
	var c Config
	assert.That(t, conf.Bind(p, &c)).Nil()
	return c
}

// TestConfig_BindDefaults pins the zero-config shape: health and load-test
// identification plus the OTel no-op tracing/metrics pair are on by default;
// every size/timeout knob stays at "leave the gRPC default" (0).
func TestConfig_BindDefaults(t *testing.T) {
	c := bindConfig(t, map[string]any{})

	assert.That(t, c.Addr).Equal(":0")
	assert.That(t, c.Health.Enabled).True()
	assert.That(t, c.LoadTest.Enabled).True()
	assert.That(t, c.Observer.Tracing.Enabled).True()
	assert.That(t, c.Observer.Metrics.Enabled).True()

	assert.That(t, c.ConnectionTimeout).Equal(time.Duration(0))
	assert.That(t, c.MaxRecvMsgSize).Equal(0)
	assert.That(t, c.MaxSendMsgSize).Equal(0)
	assert.That(t, c.MaxConcurrentStreams).Equal(uint32(0))
	assert.That(t, c.TLS.Enabled).False()
	assert.That(t, c.Keepalive.Time).Equal(time.Duration(0))
	assert.That(t, c.Keepalive.MaxConnectionAge).Equal(time.Duration(0))
}

// TestConfig_BindOverrides verifies the knobs actually reach the config struct
// through the dotted property path, including the nested keepalive block.
func TestConfig_BindOverrides(t *testing.T) {
	c := bindConfig(t, map[string]any{
		"addr":                       ":9090",
		"connectionTimeout":          "5s",
		"maxRecvMsgSize":             1024,
		"maxConcurrentStreams":       64,
		"keepalive.time":             "1m",
		"keepalive.maxConnectionAge": "2m",
		"tls.enabled":                false,
		"health.enabled":             false,
		"loadtest.enabled":           false,
		"observer.tracing.enabled":   false,
		"observer.metrics.enabled":   false,
	})
	assert.That(t, c.Addr).Equal(":9090")
	assert.That(t, c.ConnectionTimeout).Equal(5 * time.Second)
	assert.That(t, c.MaxRecvMsgSize).Equal(1024)
	assert.That(t, c.MaxConcurrentStreams).Equal(uint32(64))
	assert.That(t, c.Keepalive.Time).Equal(time.Minute)
	assert.That(t, c.Keepalive.MaxConnectionAge).Equal(2 * time.Minute)
	assert.That(t, c.Health.Enabled).False()
	assert.That(t, c.LoadTest.Enabled).False()
	assert.That(t, c.Observer.Tracing.Enabled).False()
	assert.That(t, c.Observer.Metrics.Enabled).False()
}

// TestConfig_AddrRequiredWithoutDefault asserts the port-must-be-configured
// policy: binding without spring.grpc.server.addr must fail so the server
// never silently starts on a guessed address.
func TestConfig_AddrRequiredWithoutDefault(t *testing.T) {
	p := flatten.NewPropertiesStorage(flatten.MapProperties(map[string]any{}))
	var c Config
	assert.Error(t, conf.Bind(p, &c)).Matches("addr")
}

// TestBuildOptions_NoErrorOnPlainConfig smoke-tests the option assembly on a
// plain config: it must succeed. (grpc.ServerOption values are opaque, so the
// interceptor wiring itself is behavior-tested in the per-interceptor tests.)
func TestBuildOptions_NoErrorOnPlainConfig(t *testing.T) {
	s := NewSimpleGrpcServer(bindConfig(t, map[string]any{}), func(*grpc.Server) {}, nil, nil)
	opts, err := s.buildOptions()
	assert.That(t, err).Nil()
	assert.That(t, len(opts) > 0).True() // the interceptor chains are always installed
}

// TestBuildOptions_TLSBuildErrorPropagates verifies a broken TLS config fails
// assembly with the "grpc: build TLS" explanation instead of starting plain.
func TestBuildOptions_TLSBuildErrorPropagates(t *testing.T) {
	s := NewSimpleGrpcServer(bindConfig(t, map[string]any{
		"tls.enabled":   true,
		"tls.cert-file": "/does/not/exist.pem",
		"tls.key-file":  "/does/not/exist.key",
	}), func(*grpc.Server) {}, nil, nil)
	_, err := s.buildOptions()
	assert.That(t, err != nil).True()
	assert.String(t, err.Error()).Contains("grpc: build TLS")
}
