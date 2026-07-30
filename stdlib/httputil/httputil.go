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

// Package httputil provides small, OTel-free helpers for server starters (gin,
// echo, fiber, ...) to derive OpenTelemetry HTTP semantic-convention attribute
// values from an inbound request, plus a header-flattening convenience.
//
// The semconv functions compute only the *values* (strings/ints) for the
// attributes - they return plain Go types, not otel attribute.KeyValue, so the
// package stays free of any OTel dependency. Callers wrap the results with
// attribute.String/Int as needed.
//
// The conventions followed are the OTel HTTP semconv stable since v1.27.0:
// url.scheme, network.protocol.version, server.address, server.port.
package httputil

import (
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Scheme returns the OTel url.scheme value for a request: "https" when it
// arrived over TLS, "http" otherwise.
func Scheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// ProtocolVersion maps a request's Proto ("HTTP/1.1", "HTTP/2.0", ...) to the
// OTel network.protocol.version value ("1.1", "2", "3"). It takes the proto
// string rather than a *http.Request so non-HTTP/1.1 transports (gRPC, HTTP/3
// via quic) that obtain the protocol version elsewhere can reuse it.
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

// ServerAddrPort splits a Host header value into the OTel server.address (host)
// and server.port. The port is returned as 0 when absent or the scheme default
// (80 for http, 443 for https), since the semconv makes server.port
// conditionally required only when non-default. scheme is the url.scheme value
// ("http"/"https"), typically from [Scheme].
func ServerAddrPort(host, scheme string) (addr string, port int) {
	addr = host
	if h, p, err := net.SplitHostPort(host); err == nil {
		addr = h
		port, _ = strconv.Atoi(p)
	}
	if (scheme == "https" && port == 443) || (scheme == "http" && port == 80) {
		port = 0
	}
	return addr, port
}

// FlattenHeader renders a header map as a single "Key: Value; Key: Value" string,
// joining multi-value headers with repeated keys. It is a convenience for log
// fields and similar single-string summaries where a structured header map is
// not wanted; the iteration order is non-deterministic (map over http.Header).
func FlattenHeader(h http.Header) string {
	var b strings.Builder
	for k, vs := range h {
		for _, v := range vs {
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
