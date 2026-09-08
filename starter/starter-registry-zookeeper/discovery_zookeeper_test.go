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

package StarterRegistryZookeeper

import (
	"encoding/json"
	"testing"

	"go-spring.org/cloud/discovery"
	"go-spring.org/stdlib/testing/assert"
)

// compile-time contract: the adapter satisfies the discovery interface.
var _ discovery.Discovery = (*zkDiscovery)(nil)

// mustPayload marshals v into the znode payload shape the registrar writes.
func mustPayload(t *testing.T, v instanceValue) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	assert.Error(t, err).Nil()
	return b
}

func TestServicePath(t *testing.T) {
	d := &zkDiscovery{basePath: "/services"}
	// The read path is exactly where zkRegistrar.pathFor writes beneath.
	assert.That(t, d.servicePath("orders")).Equal("/services/orders")
}

func TestNormalizeBasePath(t *testing.T) {
	// Trailing slashes collapse so the service path has one separator per level.
	assert.That(t, normalizeBasePath("/services/")).Equal("/services")
	assert.That(t, normalizeBasePath("/services")).Equal("/services")
}

func TestValuesToEndpoints(t *testing.T) {
	vals := map[string][]byte{
		"orders-10.0.0.2:8080": mustPayload(t, instanceValue{ServiceName: "orders", Addr: "10.0.0.2:8080", Weight: 3,
			Metadata: map[string]string{"zone": "b", "scheme": "tls"}}),
		"orders-10.0.0.1:8080": mustPayload(t, instanceValue{ServiceName: "orders", Addr: "10.0.0.1:8080"}),
		// A malformed payload is skipped, not fatal to the snapshot.
		"orders-bad": []byte("not-json"),
	}
	eps := valuesToEndpoints(vals)

	// Sorted by address, malformed entry dropped.
	assert.That(t, len(eps)).Equal(2)
	assert.That(t, eps[0].Addr).Equal("10.0.0.1:8080")
	assert.That(t, eps[1].Addr).Equal("10.0.0.2:8080")

	// Ephemeral znode existence is the health signal: every decoded node is healthy.
	assert.That(t, eps[0].Healthy).True()
	assert.That(t, eps[1].Healthy).True()

	// Weight and metadata (incl. scheme passthrough) survive the mapping.
	assert.That(t, eps[1].Weight).Equal(3)
	assert.That(t, eps[1].Scheme).Equal("tls")
	assert.That(t, eps[1].Metadata["zone"]).Equal("b")
}

func TestValuesToEndpointsDrainEncoding(t *testing.T) {
	// The drain signal (weight omitted from the payload) reconstructs as 0 —
	// the endpoint stays in the snapshot but load-balance picking excludes it.
	eps := valuesToEndpoints(map[string][]byte{
		"a": mustPayload(t, instanceValue{ServiceName: "orders", Addr: "10.0.0.1:8080", Weight: 0}),
	})
	assert.That(t, len(eps)).Equal(1)
	assert.That(t, eps[0].Weight).Equal(0)
}

func TestValuesToEndpointsSchemeFilter(t *testing.T) {
	vals := map[string][]byte{
		"s-10.0.0.1:80": mustPayload(t, instanceValue{Addr: "10.0.0.1:80", Metadata: map[string]string{"scheme": "tls"}}),
		"s-10.0.0.2:80": mustPayload(t, instanceValue{Addr: "10.0.0.2:80"}),
	}
	// FilterByScheme narrows to the tls instance only.
	eps := discovery.FilterByScheme(valuesToEndpoints(vals), "tls")
	assert.That(t, len(eps)).Equal(1)
	assert.That(t, eps[0].Addr).Equal("10.0.0.1:80")
}

func TestFirstEventEmptyIsNil(t *testing.T) {
	// No children armed: firstEvent returns nil, which blocks in a select —
	// the correct "nothing to wait on" case.
	assert.That(t, firstEvent(nil) == nil).True()
}
