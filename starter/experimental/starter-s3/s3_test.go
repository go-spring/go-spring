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

package StarterS3

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/minio/minio-go/v7"
	"go-spring.org/stdlib/testing/assert"
)

// TestBucketLookupType covers the config-string mapping, including the
// virtual-host alias and the rejection of unknown values.
func TestBucketLookupType(t *testing.T) {
	cases := map[string]minio.BucketLookupType{
		"":             minio.BucketLookupAuto,
		"auto":         minio.BucketLookupAuto,
		"virtual-host": minio.BucketLookupDNS,
		"dns":          minio.BucketLookupDNS,
		"path":         minio.BucketLookupPath,
	}
	for s, want := range cases {
		got, err := bucketLookupType(s)
		assert.Error(t, err).Nil()
		assert.That(t, got).Equal(want)
	}
	_, err := bucketLookupType("bogus")
	assert.That(t, err != nil).True()
}

// TestDynamicTransportSwap proves the indirection passes through to the base
// transport until Swap installs a replacement, and serves the replacement
// afterwards — the mechanism Init uses to arm observe+resilience after
// construction.
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

	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9000/bucket/key", nil)
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
