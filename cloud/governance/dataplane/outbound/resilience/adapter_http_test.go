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

package resilience

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go-spring.org/cloud/governance/dataplane/outbound/chain"
	"go-spring.org/stdlib/testing/assert"
)

func TestRoundTripperNilExecIsPassThrough(t *testing.T) {
	base := http.DefaultTransport
	assert.That(t, NewRoundTripper(base, nil, "", nil) == base).True()
}

func TestRoundTripperRetriesOn5xx(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&hits, 1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	e := newBuiltin(t, Limits{Retry: chain.RetryPolicy{MaxRetries: 3}})
	client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e, "svc", nil)}

	resp, err := client.Get(srv.URL)
	assert.Error(t, err).Nil()
	assert.That(t, resp.StatusCode).Equal(http.StatusOK)
	_ = resp.Body.Close()
	assert.That(t, atomic.LoadInt32(&hits)).Equal(int32(3))
}

// A non-5xx response is an answer, not a fault: 4xx rejects the request and
// 429 asks us to back off, but neither says the unit is unhealthy. Neither may
// trip the breaker.
func TestRoundTripperNon5xxDoesNotTrip(t *testing.T) {
	for _, code := range []int{http.StatusBadRequest, http.StatusTooManyRequests} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer srv.Close()

			e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Minute})
			client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e, "svc", nil)}

			resp, err := client.Get(srv.URL)
			assert.Error(t, err).Nil()
			assert.That(t, resp.StatusCode).Equal(code)
			_ = resp.Body.Close()

			// Still closed: if the status had counted as a failure, this call
			// would come back as ErrCircuitOpen instead of the response.
			resp, err = client.Get(srv.URL)
			assert.Error(t, err).Nil()
			assert.That(t, resp.StatusCode).Equal(code)
			_ = resp.Body.Close()
		})
	}
}

func TestRoundTripperCircuitOpenIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	e := newBuiltin(t, BreakerPolicy{ErrorThreshold: 1, OpenDuration: time.Minute})
	client := &http.Client{Transport: NewRoundTripper(http.DefaultTransport, e, "svc", nil)}

	_, err := client.Get(srv.URL) // trips the breaker
	assert.Error(t, err).NotNil()
	_, err = client.Get(srv.URL) // now short-circuited
	assert.Error(t, err).Is(chain.ErrCircuitOpen)
}
