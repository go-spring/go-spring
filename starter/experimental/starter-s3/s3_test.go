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
	"go-spring.org/cloud/chain"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/minio/minio-go/v7"
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

// TestCallForDeclaresOperation proves the identity seam builds the request's
// operation with the family's vocabulary — the operation the resilience
// round-tripper passes explicitly to the executor at Execute entry. It also pins
// the cardinality fix: the bounded HTTP method is a label (Attrs), the unbounded
// path is detail (Detail, as db.statement).
func TestCallForDeclaresOperation(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "http://127.0.0.1:9000/bucket/key", nil)
	got := callFor(req)
	assert.That(t, got.Request.Action).Equal("PUT /bucket/key")
	assert.That(t, got.Observe.Metric.Name).Equal("db.client")
	assert.That(t, got.Observe.Log.Tag).Equal(accessTag)
	assert.That(t, attrString(got.Observe.Metric.Attrs, "db.operation")).Equal("PUT")
	assert.That(t, attrString(got.Observe.Metric.Attrs, "db.system")).Equal("s3")
	assert.That(t, attrString(detailAttrs(got), "db.statement")).Equal("/bucket/key")
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

// detailAttrs returns the operation's span attributes that are NOT metric
// labels — the per-call detail the old Call.Detail field held, reconstructed
// from the split destination lists.
func detailAttrs(op chain.Operation) []attribute.KeyValue {
	labeled := make(map[attribute.Key]bool, len(op.Observe.Metric.Attrs))
	for _, a := range op.Observe.Metric.Attrs {
		labeled[a.Key] = true
	}
	var out []attribute.KeyValue
	for _, a := range op.Observe.Span.Attrs {
		if !labeled[a.Key] {
			out = append(out, a)
		}
	}
	return out
}
