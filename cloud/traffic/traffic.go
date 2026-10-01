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

// Package traffic answers one question for the whole process — "is the request
// in flight a load-test request?" — and carries go-spring's way of answering
// it: a marker in the [context.Context], ferried across HTTP headers, gRPC
// metadata and message properties, so every hop in a call chain can tell
// synthetic load apart from real traffic.
//
// The convention itself is a [Binding]: where the flag lives in the context,
// the name the marker rides under on the wire, and the value written there.
// [DefaultBinding] is go-spring's, so an application whose load generator or
// gateway stamps synthetic traffic with a marker of its own takes that
// binding, fills in what differs, and hands it to [NewDefaultPropagator] as
// its own bean.
//
// This package holds no package-level state: one process has one propagator
// bean, components take it (a starter injects it optionally and falls back to
// [NewDefaultPropagator] over [DefaultBinding]), and business code reads the
// answer from the same instance rather than from a global. Business code never
// constructs the marker itself, and how the flag got there (a header, a
// metadata key, a message property) is out of the reader's concern.
//
// Layering. Everything else in the cloud stack (observe, resilience, fault) may
// consume the marker; this package consumes nothing from them. Its only
// dependency outside the standard library is [go-spring.org/cloud/propagate],
// the generic carrier the protocol seams are written against, so any starter
// can import it without pulling a heavier graph.
//
// Consumption is opt-in. Marking a context and propagating the marker costs
// almost nothing; deciding what to DO with a load-test request (shadow table,
// isolated breaker, metric tag) is intentionally NOT built in. A propagator
// only carries the flag; it never acts on it.
package traffic

import (
	"context"
	"strings"

	"go-spring.org/cloud/propagate"
)

const (
	// canonicalKey is the go-spring marker name, spelled lower-case so it is
	// legal everywhere at once: gRPC requires lower-case metadata keys, and
	// HTTP header names are case-insensitive, so net/http's canonicalised
	// spelling ("X-Loadtest") is the same name.
	canonicalKey = "x-loadtest"

	// canonicalValue is the canonical spelling on the wire: the [Binding.Value]
	// [DefaultBinding] sets, and the value it accepts back.
	canonicalValue = "1"
)

// Propagator is one process's load-test convention at work: the seams that move
// the marker across a protocol hop, all of them answered from its [Binding].
// [DefaultPropagator] implements it, and re-basing the convention means
// building one over a different binding — not overriding a method. A rule
// beyond equal-value matching (a legacy gateway that writes "synthetic" and
// "shadow" for the same thing) is a custom implementation of this interface,
// not a knob on [Binding].
//
// The wire vocabulary is not part of this interface: a seam is handed the hop's
// own metadata as a [propagate.Carrier] — an adapter every protocol's metadata
// already satisfies or wraps — so an adapter moves the marker without ever
// asking what it is called.
//
// An inbound method tags the context through [Propagator.WithLoadTest], and an
// outbound method is a no-op when the context is not load-test traffic. Every
// method tolerates a nil request or a nil carrier. [Propagate] — the
// context-to-context hop — is a plain function over the interface, not a
// method of it.
type Propagator interface {
	// IsLoadTest reports whether ctx carries load-test traffic.
	IsLoadTest(ctx context.Context) bool

	// WithLoadTest returns a copy of ctx tagged as load-test traffic, or ctx
	// itself when it already is. A nil ctx is treated as
	// [context.Background].
	WithLoadTest(ctx context.Context) context.Context

	// Extract tags ctx when the carrier holds the wire value under the
	// marker's name. A key matches the binding's key case-insensitively —
	// [net/http] canonicalises header keys on insert ("x-loadtest" is stored
	// as "X-Loadtest") — so one call serves an HTTP header map, a gRPC
	// metadata.MD and a broker's message properties alike. Pass the hop's own
	// metadata as a carrier, e.g. propagate.Header(req.Header).
	Extract(ctx context.Context, c propagate.Carrier) context.Context

	// Inject writes the marker onto the carrier when ctx is load-test traffic
	// and does nothing otherwise. The carrier's Set decides where the write
	// lands: a map adapter writes through to the protocol's own map, a
	// getter/setter adapter (a Kafka record's header slice) writes through
	// its setter.
	Inject(ctx context.Context, c propagate.Carrier)
}

// Binding is one process's load-test convention in data form: the slot the flag
// lives in and the name and value it is spelled with on the wire. The name is
// one string for every hop — HTTP, gRPC, the brokers — because the only
// cross-protocol variance a chain tolerates is case, and the seams absorb that.
// It is the one place a convention is re-based, and it is what lets this
// package own no context key of its own — go-spring's own slot
// ([DefaultBinding]) is used unless an application supplies its own.
//
// Every field is required: an unset Bind, Bound, Key or Value quietly turns
// every seam into a no-op, so fill the binding completely — a re-based
// convention is [DefaultBinding] with the fields that differ overwritten.
type Binding struct {
	// Key is the name the marker rides under on every hop. It is matched
	// case-insensitively, so net/http's canonicalised spelling of an HTTP
	// header is the same name.
	Key string

	// Value is what the outbound seams write and the inbound seams accept
	// back, compared case-insensitively.
	Value string

	// Bind returns a copy of ctx carrying the flag, or ctx itself when it
	// already carries it. A nil ctx is treated as [context.Background].
	Bind func(ctx context.Context) context.Context

	// Bound reports whether ctx carries the flag.
	Bound func(ctx context.Context) bool
}

// loadTestKey is the context key go-spring's slot — [DefaultBinding]'s Bind and
// Bound — stores the flag under. It is an unexported struct type, the usual
// context-key discipline so no other package can collide with it, and its
// presence is the whole payload.
type loadTestKey struct{}

// DefaultBinding returns go-spring's [Binding]: the flag is the presence of
// [loadTestKey] in the context, and the marker is named and spelled the
// canonical way. Copy it when re-basing one part of the convention.
func DefaultBinding() Binding {
	return Binding{
		Key:   canonicalKey,
		Value: canonicalValue,
		Bind: func(ctx context.Context) context.Context {
			if ctx == nil {
				ctx = context.Background()
			}
			if ctx.Value(loadTestKey{}) != nil {
				return ctx
			}
			return context.WithValue(ctx, loadTestKey{}, true)
		},
		Bound: func(ctx context.Context) bool {
			return ctx != nil && ctx.Value(loadTestKey{}) != nil
		},
	}
}

// DefaultPropagator is go-spring's [Propagator]. It answers every seam from
// one [Binding], held by value.
type DefaultPropagator struct {
	binding Binding
}

// NewDefaultPropagator returns a propagator over binding. Fill the binding
// completely — a re-based convention is [DefaultBinding] with the fields that
// differ overwritten. The error is always nil; the return keeps room for a
// future check without breaking callers.
func NewDefaultPropagator(binding Binding) (DefaultPropagator, error) {
	return DefaultPropagator{binding: binding}, nil
}

func (p DefaultPropagator) IsLoadTest(ctx context.Context) bool {
	return p.binding.Bound(ctx)
}

func (p DefaultPropagator) WithLoadTest(ctx context.Context) context.Context {
	return p.binding.Bind(ctx)
}

// Extract scans the carrier's keys for the marker's name, case-insensitively —
// an HTTP header map holds the textproto-canonicalised spelling of the key, and
// every other hop holds it as written. A key's values tag the context when one
// is the binding's wire value, also compared case-insensitively. A carrier
// holds a handful of keys, so the scan is linear and cheap.
func (p DefaultPropagator) Extract(ctx context.Context, c propagate.Carrier) context.Context {
	if c == nil {
		return ctx
	}
	for _, k := range c.Keys() {
		if !strings.EqualFold(k, p.binding.Key) {
			continue
		}
		for _, v := range c.Values(k) {
			if strings.EqualFold(v, p.binding.Value) {
				// Tagged: no later value can add anything, so stop scanning.
				return p.WithLoadTest(ctx)
			}
		}
	}
	return ctx
}

// Inject writes the binding's key as spelled; [propagate.Carrier.Set] replaces,
// so re-injecting is idempotent. A nil carrier drops the write.
func (p DefaultPropagator) Inject(ctx context.Context, c propagate.Carrier) {
	if c == nil || !p.IsLoadTest(ctx) {
		return
	}
	c.Set(p.binding.Key, p.binding.Value)
}

// Propagate tags child as load-test traffic when parent is load-test traffic,
// and returns child unchanged otherwise. It carries the flag across a context
// boundary — a handler fanning work out to a goroutine, or building a fresh
// background context — so it survives the hop. A pure function over the
// propagator's own two slot methods, not an interface method: no
// implementation state is involved.
func Propagate(p Propagator, parent, child context.Context) context.Context {
	if !p.IsLoadTest(parent) {
		return child
	}
	return p.WithLoadTest(child)
}
