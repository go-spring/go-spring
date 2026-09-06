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

package canonical

import (
	"context"
	"net/http"
	"slices"
	"strings"

	"go-spring.org/cloud/governance/traffic"
)

// HeaderLoadTest is the canonical HTTP header carrying the load-test marker.
// It is also read case-insensitively via [net/http.Header.Get], so
// "x-loadtest" / "X-LoadTest" / "X-LOADTEST" all match.
//
// It is a package variable, not a constant, so a company that stamps synthetic
// traffic with its own header name can re-bind it in an init() — to the marker
// its own load generator or gateway already writes. Every traffic read/write
// follows the current value: the HTTP [PropagateHTTP] helpers below, the
// LoadTest middleware default in each server starter, and the outbound
// [InjectHTTP]. Re-bind before serving; the go-spring canonical "X-LoadTest"
// is the default, so a plain go-spring deployment needs no change.
var HeaderLoadTest = "X-LoadTest"

// MetaKeyLoadTest is the gRPC / non-HTTP metadata key carrying the load-test
// marker. gRPC requires lowercase metadata keys, so the HTTP header name is not
// reused verbatim. starter-grpc adapts a metadata.MD to the generic [Carrier]
// and uses this key.
//
// Like [HeaderLoadTest] it is a package variable a company can re-bind to its
// own convention in an init(). Note the lower-case constraint still applies —
// gRPC lowercases metadata keys itself, so bind a lower-case key here.
var MetaKeyLoadTest = "x-loadtest"

// Carrier is the generic string-multi-map shape both [net/http.Header] and a
// gRPC metadata.MD satisfy (a metadata.MD is literally a map[string][]string).
// The propagator reads and writes the marker through a Carrier so the
// protocol-specific glue stays out of this package: starter-grpc casts its
// metadata.MD to Carrier and reuses the same helpers starter-gin uses on an
// http.Header.
type Carrier map[string][]string

// Has reports whether c carries the load-test marker under key. Key matching is
// canonicalised by the caller: HTTP callers pass [HeaderLoadTest] and rely on
// [net/http.Header.Get] case folding; gRPC callers pass [MetaKeyLoadTest]
// (metadata keys are already lowercased by gRPC).
func (c Carrier) Has(key string) bool {
	if c == nil {
		return false
	}
	return slices.ContainsFunc(c[key], isTruthy)
}

// Set writes the load-test marker into c under key. It is idempotent: calling
// it twice does not append a second value.
func (c Carrier) Set(key string) {
	if c == nil {
		return
	}
	c[key] = []string{"1"}
}

// IsAffirmative reports whether s is an affirmative load-test marker value
// ("1", "true", "on", "yes", "t", case-insensitive). Server starters (gin,
// echo, hertz, ...) use it to test a raw inbound header/metadata value before
// tagging the request context via [WithLoadTest], so the truthy rule stays in
// one place. Any other value (including "0", "false", "no", "") is negative.
func IsAffirmative(s string) bool { return isTruthy(s) }

// isTruthy reports whether s carries a load-test-affirmative value. The marker
// header is considered set by any of "1", "true", "on", "yes", "t"
// (case-insensitive); any other value (including "0", "false", "no", "") is
// treated as absent.
func isTruthy(s string) bool {
	for _, v := range []string{"1", "true", "on", "yes", "t"} {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

// ExtractHTTP tags ctx as load-test traffic when req carries the canonical
// marker header ([HeaderLoadTest]); otherwise it returns ctx unchanged. The
// marker's source is recorded as "http-header".
//
// It is the inbound seam for HTTP servers: a gin/echo/hertz middleware calls
// ExtractHTTP on the incoming request and threads the returned ctx through
// the handler chain.
//
// The header lookup goes through [net/http.Header.Get], which canonicalises
// the key, so any case spelling of the header name matches. The raw [Carrier]
// map helpers are NOT used here because HTTP header keys are textproto-
// canonicalised on insert (e.g. "X-LoadTest" is stored as "X-Loadtest"),
// which a plain map lookup would miss.
func ExtractHTTP(ctx context.Context, req *http.Request) context.Context {
	if req == nil || !isTruthy(req.Header.Get(HeaderLoadTest)) {
		return ctx
	}
	return WithLoadTest(ctx, "http-header")
}

// InjectHTTP writes the canonical marker header ([HeaderLoadTest]) onto req when
// ctx is a load-test context (the parent [traffic.IsLoadTest]); otherwise it is
// a no-op. It is the outbound seam for HTTP clients: before sending a request,
// an http.RoundTripper (or the starter's transport) calls InjectHTTP so the
// downstream hop can recognise the traffic. The header is set via
// [net/http.Header.Set] so the key is canonicalised correctly.
func InjectHTTP(ctx context.Context, req *http.Request) {
	if req == nil || !traffic.IsLoadTest(ctx) {
		return
	}
	req.Header.Set(HeaderLoadTest, "1")
}

// ExtractCarrier is the inbound seam for non-HTTP protocols whose metadata is
// a [Carrier] (notably gRPC metadata.MD). It tags ctx when c carries the
// marker under key; the source is recorded as source (e.g. "grpc-metadata").
// Pass [MetaKeyLoadTest] for the default metadata key (gRPC lowercases keys
// itself, so no canonicalisation happens here).
func ExtractCarrier(ctx context.Context, c Carrier, key, source string) context.Context {
	if !c.Has(key) {
		return ctx
	}
	return WithLoadTest(ctx, source)
}

// InjectCarrier is the outbound seam for non-HTTP protocols whose metadata is
// a [Carrier]. When ctx is a load-test context it writes the marker under
// key; otherwise it is a no-op.
func InjectCarrier(ctx context.Context, c Carrier, key string) {
	if !traffic.IsLoadTest(ctx) {
		return
	}
	c.Set(key)
}
