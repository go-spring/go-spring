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

package StarterInfluxdb

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/influxdata/influxdb-client-go/v2/domain"
	observe "go-spring.org/cloud/observe"
	health2 "go-spring.org/starter-influxdb/health"
	"go-spring.org/stdlib/testing/assert"
)

// TestHealthError pins the /health mapping: pass is healthy, anything else
// carries the reported message.
func TestHealthError(t *testing.T) {
	pass := domain.HealthCheckStatusPass
	assert.Error(t, health2.HealthError(&domain.HealthCheck{Status: pass})).Nil()

	fail := domain.HealthCheckStatusFail
	msg := "corrupt tsdb"
	err := health2.HealthError(&domain.HealthCheck{Status: fail, Message: &msg})
	assert.That(t, err != nil).True()
	assert.That(t, strings.Contains(err.Error(), "corrupt tsdb")).True()
}

// TestDynamicTransportSwap proves the indirection passes through to the base
// transport until Swap installs a replacement — the mechanism Init uses to
// arm observe+resilience after construction.
func TestDynamicTransportSwap(t *testing.T) {
	var served string
	base := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		served = "base"
		return httptest.NewRecorder().Result(), nil
	})
	swapped := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		served = "swapped"
		return httptest.NewRecorder().Result(), nil
	})

	dyn := newDynamicTransport()
	dyn.cur = base

	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8086/api/v2/write", nil)
	_, err := dyn.RoundTrip(req)
	assert.Error(t, err).Nil()
	assert.That(t, served).Equal("base")

	dyn.Swap(swapped)
	_, err = dyn.RoundTrip(req)
	assert.Error(t, err).Nil()
	assert.That(t, served).Equal("swapped")
}

// roundTripFunc adapts a function to http.RoundTripper for the swap test.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestResolveObservability pins the precedence between the two observability
// config surfaces: instance-prefixed spring.influxdb.<name>.observability.*
// (bound into Config.Observability by BindEach) overrides the top-level
// observability.* keys field-injected into the wrapper. Binding fills the
// defaults (brief/512/no skips) even when no instance key is present, so only
// non-default instance values count as "set".
func TestResolveObservability(t *testing.T) {
	// Instance unset (binding defaults) -> top-level field wins untouched.
	w := &Client{Observability: observe.ObserveConfig{
		Level: "detailed", MaxArgBytes: 1024, SkipOps: []string{"POST /write"},
	}}
	w.cfg.Observability = observe.ObserveConfig{Level: "brief", MaxArgBytes: 512}
	got := w.resolveObservability()
	assert.That(t, got.Level).Equal("detailed")
	assert.That(t, got.MaxArgBytes).Equal(1024)
	assert.That(t, got.SkipOps).Equal([]string{"POST /write"})

	// Instance set -> overrides top-level per field.
	w.cfg.Observability = observe.ObserveConfig{
		Level: "off", MaxArgBytes: 2048, SkipOps: []string{"POST /query"},
	}
	got = w.resolveObservability()
	assert.That(t, got.Level).Equal("off")
	assert.That(t, got.MaxArgBytes).Equal(2048)
	assert.That(t, got.SkipOps).Equal([]string{"POST /query"})

	// Partial instance override: only the field set to a non-default value
	// changes; the rest keeps the top-level value.
	w.cfg.Observability = observe.ObserveConfig{Level: "off", MaxArgBytes: 512}
	got = w.resolveObservability()
	assert.That(t, got.Level).Equal("off")
	assert.That(t, got.MaxArgBytes).Equal(1024)
}
