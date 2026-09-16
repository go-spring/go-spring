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

package netutil

import (
	"net"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// The result is environment-dependent, so pin the contract rather than a
// specific address: either the fallback sentinel or a real non-loopback IPv4.
func TestLocalIPv4_ReturnsUsableAddress(t *testing.T) {
	ip := LocalIPv4()
	if ip != "0.0.0.0" {
		parsed := net.ParseIP(ip)
		assert.That(t, parsed != nil).True("result is the 0.0.0.0 sentinel or a valid IP")
		assert.That(t, parsed.To4() != nil).True("result must be IPv4")
		assert.That(t, parsed.IsLoopback()).False("result must not be a loopback address")
	}
}

// The value is cached with sync.Once; a second call must return the same string.
func TestLocalIPv4_Cached(t *testing.T) {
	assert.String(t, LocalIPv4()).Equal(LocalIPv4())
}

func TestSplitHostPort(t *testing.T) {
	// Plain host:port.
	host, port, err := SplitHostPort("127.0.0.1:8848")
	assert.That(t, err).Nil()
	assert.That(t, host).Equal("127.0.0.1")
	assert.That(t, port).Equal(uint64(8848))

	// Bracketed IPv6 literals parse with the brackets stripped.
	host, port, err = SplitHostPort("[::1]:8848")
	assert.That(t, err).Nil()
	assert.That(t, host).Equal("::1")
	assert.That(t, port).Equal(uint64(8848))

	// Missing host, missing port, no colon, or a non-numeric port each
	// fail loudly.
	for _, bad := range []string{":8848", "127.0.0.1:", "127.0.0.1", "127.0.0.1:http"} {
		_, _, err = SplitHostPort(bad)
		assert.That(t, err).NotNil()
	}
}
