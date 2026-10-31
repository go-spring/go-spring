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

// Package traffic answers one question for the whole process — "who sent this
// request, and what kind of traffic is it?" — and carries go-spring's way of
// answering it: a label in the [context.Context], ferried across HTTP headers,
// gRPC metadata and message properties, so every hop in a call chain sees the
// same classification.
//
// The vocabulary is open. go-spring names the three kinds a chain commonly
// needs to distinguish — [KindLoadTest], [KindMirror], [KindTest] — plus the
// absence that means real traffic ([KindReal], the zero value). The label on the
// wire is the kind itself ("x-traffic: loadtest"); a process whose gateway
// already stamps a classification of its own ("stress", "shadow", "gray")
// writes that label into the same marker and reads it back as the same [Kind].
// Nothing here enumerates what a process may send. A kind is a lower-case
// token: the default implementation folds case in both directions, so
// "LoadTest" and "loadtest" are the same kind. Only case is normalized;
// whitespace is part of the label.
//
// The convention itself is a [Binding]: where the label lives in the context and
// the name the marker rides under on the wire. [DefaultBinding] is go-spring's,
// so an application whose gateway stamps traffic with a different header takes
// that binding, fills in what differs, and hands it to [NewDefaultPropagator] as
// its own bean. A process that must also REWRITE labels — a legacy gateway
// writes "pts" for what this process calls "loadtest" — supplies its own
// [Propagator]; a mapping is a rule, not a knob on [Binding].
//
// One process has one propagator bean, and a component that injects [Propagator]
// gets go-spring's [DefaultBinding] unless the application registers its own bean
// first; a standalone caller outside the container builds one with
// [NewDefaultPropagator]. Business code reads the answer from that instance
// rather than from a global, never constructs the marker itself, and does not
// care how the label got there (a header, a metadata key, a message property).
//
// Layering. Everything else in the cloud stack (observe, resilience, fault) may
// consume the label; this package consumes nothing from them. Outside the
// standard library it depends only on [go-spring.org/cloud/propagate], the
// generic carrier the protocol seams are written against, and on spring/gs solely
// for the default bean registration, so any starter can import it without
// pulling a heavier graph.
//
// Consumption is opt-in. Marking a context and propagating the label costs
// almost nothing; deciding what to DO with a classified request (mirror tables,
// isolated breakers, metric tags) is intentionally NOT built in. A propagator
// only carries the label; it never acts on it.
package traffic

import (
	"context"
	"strings"

	"go-spring.org/cloud/propagate"
)

// Kind is one process's classification of a request. [KindReal] (the zero
// value) means "no label", and the outbound seams write nothing for it. Every
// other kind is an open, lower-case label: the constants below are go-spring's
// names, and a process is free to use its own ("stress", "shadow", "gray").
//
// A kind answers ONE question — "who sent this request here, and why?" — and
// is single-valued because a request arrives from exactly one immediate sender.
// The senders go-spring names are disjoint roles:
//
//	KindReal     an end user (or a service acting for one)
//	KindLoadTest a capacity / load-test generator
//	KindMirror   a mirroring proxy delivering a copy of another request
//	KindTest     a functional-test / sandbox harness
//
// Two classifications are deliberately NOT kinds:
//
//   - canary / gray. The sender is still the user; the tag only says which
//     version the gateway routed the request to. Lanes and versions are routing
//     facts (labels, discovery metadata), not traffic sources.
//   - health / liveness probes. They terminate at the endpoint they probe, so
//     there is no further hop for a marker to ride and no downstream state to
//     isolate; an entry layer excludes them by route. Synthetic monitoring that
//     exercises a real business flow does traverse the chain and is [KindTest].
//
// A mirrored copy of load-test traffic is [KindMirror], not [KindLoadTest]: the
// immediate sender is the mirroring proxy, and mirror handling (discard the
// response, forbid side effects) wins.
//
// The type is a string so a kind stays a plain label — compared, logged and
// written as one — not a closed enumeration the framework has to grow. The
// constants are therefore conventions, not a whitelist: [Propagator.Extract]
// accepts any non-empty label.
type Kind string

const (
	// KindReal is real (production) traffic. It is the zero value, so an
	// unmarked context reads as real traffic and real traffic never writes a
	// marker. [Propagator.WithKind] does not tag for this kind: marking is
	// monotonic, and the way to say "real" is to never mark it.
	KindReal Kind = ""

	// KindLoadTest is traffic from a capacity / load-test generator
	// (cloud/experimental/loadtest is the runner that produces it). A gateway
	// that spells it "stress" or "pressure" (X-Pressure-Test, Cpts-X-Test) is
	// using a label of its own under the open vocabulary.
	KindLoadTest Kind = "loadtest"

	// KindMirror is a copy delivered by a mirroring proxy — Istio/Envoy request
	// mirroring, or record-and-replay. Its sender is the proxy, not the user
	// whose request was copied, and its response is discarded. Consumers must
	// keep it side-effect free (writes go to mirror tables, keys and topics)
	// and out of business metrics.
	KindMirror Kind = "mirror"

	// KindTest is traffic from a functional-test / sandbox harness, including
	// synthetic monitoring that exercises a real business flow (AWS ECS
	// test-traffic headers, Signadot sandboxes, online integration testing).
	// Its response matters — the harness checks it — but its data must stay
	// isolated from production. A bare health probe is not this kind: it
	// terminates at the probe endpoint and never reaches a business hop.
	KindTest Kind = "test"
)

// Label names k as a non-empty value. [KindReal] is the ABSENCE of a marker on
// the wire and has no string spelling of its own, so as a value it is written
// "real" — the word a consumer puts into a log field or a metadata attribute.
// Every other kind is already its own lower-case label.
func (k Kind) Label() string {
	if k == KindReal {
		return "real"
	}
	return string(k)
}

// Propagator is one process's traffic convention at work: the seams that move
// the label across a protocol hop, all of them answered from its [Binding].
// [DefaultPropagator] implements it, and re-basing the convention means
// building one over a different binding — not overriding a method. A rule beyond
// carrying the label (a legacy gateway that writes "pts" and "stress" for the
// same thing, or a process that wants two labels to collapse into one kind) is a
// custom implementation of this interface, not a knob on [Binding].
//
// The wire vocabulary is not part of this interface: a seam is handed the hop's
// own metadata as a [propagate.Carrier] — an adapter every protocol's metadata
// already satisfies or wraps — so an adapter moves the marker without ever
// asking what it is called.
//
// An inbound seam tags the context through [Propagator.WithKind], and an
// outbound seam is a no-op for [KindReal]. Contexts are non-nil, per the
// [context.Context] contract; every seam tolerates a nil carrier. [Propagate] —
// the context-to-context hop — is a plain function over the interface, not a
// method of it.
type Propagator interface {
	// Kind reports the ctx's traffic classification, [KindReal] when it
	// carries none.
	Kind(ctx context.Context) Kind

	// WithKind returns a copy of ctx tagged as kind, or ctx itself when it is
	// already tagged with that kind. A context carrying a DIFFERENT kind is
	// re-tagged: the caller is making an explicit statement, which is how a
	// handler overwrites an inherited classification. [KindReal] leaves the
	// context unmarked — real traffic is the absence of a label, and marking is
	// monotonic. The argument is folded to a lower-case token.
	WithKind(ctx context.Context, kind Kind) context.Context

	// Extract tags ctx when the carrier holds a non-empty value under the
	// marker's name. A key matches the binding's key case-insensitively —
	// [net/http] canonicalises header keys on insert ("x-traffic" is stored
	// as "X-Traffic") — so one call serves an HTTP header map, a gRPC
	// metadata.MD and a broker's message properties alike. Pass the hop's own
	// metadata as a carrier, e.g. propagate.Header(req.Header).
	//
	// The first non-empty value is the kind, folded to lower case. Only case is
	// normalized — surrounding whitespace stays part of the label. A context
	// that already carries a kind keeps it: the label that classified the
	// request wins over one a later hop might overwrite.
	Extract(ctx context.Context, c propagate.Carrier) context.Context

	// Inject writes the marker onto the carrier when ctx carries a kind and
	// does nothing for [KindReal]. The carrier's Set decides where the write
	// lands: a map adapter writes through to the protocol's own map, a
	// getter/setter adapter (a Kafka record's header slice) writes through
	// its setter.
	Inject(ctx context.Context, c propagate.Carrier)
}

// Binding is one process's traffic convention in data form: the name the marker
// rides under and the context slot the label lives in. The name is one string
// for every hop — HTTP, gRPC, the brokers — because the only cross-protocol
// variance a chain tolerates is case, and the seams absorb that. This is the one
// place a convention is re-based: Key renames the marker, Kind and WithKind
// replace the slot. go-spring's own slot ([DefaultBinding]) is used unless an
// application supplies its own.
//
// The wire value needs no field: it is the kind itself. A process whose wire
// spelling differs from its kind supplies its own [Propagator].
//
// Every field is required: an unset Key silently matches nothing, and an unset
// WithKind or Kind breaks the slot outright, so fill the binding completely — a
// re-based convention is [DefaultBinding] with the fields that differ
// overwritten.
type Binding struct {
	// Key is the name the marker rides under on every hop. It is matched
	// case-insensitively, so net/http's canonicalised spelling of an HTTP
	// header is the same name.
	Key string

	// Kind reports the kind ctx carries, [KindReal] when it carries none.
	Kind func(ctx context.Context) Kind

	// WithKind returns a copy of ctx carrying kind. The propagator never passes
	// [KindReal], and it returns ctx itself when the kind is already there, so
	// the slot may store unconditionally.
	WithKind func(ctx context.Context, kind Kind) context.Context
}

// kindKey is the context key go-spring's slot — [DefaultBinding]'s
// WithKind and Kind — stores the label under. It is an unexported struct type,
// the usual context-key discipline so no other package can collide with it, and
// its value is the [Kind].
type kindKey struct{}

// DefaultBinding returns go-spring's [Binding]: the label is the [Kind] stored
// under [kindKey], and the marker is named "x-traffic". Copy it when re-basing
// one part of the convention.
func DefaultBinding() Binding {
	return Binding{
		Key: "x-traffic",
		Kind: func(ctx context.Context) Kind {
			kind, _ := ctx.Value(kindKey{}).(Kind)
			return kind
		},
		WithKind: func(ctx context.Context, kind Kind) context.Context {
			return context.WithValue(ctx, kindKey{}, kind)
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
// differ overwritten.
func NewDefaultPropagator(binding Binding) DefaultPropagator {
	return DefaultPropagator{binding: binding}
}

// normalize folds a kind to lower case, the one spelling this package stores on
// a context and writes on the wire. It is applied at both writing seams
// ([Propagator.WithKind] and [Propagator.Extract]) so a kind bound by hand and a
// kind read off a header compare equal.
func normalize(kind Kind) Kind {
	return Kind(strings.ToLower(string(kind)))
}

func (p DefaultPropagator) Kind(ctx context.Context) Kind {
	return p.binding.Kind(ctx)
}

// WithKind owns the real-kind and idempotence checks so the binding slot can
// store unconditionally: the kind is folded, a real kind does not tag, a context
// already carrying kind is returned as-is, and a different kind re-tags.
func (p DefaultPropagator) WithKind(ctx context.Context, kind Kind) context.Context {
	kind = normalize(kind)
	if kind == KindReal {
		return ctx
	}
	if p.Kind(ctx) == kind {
		return ctx
	}
	return p.binding.WithKind(ctx, kind)
}

// Extract scans the carrier's keys for the marker's name, case-insensitively —
// an HTTP header map holds the textproto-canonicalised spelling of the key, and
// every other hop holds it as written. The first non-empty value under it is
// the kind; a context that already carries one is left alone, so the label
// that reached this hop first is the one that sticks. A carrier holds a handful
// of keys, so the scan is linear and cheap.
func (p DefaultPropagator) Extract(ctx context.Context, c propagate.Carrier) context.Context {
	if c == nil || p.Kind(ctx) != KindReal {
		return ctx
	}
	for _, k := range c.Keys() {
		if !strings.EqualFold(k, p.binding.Key) {
			continue
		}
		for _, v := range c.Values(k) {
			if kind := normalize(Kind(v)); kind != KindReal {
				return p.WithKind(ctx, kind)
			}
		}
	}
	return ctx
}

// Inject writes the binding's key as spelled; [propagate.Carrier.Set] replaces,
// so re-injecting is idempotent. A nil carrier and [KindReal] drop the write.
func (p DefaultPropagator) Inject(ctx context.Context, c propagate.Carrier) {
	if c == nil {
		return
	}
	kind := p.Kind(ctx)
	if kind == KindReal {
		return
	}
	c.Set(p.binding.Key, string(kind))
}

// Propagate tags child as the parent's kind when the parent carries one, and
// returns child unchanged otherwise. It carries the label across a context
// boundary — a handler fanning work out to a goroutine, or building a fresh
// background context — so it survives the hop. A pure function over the
// propagator's own slot methods, not an interface method: no implementation
// state is involved.
//
// parent comes first because it is the call's own context; child is the
// context being derived from it.
func Propagate(parent context.Context, p Propagator, child context.Context) context.Context {
	kind := p.Kind(parent)
	if kind == KindReal {
		return child
	}
	return p.WithKind(child, kind)
}

// IsLoadTest reports whether ctx is load-test traffic under p — shorthand for
// p.Kind(ctx) == [KindLoadTest], the one question most callers ask. It is a
// function rather than a [Propagator] method so the interface carries only the
// seams an implementation must answer; asking the question is a reading of
// [Propagator.Kind], not another seam.
func IsLoadTest(ctx context.Context, p Propagator) bool {
	return p.Kind(ctx) == KindLoadTest
}
