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

// Package canonical is go-spring's standard load-test marker: it is the
// default implementation that the thin [traffic] contract leaves open. A
// binary that imports canonical gets the go-spring behaviour for free — tag a
// request with [WithLoadTest], and [traffic.IsLoadTest] reports it everywhere
// in the process. A company with its own convention does not import this
// package and instead installs its own [traffic.IsLoadTest].
//
// What this package carries. It answers "is the request in flight a load-test
// request?" with the smallest possible primitive: a marker carried in the
// [context.Context], installed into [traffic.IsLoadTest], and a propagator
// that ferries it across HTTP headers and gRPC metadata so every hop in a call
// chain can tell synthetic load apart from real traffic.
//
// Layering. This is the most foundational cross-cutting concern in the cloud
// stack — everything else (observe, resilience, fault) may consume the marker
// but it consumes nothing from them. It stays stdlib-only and dependency-free
// except for the parent [traffic] contract, so any starter can import it
// without pulling a heavier graph.
//
// Consumption is opt-in. Marking a context and propagating the marker costs
// almost nothing; deciding what to DO with a load-test request (shadow table,
// isolated breaker, metric tag) is intentionally NOT built in. canonical only
// carries the flag; it never acts on it.
package canonical

import (
	"context"

	"go-spring.org/cloud/governance/traffic"
)

// marker is the value type stored under ctxKey. It is unexported so the only
// way to set it is via [WithLoadTest]; callers test for presence through the
// parent [traffic.IsLoadTest], whose default this package installs. Carrying a
// Source string (rather than a bare bool) lets observability report where the
// flag entered the process — "http-header", "grpc-metadata", "loadtest.Run" —
// without expanding the public API when new entry points are added. A
// present-but-zero marker still counts as a load-test request; Source is purely
// informational.
type marker struct {
	source string
}

type ctxKey struct{}

// installDefault is run on import so a binary that pulls canonical in sees the
// go-spring default load-test read everywhere, without any wiring step.
func init() {
	traffic.IsLoadTest = func(ctx context.Context) bool {
		_, ok := ctx.Value(ctxKey{}).(marker)
		return ok
	}
}

// WithLoadTest returns a copy of ctx tagged as carrying load-test traffic. The
// optional source records where the marker originated (a free-form label for
// telemetry, e.g. "http-header" or "loadtest.Run"); only the first value is
// kept when several are supplied. Tagging an already-tagged context keeps the
// existing source so the original entry point is preserved across hops.
//
// Passing load-test context to real downstream systems is safe: the marker is
// inert until a consumer reads it. To act on it, see the parent
// [traffic.IsLoadTest].
func WithLoadTest(ctx context.Context, source ...string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Value(ctxKey{}).(marker); ok {
		return ctx // keep the original source across propagation
	}
	src := ""
	if len(source) > 0 {
		src = source[0]
	}
	return context.WithValue(ctx, ctxKey{}, marker{source: src})
}

// Source reports the free-form label recorded when the marker was set, or the
// empty string if ctx does not carry the canonical marker. Useful for access
// logs and metrics dimensions.
func Source(ctx context.Context) string {
	m, ok := ctx.Value(ctxKey{}).(marker)
	if !ok {
		return ""
	}
	return m.source
}

// Propagate copies the canonical load-test marker from parent onto child if
// parent carries it, preserving the original source. It is the cross-goroutine
// / cross-API continuation helper: when a handler fans work out to a goroutine
// or a new background context, call Propagate(parent, child) so the child
// inherits the flag. If parent does not carry the marker, child is returned
// unchanged.
func Propagate(parent, child context.Context) context.Context {
	if parent == nil {
		return child
	}
	m, ok := parent.Value(ctxKey{}).(marker)
	if !ok {
		return child
	}
	if child == nil {
		child = context.Background()
	}
	return context.WithValue(child, ctxKey{}, m)
}
