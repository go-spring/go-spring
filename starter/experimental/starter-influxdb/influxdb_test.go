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
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	influxhttp "github.com/influxdata/influxdb-client-go/v2/api/http"
	"github.com/influxdata/influxdb-client-go/v2/domain"
	"go-spring.org/stdlib/testing/assert"
)

// TestHealthError pins the /health mapping: pass is healthy, anything else
// carries the reported message.
func TestHealthError(t *testing.T) {
	pass := domain.HealthCheckStatusPass
	assert.Error(t, healthError(&domain.HealthCheck{Status: pass})).Nil()

	fail := domain.HealthCheckStatusFail
	msg := "corrupt tsdb"
	err := healthError(&domain.HealthCheck{Status: fail, Message: &msg})
	assert.That(t, err != nil).True()
	assert.That(t, strings.Contains(err.Error(), "corrupt tsdb")).True()
}

// TestClientWriteErrorClassification pins the split: a 4xx is the server's
// verdict on the write; 5xx, throttling and timeouts are conditions of the
// server; a transport error has no status at all.
func TestClientWriteErrorClassification(t *testing.T) {
	for _, code := range []int{
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusNotFound, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity,
	} {
		assert.That(t, isClientWriteError(&influxhttp.Error{StatusCode: code})).True()
	}
	for _, code := range []int{
		0, http.StatusRequestTimeout, http.StatusTooManyRequests,
		http.StatusInternalServerError, http.StatusServiceUnavailable,
	} {
		assert.That(t, isClientWriteError(&influxhttp.Error{StatusCode: code})).False()
	}
	assert.That(t, isClientWriteError(errors.New("connection refused"))).False()
}

// TestDynamicTransportSwap proves the indirection passes through to the base
// transport until Swap installs a replacement — the mechanism [NewClient] uses
// to install resilience while the client is built.
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
