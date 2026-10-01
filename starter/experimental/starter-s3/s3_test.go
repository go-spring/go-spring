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
	"go-spring.org/cloud/observability"
	"go-spring.org/stdlib/testing/assert"
	"go.opentelemetry.io/otel/attribute"
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
// afterwards — the mechanism the wrapper uses to install observe+resilience
// after construction.
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

// TestDeclareTransportDeclaresOperation proves the declaration seam puts the
// request's identity on the context it hands inward: the resilience emitter
// reads it there at Execute entry, so a declaration that did not survive the
// inward call would mean no signals at all. It also pins the cardinality fix:
// the bounded HTTP method is a label (Attrs), the unbounded path is detail
// (Detail, as db.statement).
func TestDeclareTransportDeclaresOperation(t *testing.T) {
	var got observability.Operation
	var has bool
	next := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		got, has = observability.OperationFrom(r.Context())
		return httptest.NewRecorder().Result(), nil
	})

	tr := &declareTransport{next: next}
	req := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:9000/bucket/key", nil)
	_, err := tr.RoundTrip(req)
	assert.Error(t, err).Nil()
	assert.That(t, has).True()
	assert.That(t, got.Name).Equal("PUT /bucket/key")
	assert.That(t, got.Metric).Equal("db.client")
	assert.That(t, got.LogTag).Equal(accessTag)
	assert.That(t, attrString(got.Attrs, "db.operation")).Equal("PUT")
	assert.That(t, attrString(got.Attrs, "db.system")).Equal("s3")
	assert.That(t, attrString(got.Detail, "db.statement")).Equal("/bucket/key")
}

// attrString returns the string value of key in attrs, or "" when absent.
func attrString(attrs []attribute.KeyValue, key string) string {
	for _, a := range attrs {
		if string(a.Key) == key {
			return a.Value.AsString()
		}
	}
	return ""
}

// roundTripFunc adapts a function to http.RoundTripper for the swap test.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
