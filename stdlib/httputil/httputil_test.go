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

package httputil

import (
	"crypto/tls"
	"net/http"
	"strings"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

func TestScheme(t *testing.T) {
	assert.String(t, Scheme(&http.Request{})).Equal("http")

	r := &http.Request{}
	r.TLS = &tls.ConnectionState{}
	assert.String(t, Scheme(r)).Equal("https")
}

func TestProtocolVersion(t *testing.T) {
	cases := map[string]string{
		"HTTP/1.0": "1.0",
		"HTTP/1.1": "1.1",
		"HTTP/2.0": "2",
		"HTTP/2":   "2",
		"HTTP/3.0": "3",
		"HTTP/3":   "3",
		"SPDY/3":   "SPDY/3", // unknown proto passes through unchanged
		"":         "",
	}
	for proto, want := range cases {
		assert.String(t, ProtocolVersion(proto)).Equal(want)
	}
}

func TestServerAddrPort(t *testing.T) {
	t.Run("host and explicit port", func(t *testing.T) {
		addr, port := ServerAddrPort("example.com:8080", "http")
		assert.String(t, addr).Equal("example.com")
		assert.Number(t, port).Equal(8080)
	})

	t.Run("no port", func(t *testing.T) {
		addr, port := ServerAddrPort("example.com", "http")
		assert.String(t, addr).Equal("example.com")
		assert.Number(t, port).Zero()
	})

	t.Run("http default port 80 dropped", func(t *testing.T) {
		addr, port := ServerAddrPort("example.com:80", "http")
		assert.String(t, addr).Equal("example.com")
		assert.Number(t, port).Zero("default port dropped per semconv")
	})

	t.Run("https default port 443 dropped", func(t *testing.T) {
		addr, port := ServerAddrPort("example.com:443", "https")
		assert.String(t, addr).Equal("example.com")
		assert.Number(t, port).Zero("default port dropped per semconv")
	})

	t.Run("https non-default port kept", func(t *testing.T) {
		addr, port := ServerAddrPort("example.com:8443", "https")
		assert.String(t, addr).Equal("example.com")
		assert.Number(t, port).Equal(8443)
	})

	t.Run("ipv6 host with port", func(t *testing.T) {
		addr, port := ServerAddrPort("[::1]:9000", "http")
		assert.String(t, addr).Equal("::1")
		assert.Number(t, port).Equal(9000)
	})
}

func TestFlattenHeader(t *testing.T) {
	t.Run("empty header", func(t *testing.T) {
		assert.String(t, FlattenHeader(http.Header{})).Equal("")
	})

	t.Run("single header", func(t *testing.T) {
		h := http.Header{"X-Request-Id": []string{"abc"}}
		assert.String(t, FlattenHeader(h)).Equal("X-Request-Id: abc")
	})

	// Map iteration order is non-deterministic, so assert on the set of
	// "Key: Value" pieces rather than the exact joined string.
	t.Run("multiple headers and multi-value", func(t *testing.T) {
		h := http.Header{
			"Accept": []string{"text/plain", "application/json"},
			"Host":   []string{"example.com"},
		}
		got := FlattenHeader(h)
		assert.That(t, strings.Contains(got, "Accept: text/plain")).True()
		assert.That(t, strings.Contains(got, "Accept: application/json")).True()
		assert.That(t, strings.Contains(got, "Host: example.com")).True()
		// Three pieces (two Accept values + one Host) joined by "; " -> two separators.
		assert.Number(t, strings.Count(got, "; ")).Equal(2)
	})
}
