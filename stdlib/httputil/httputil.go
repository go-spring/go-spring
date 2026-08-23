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

// Package httputil provides small helpers for deriving common values from an
// HTTP request - the scheme, the protocol version, and the server address and
// port - plus a header-flattening convenience. All functions return plain Go
// types (string/int) and depend only on the standard library.
package httputil

import (
	"net"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Scheme returns the scheme of a request: "https" when it arrived over TLS,
// "http" otherwise.
func Scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// ProtocolVersion maps a request's Proto ("HTTP/1.1", "HTTP/2.0", ...) to its
// version number ("1.1", "2", "3"). It takes the proto string rather than a
// *http.Request so non-HTTP/1.1 transports (gRPC, HTTP/3 via quic) that obtain
// the protocol version elsewhere can reuse it.
func ProtocolVersion(proto string) string {
	switch proto {
	case "HTTP/1.0":
		return "1.0"
	case "HTTP/1.1":
		return "1.1"
	case "HTTP/2.0", "HTTP/2":
		return "2"
	case "HTTP/3.0", "HTTP/3":
		return "3"
	default:
		return proto
	}
}

// ServerAddrPort splits a Host header value into its host and port. The port is
// returned as 0 when absent or equal to the scheme's default (80 for http, 443
// for https). scheme is "http"/"https", typically from [Scheme].
func ServerAddrPort(host, scheme string) (addr string, port int) {
	addr = host
	if h, p, err := net.SplitHostPort(host); err == nil {
		addr = h
		port, _ = strconv.Atoi(p)
	} else if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		// A bare bracketed IPv6 literal without a port (e.g. "[::1]"):
		// SplitHostPort requires a port, so strip the brackets to match the
		// with-port case.
		addr = host[1 : len(host)-1]
	}
	if (scheme == "https" && port == 443) || (scheme == "http" && port == 80) {
		port = 0
	}
	return addr, port
}

// FlattenHeader renders a header map as a single "Key: Value; Key: Value" string,
// joining multi-value headers with repeated keys. It is a convenience for log
// fields and similar single-string summaries where a structured header map is
// not wanted. Keys are emitted in sorted order so the output is deterministic
// across calls; values within a key keep their original order.
func FlattenHeader(h http.Header) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		for _, v := range h[k] {
			if b.Len() > 0 {
				b.WriteString("; ")
			}
			b.WriteString(k)
			b.WriteString(": ")
			b.WriteString(v)
		}
	}
	return b.String()
}
