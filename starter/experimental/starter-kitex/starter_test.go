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

package StarterKitex

import (
	"net"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

// setGlobalTracerProvider installs a real SDK TracerProvider as the OTel
// process global (mirroring what starter-otel does in its setup phase) and
// registers a cleanup that restores the no-op default, so the process globals
// are not poisoned for later tests.
func setGlobalTracerProvider(t *testing.T) {
	t.Helper()
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(tracenoop.NewTracerProvider())
		_ = tp.Shutdown(t.Context())
	})
}

// TestGlobalTracingActive covers the global-pipeline probe: with the default
// (unset) globals it must report false; with a real SDK provider installed it
// must report true.
func TestGlobalTracingActive(t *testing.T) {
	if globalTracingActive() {
		t.Fatal("expected no global pipeline with unset otel globals")
	}
	setGlobalTracerProvider(t)
	if !globalTracingActive() {
		t.Fatal("expected global pipeline with an installed sdk tracer provider")
	}
}

// TestObservabilityOptionsGlobalPipeline covers the convergence path: when a
// global pipeline is live, tracing attaches the suite WITHOUT creating a local
// provider, metrics ride the global pipeline when no port is set, and an
// explicit port still wins for a dedicated prometheus endpoint.
func TestObservabilityOptionsGlobalPipeline(t *testing.T) {
	setGlobalTracerProvider(t)

	t.Run("no explicit metrics port", func(t *testing.T) {
		cfg := Config{ServiceName: "svc", Tracing: TracingCfg{Enable: true}}
		opts, localProvider := observabilityOptions(cfg)
		if localProvider != nil {
			t.Fatal("global pipeline present: must not create a local otel provider")
		}
		// Exactly one option: the tracing suite. No prometheus tracer (no port).
		if len(opts) != 1 {
			t.Fatalf("expected 1 option (tracing suite only), got %d", len(opts))
		}
	})

	t.Run("explicit metrics port", func(t *testing.T) {
		cfg := Config{ServiceName: "svc", Tracing: TracingCfg{Enable: true}, Metrics: MetricsCfg{Enable: true, Port: freePort(t), Path: "/metrics"}}
		opts, localProvider := observabilityOptions(cfg)
		if localProvider != nil {
			t.Fatal("global pipeline present: must not create a local otel provider")
		}
		if len(opts) != 2 {
			t.Fatalf("expected 2 options (suite + prometheus tracer), got %d", len(opts))
		}
	})
}

// TestObservabilityOptionsLocalFallback covers the kitex-only path: with no
// global pipeline, tracing builds its own provider and metrics stay dormant
// unless a port is explicitly configured (never a defaulted server port).
func TestObservabilityOptionsLocalFallback(t *testing.T) {
	if globalTracingActive() {
		t.Skip("a global pipeline is installed in this process; local fallback unreachable")
	}

	t.Run("zero config", func(t *testing.T) {
		cfg := Config{ServiceName: "svc", Tracing: TracingCfg{Enable: true}}
		opts, localProvider := observabilityOptions(cfg)
		if localProvider == nil {
			t.Fatal("no global pipeline: expected a local otel provider fallback")
		}
		t.Cleanup(func() {
			_ = localProvider.Shutdown(t.Context())
			// NewOpenTelemetryProvider installs itself as the OTel global;
			// restore the no-op default so later tests are unaffected.
			otel.SetTracerProvider(tracenoop.NewTracerProvider())
		})
		if len(opts) != 1 {
			t.Fatalf("expected 1 option (tracing suite only), got %d", len(opts))
		}
	})

	t.Run("tracing disabled", func(t *testing.T) {
		cfg := Config{ServiceName: "svc", Tracing: TracingCfg{Enable: false}}
		opts, localProvider := observabilityOptions(cfg)
		if localProvider != nil {
			t.Fatal("tracing.enable=false: must not create any provider or suite")
		}
		if len(opts) != 0 {
			t.Fatalf("expected no options, got %d", len(opts))
		}
	})
}

// freePort asks the kernel for a free TCP port and hands it back after closing
// the probe listener, so the prometheus tracer test binds a real port without
// collisions.
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve a free port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
