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

// round.go is the ROUND itself — the per-attempt unit a governed call runs and
// records in — and everything that travels with it: the peer the round runs
// against (addressing is part of the ROUND, not something a starter's dial
// hook does once per connection — a retry re-picks, a failed peer is excluded
// from the next pick, and the round's outcome settles the peer exactly once,
// here) and the context staging that carries the round to a driver deep below
// the fn (see [WithRound]).

package chain

import (
	"context"
	"time"

	"go-spring.org/cloud/discovery"
	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// Peer is the endpoint one round runs against. Addr is the address to dial or
// route to; an empty Addr means the call has a FIXED target the caller owns
// (no discovery, a mesh sidecar, or a driver that routes by itself), in which
// case the peer takes no part in addressing at all. The zero Peer is exactly
// that: a fixed peer that completes nothing.
//
// Complete — the write-back the peer's owner (the balancer) asked for —
// travels INSIDE the value, bound to the generation that made the pick. The
// chain calls [Peer.Complete] exactly once per round, after the round's error
// is known; a fixed peer completes nothing. Starters never complete: one
// round, one complete, one owner.
type Peer struct {
	// Endpoint is the discovery value the balancer picked, embedded so the
	// round's peer reads directly as the endpoint it is: peer.Addr, peer.Scheme.
	// A zero Endpoint means a FIXED target the caller owns — no discovery, a mesh
	// sidecar, or a driver that routes by itself. The fields are the pick-time
	// snapshot the balancer already filtered on, not live health.
	discovery.Endpoint

	// complete writes the round's outcome back to the endpoint's health and
	// the balancer's tracker. nil means this peer is fixed and completes
	// nothing.
	complete func(ctx context.Context, err error)
}

// NewPeer builds a picked peer: ep is the chosen endpoint and complete is
// the write-back its owner asked for, called exactly once by [Peer.Complete]
// after the round's outcome is known. It exists for the addressing layer — the
// complete closure is the balancer's to bind (to the generation that made the
// pick), which is why the field itself stays unexported while the constructor
// is not.
func NewPeer(ep discovery.Endpoint, complete func(ctx context.Context, err error)) Peer {
	return Peer{Endpoint: ep, complete: complete}
}

// Complete reports the round's outcome (err nil on success) to the endpoint's
// health and the balancer's tracker. It is the chain's to call, once per
// round; on a fixed peer it is a no-op. The classification is [Classify]'s —
// the caller hands over the round's plain error.
func (p Peer) Complete(ctx context.Context, err error) {
	if p.complete != nil {
		p.complete(ctx, err)
	}
}

// Round is one round of a call against the downstream — the initial try or one
// retry — as recorded by [Recorder]. It is the per-round detail a whole-call
// view cannot express: a retried call's rounds may differ in duration, outcome,
// peer, fire and produced signals — the balancer (backed by discovery)
// re-resolves between rounds, so a retry may land on another instance than the
// first try, and "which node is actually failing" is answerable only per round.
//
// A round is recorded for every DECISION the chain made, including one that
// never reached the downstream: a call refused by a gate is a round whose
// ErrorType names the gate (see [Recorder]).
//
// WHO WRITES: the retry loop CREATES the round ([Recorder.NewRound]) before the
// attempt runs and SEALS it — the measured facts, Start/Duration/Status/
// ErrorType/Error — when the attempt returns. Everything between the open and
// the seal is written by the layer that owns it, into the round it is handed:
// the addressing stage stamps the peer, the fault layer its fire, the fn its
// produced signals. A round that never runs — a gate refusal — is appended
// created and sealed in place — see [Round].
type Round struct {
	// Start is when the round began — the moment it really began, after any
	// backoff or admission wait. Its end is Start + [Round.Duration], derived
	// rather than stored: the sleep between two rounds is the gap between one
	// round's end and the next round's start, so a separate backoff field would
	// be a second copy of the same fact (and one that could drift).
	Start time.Time

	// Duration is the time this round itself took — including any admission wait,
	// excluding nothing: it is what the caller OBSERVED for the round, not a claim
	// about what the downstream spent (that is not observable from here).
	Duration time.Duration

	// Status is the round's outcome on the shared axis ("ok" / "error"). The
	// call's own status derives from the last round's.
	Status string

	// ErrorType names the class of this round's outcome, on [RoundErrorType]'s
	// vocabulary: [ErrorTypeNone] on success; "timeout" (the round's own budget
	// fired); "rate_limited" / "circuit_open" / "bulkhead_full" /
	// "retry_exceeded" / "no_endpoints" (a gate refused the round before the
	// downstream was touched); "connect" (the dial was marked as failed);
	// "resource_exhausted" (one of OUR resources refused before sending);
	// "canceled" (the caller walked away); "unknown" (no class the chain can
	// prove — the protocol's own attribute carries the detail). It is total:
	// every round has exactly one word.
	//
	// The name follows OTel's semantic convention for "the class of error the
	// operation ended with" (`error.type`), because that is the question it
	// answers; the VALUES are this package's own vocabulary, which is what makes
	// "was this failure ours or theirs" answerable from the round alone — a
	// question `status` cannot answer, since it says only ok/error.
	//
	// The gate words double as the answer to "was it ever sent": a round whose
	// ErrorType is one of them never reached the downstream, while a timeout,
	// connect, canceled or unknown round did. Per-round by design: a gate can
	// refuse one round and let the next through, and a timeout that fired on the
	// first try says nothing about the second.
	ErrorType ErrorType

	// Error is the round's own error, nil on success. It is per-round: a
	// retried-then-succeeded call keeps the failed round's error here while
	// Execute returns nil.
	Error error

	// Peer is the endpoint this round ran against — the handoff the addressing
	// stage staged, WITH its complete closure, which [chain.Next]'s terminal
	// passes to the fn. [Peer.Addr] is the flat fact a reader queries; empty
	// means the round never reached a peer decision: the call has no addressing
	// (a fixed target the caller owns), a gate refused it ([Round.ErrorType]
	// names the gate), or addressing found no endpoint ([ErrNoEndpoints]).
	Peer Peer

	// Timeout is the bound this round's call ran under — the resolved value
	// after a hot-fix override, zero when it ran under none of its own. It is
	// written when the round is created, from the plan the WHOLE call shares (one
	// context deadline over every round and the backoff between them), so every
	// round of a call carries the same value; the call-level view is the last
	// round's.
	//
	// It is what the call was GIVEN, not what it took: the time actually spent
	// is [Round.Duration]. Reading the two side by side is what answers "was
	// this call's timeout too tight, or was the downstream just slow".
	//
	// It is a write-back fact: a center may push the bound while the call is in
	// flight, so only the running chain knows which value applied.
	Timeout time.Duration

	// ConnectTimeout is the connection budget this round's ESTABLISHMENT — the
	// dial, and any pool wait that precedes it — was allowed to spend, written
	// when the round is created, from the same plan [Round.Timeout] comes from. A
	// dial site beneath the fn reads it (see [RoundFrom]) and applies
	// it as min(this, its own static cap, the round's remaining deadline) —
	// the budget only tightens, never widens. Zero means no budget of its own.
	ConnectTimeout time.Duration

	// MaxRetries is how many EXTRA attempts the call's plan allowed after the
	// first — the configured ceiling, not what the call used. What it used is
	// derived from the record instead (the rounds beyond the first), because the
	// count is already there and a second stored copy could drift.
	//
	// It is written when the round is created, from the plan the WHOLE call shares, so
	// every round of a call carries the same value; the call-level view is the
	// last round's. Zero means the call was allowed its first try only — either
	// the plan configured no retry, or the call ran no governance plan at all.
	// Reading it beside the round count is what answers "did this call exhaust
	// its retry budget, or did it simply never have one".
	MaxRetries int

	// Observed is the observability block this round produced — the counterpart
	// of the one the caller declares ([Operation.Observe]). Written by the fn
	// or the client's own executor through [Round.AddLogFields],
	// [Round.AddSpanAttrs] and [Round.AddMetricAttrs], which address the
	// LAST round. See [Observed].
	Observed Observed

	// Fault is the fire the fault layer set on THIS round, nil when none was.
	// The fault gate runs inside the retry loop, so every attempt is judged on
	// its own; the call-level view is the most recent round that fired.
	Fault *kfault.Result
}

// SetError settles the round's error side: err itself plus the two words
// derived from it — the status axis ([ClassifyStatus]) and, when the chain is
// what failed the round, the gate word ([RoundErrorType]). Writing them is
// ONE transition, not a triple every sealing layer must remember to keep
// whole. The round's value side is not this method's business.
func (rd *Round) SetError(err error) {
	rd.Error = err
	rd.Status = ClassifyStatus(err)
	rd.ErrorType = RoundErrorType(err)
}

// Observed is the observability block the chain FILLS: what the call LEARNED,
// per signal group — the counterpart of [Observe], which is what the caller
// DECLARES. The two are deliberately different types rather than one: the
// declared block carries configuration that is settled before the call runs (the
// log tag, the quiet-success level, the span kind and links, the metric family
// name) and the produced block carries none of it, because none of it can still
// be chosen once the call is under way. What they share is only the SHAPE —
// three groups, addressed by signal type — so a value meant for the span is
// written to the span group on either side and never has to be translated.
//
// It exists because a signal group cannot be closed at declaration time. The
// caller states what it knows up front; the fn knows what the response carried,
// and an executor a business wrote for its own protocol knows its own semantics —
// "cache hit or miss", "rows scanned", "which shard answered". Those are learned,
// not declared, so they need a write-back home. The home is PER ROUND: each round
// keeps its own block isolated, and the emitter derives the call-level view — the
// signals themselves are per call (one access line, one span, one metric
// family) — by concatenating the rounds' blocks in order.
//
// MERGE RULE: the emitter appends each round's groups AFTER the declared
// counterpart, in round order, so where a key appears on both sides the produced
// value — and among rounds, the LATER round's — is the one a reader sees last.
// Nothing is deduplicated: a writer that wants to correct a declared value
// appends its own, and one that wants only to add does the same.
//
// CARDINALITY: [Observed.MetricAttrs] is bounded the same hard way [Metric.Attrs] is —
// the values must come from a small, closed set. This side is the easier one to
// get wrong, because a metric label is the natural place to want a key, a
// statement or a path, and the write-back happens far from where the family was
// declared. An open-set value belongs in [Observed.SpanAttrs] or [Observed.LogFields].
type Observed struct {
	// LogFields holds access-log fields appended after the declared [Log.Fields]:
	// the RESPONSE half of the line — what the fn or the client's own executor
	// found — as opposed to the declared request half. Per round: the access line
	// is one line, so the emitter concatenates the rounds' fields, and a writer
	// whose key differs per attempt still wants each round to write its own.
	LogFields []log.Field

	// SpanAttrs holds attributes appended after the declared [Span.Attrs]. Unlike
	// the declared half, which is set when the span STARTS, these are set when
	// the call ends — which is the whole point: they are the values only the call
	// could produce.
	SpanAttrs []attribute.KeyValue

	// MetricAttrs holds labels appended after the declared [Metric.Attrs], on the
	// CALL-level duration only: an attempt-level sample describes one round, and
	// a call-level supplement does not belong on it. See the cardinality rule
	// above.
	MetricAttrs []attribute.KeyValue
}

// AddLogFields appends access-log fields to the round's produced observability
// block. The round owns its block; the appends are its own methods.
func (rd *Round) AddLogFields(fields ...log.Field) {
	rd.Observed.LogFields = append(rd.Observed.LogFields, fields...)
}

// AddSpanAttrs appends span attributes to the round's produced observability
// block, set on the span when the call ends.
func (rd *Round) AddSpanAttrs(attrs ...attribute.KeyValue) {
	rd.Observed.SpanAttrs = append(rd.Observed.SpanAttrs, attrs...)
}

// AddMetricAttrs appends metric labels to the round's produced observability
// block, carried by the call-level duration. The values must be bounded — see
// [Observed].
func (rd *Round) AddMetricAttrs(attrs ...attribute.KeyValue) {
	rd.Observed.MetricAttrs = append(rd.Observed.MetricAttrs, attrs...)
}

// roundKey is the context key the chain stages the running round under, so a
// driver that dials deep inside its own call stack — behind a vendor pool the
// chain cannot see through — can still read everything the round carries: the
// peer addressing picked, the connection budget the plan allowed.
type roundKey struct{}

// The produced-block writers are the [Round] methods above: a writer that
// holds the recorder navigates first — rec.LastRound().AddSpanAttrs(...) —
// because the round, not the recorder, owns its block. With no round open
// there is nothing to write to; only the loop opens rounds.

// WithRound returns a context carrying the round the call is currently in. The
// executor stages it when it creates the round, before handing control down; the
// value is the LIVE round — layers beneath the staging point see the fields
// later stages stamp into it.
func WithRound(ctx context.Context, rd *Round) context.Context {
	return context.WithValue(ctx, roundKey{}, rd)
}

// RoundFrom reads the round [WithRound] staged, reporting whether one was
// staged at all.
func RoundFrom(ctx context.Context) (*Round, bool) {
	rd, ok := ctx.Value(roundKey{}).(*Round)
	return rd, ok
}
