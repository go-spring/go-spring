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

// recorder.go is the WRITE-BACK half of a call: what the chain LEARNED while the
// call ran, as opposed to the declaration it was handed ([Operation], in
// operation.go). [Recorder] holds it all — the rounds themselves, the wall-time
// bound the call ran under, the observability block it produced ([Observed]) and
// the fire the fault layer set on it. It is written through a pointer as the call
// proceeds and is only valid to read once [Executor.Execute] returns.

package chain

import (
	"context"
	"errors"
	"time"

	kfault "go-spring.org/cloud/governance/capability/fault"
	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// Round is one round of a call against the downstream — the initial try or one
// retry — as recorded by [Recorder]. It is the per-round detail a whole-call
// view cannot express: a retried call's rounds may differ in duration, outcome
// AND peer — the balancer (backed by discovery) re-resolves between rounds, so a
// retry may land on another instance than the first try, and "which node is
// actually failing" is answerable only per round.
//
// A round is recorded for every DECISION the chain made, including one that
// never reached the downstream: a call refused by a gate is a round whose
// ErrorType names the gate (see [Recorder]).
//
// A round is filled by one side only: the caller of [Recorder.AddRound], which
// measured it from outside — when it ran, how long it took, how it ended.
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

	// ErrorType names the class of the failure when the CHAIN is what failed this
	// round — the vocabulary is [ClassifyErrorType]'s: "timeout" (the round's own
	// budget fired), "rate_limited" / "circuit_open" / "bulkhead_full" /
	// "retry_exceeded" (an admission gate refused the round before the
	// downstream was touched). Empty means the chain did not decide this round's
	// fate: it simply ran, and its error (if any) came from the downstream.
	//
	// The name follows OTel's semantic convention for "the class of error the
	// operation ended with" (`error.type`), because that is the question it
	// answers; the VALUES are this package's own vocabulary, which is what makes
	// "was this failure ours or theirs" answerable from the round alone — a
	// question `status` cannot answer, since it says only ok/error.
	//
	// A gate's vocabulary doubles as the answer to "was it ever sent": a round
	// whose ErrorType is one of the gate words never reached the downstream,
	// while a timeout round did. Per-round by design: a gate can refuse one round
	// and let the next through, and a timeout that fired on the first try says
	// nothing about the second.
	ErrorType string

	// Error is the round's own error, nil on success. It is per-round: a
	// retried-then-succeeded call keeps the failed round's error here while
	// Execute returns nil.
	Error error
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
// not declared, so they need a write-back home — per call, because the signals
// they feed (one access line, one span, one metric family) are per call — and not
// a second vocabulary either, which is why this is the same three groups the
// caller already uses.
//
// MERGE RULE: the emitter appends each group AFTER its declared counterpart, in
// the order given, so where a key appears on both sides the produced value is the
// one a reader sees last. Nothing is deduplicated: a writer that wants to correct
// a declared value appends its own, and one that wants only to add does the same.
//
// CARDINALITY: [Observed.MetricAttrs] is bounded the same hard way [Metric.Attrs] is —
// the values must come from a small, closed set. This side is the easier one to
// get wrong, because a metric label is the natural place to want a key, a
// statement or a path, and the write-back happens far from where the family was
// declared. An open-set value belongs in [Observed.SpanAttrs] or [Observed.LogFields].
type Observed struct {
	// LogFields holds access-log fields appended after the declared [Log.Fields]:
	// the RESPONSE half of the line — what the fn or the client's own executor
	// found — as opposed to the declared request half. Per CALL, not per round:
	// the access line is one line, so a field that differs per attempt has to be
	// summarised rather than appended once per attempt.
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

// AddLogFields appends access-log fields to the call's produced observability block.
func (r *Recorder) AddLogFields(fields ...log.Field) {
	r.Observed.LogFields = append(r.Observed.LogFields, fields...)
}

// AddSpanAttrs appends span attributes to the call's produced observability block, set
// on the span when the call ends.
func (r *Recorder) AddSpanAttrs(attrs ...attribute.KeyValue) {
	r.Observed.SpanAttrs = append(r.Observed.SpanAttrs, attrs...)
}

// AddMetricAttrs appends metric labels to the call's produced observability block,
// carried by the call-level duration. The values must be bounded — see
// [Observed].
func (r *Recorder) AddMetricAttrs(attrs ...attribute.KeyValue) {
	r.Observed.MetricAttrs = append(r.Observed.MetricAttrs, attrs...)
}

// Recorder is the write-back half of a call: what the chain LEARNED as the call
// ran, as opposed to the declaration ([Operation]), which is immutable and
// travels by value. Splitting them is what makes the declaration shareable — a
// layer that mutates its own copy of the Operation changes nothing anyone else
// sees — and gives every layer one explicit place to write.
//
// It exists because a call's outcome is not known to the caller who builds the
// Operation: only the chain sees what the downstream answered, whether protection
// rejected the call, which rule applied and whether a fault was injected.
// Funnelling all of it through one object — rather than a context value per
// concern — gives every layer one place to write and the caller one place to read.
//
// It is the RESPONSE block of a call, the counterpart of [Operation.Request]: what
// the call came back with, layered the way it happened — the TIMEOUT every round
// ran under (the effective, resolved configuration), the OBSERVABILITY the call
// produced ([Observed]) and the ROUNDS themselves.
//
// SCOPE: a call is NATIVELY MULTI-ROUND — the first try plus however many retries
// ran — so everything that happened DURING a round lives on that round ([Round]),
// and only what every round SHARES lives here. That is the whole of this struct:
// [Recorder.Timeout] is one bound applied to the call and therefore shared by
// every round, [Recorder.Observed] is one block of signals for the one access line
// the call writes, [Recorder.Fault] is the fire the call was subjected to, and the
// rounds are the record.
//
// The call's own error is the one Execute RETURNS and its status is the last
// round's status — readers derive both from the rounds.
//
// OWNERSHIP: one Recorder belongs to exactly ONE call. It is written by the chain
// on the calling goroutine (no lock: a nested call gets its own Recorder, not a
// shared one) and is only valid to READ after Execute returns. It must be
// non-nil: a caller that wants only the signals passes a record it simply never
// reads.
type Recorder struct {

	// Rounds holds the call's rounds, oldest first. len() is the count; empty means
	// the call never entered the chain (a dead context on entry, or a call that
	// was never attempted at all) — NOT that it was rejected, which records a
	// round like everything else. Appended through [Recorder.AddRound]; readers
	// treat it as read-only.
	Rounds []Round

	// Timeout is the bound the governance layer put on the call's WALL TIME —
	// the resolved value after a hot-fix override, zero when the call ran under
	// none of its own. It is the one number the chain applies to every stage at
	// once, as a single context deadline over the whole call, so a call that
	// spent its budget in an early round has none left for the later ones.
	//
	// It is what the call was GIVEN, not what it took: the time actually spent is
	// the rounds' own durations ([Round.Duration]) and what the caller observed.
	// Reading the two side by side is what answers "was this call's timeout too
	// tight, or was the downstream just slow".
	//
	// Like [Recorder.Fault] it is a write-back fact: a center may push the bound while
	// the call is in flight, so only the running chain knows which value applied.
	Timeout time.Duration

	// Observed is the observability block the chain FILLS — the counterpart of the
	// one the caller declares ([Operation.Observe]) and the only way a signal group
	// grows after the call starts. It is read by the emitter, which appends each
	// group after the declared one; a writer appends through [Recorder.AddLogFields],
	// [Recorder.AddSpanAttrs] and [Recorder.AddMetricAttrs]. See [Observed].
	Observed Observed

	// Fault is the fire the fault layer set on this call, nil when none was. It is
	// CALL-level, not per-round: a fault gate runs inside the retry loop, and the
	// recorder is the one thing every attempt shares, so the last fire wins —
	// which is the same answer a reader of the rounds would derive (the most
	// recent round that fired).
	Fault *kfault.Result
}

// SetFault records the fire the fault layer set on this call. The last fire wins: a retry that
// succeeds after a firing attempt leaves the first attempt's fire on the record,
// because that is what the call was subjected to.
func (r *Recorder) SetFault(f *kfault.Result) {
	r.Fault = f
}

// AddRound appends one sealed round to the record. The caller supplies the facts
// only it can measure — when the round ran, how long it took, how it ended and
// its error (see [Round]).
//
// It is called by the one place that knows a round ended — the governance retry
// loop, or the emitter for a call that ran with no governance layer.
func (r *Recorder) AddRound(a Round) {
	r.Rounds = append(r.Rounds, a)
}

// LastRound returns the most recent round, zero when the call made none. It is
// the round the call's own status, peer and error derive from, so the derivation
// lives here rather than being spelled out again at every reader.
func (r *Recorder) LastRound() (Round, bool) {
	if len(r.Rounds) == 0 {
		return Round{}, false
	}
	return r.Rounds[len(r.Rounds)-1], true
}

// Reset clears the record so the same Recorder can serve another call — the
// supported way to pool them. It keeps nothing: a fresh call starts from zero.
func (r *Recorder) Reset() {
	r.Timeout = 0
	// The produced block is cleared the way the rounds are: a pooled Recorder must
	// not carry one call's business fields onto the next — nor its fire.
	r.Observed = Observed{}
	r.Fault = nil
	r.Rounds = r.Rounds[:0]
}

// ClassifyStatus maps a call's return error onto the status axis go-spring uses
// everywhere else — `ok` / `error`, the two words a cross-family query joins on.
// A protected call is not a special case: it reports the same two values as an
// inbound request or a discovery lookup.
func ClassifyStatus(err error) string {
	if err == nil {
		return "ok"
	}
	return "error"
}

// ClassifyErrorType names the class of an error the CHAIN produced, on the
// vocabulary [Round.ErrorType] carries: "rate_limited", "circuit_open",
// "bulkhead_full", "retry_exceeded", "timeout" — or "" for an ordinary
// error that came from the downstream rather than from protection.
//
// Whether the call succeeded is a different question, and [ClassifyStatus]
// answers it: `status` is the shared axis every family reports on, while this is
// the family-specific key that says the failure was OURS. The split is the same
// one as `status` beside `rpc.grpc.status_code`.
//
// It lives here, beside the sentinels whose errors it classifies, so an engine
// and the emitter both read it from the owner of the vocabulary rather than the
// emitter owning a word that is really the governance layer's.
func ClassifyErrorType(err error) string {
	switch {
	case errors.Is(err, ErrRateLimited):
		return "rate_limited"
	case errors.Is(err, ErrCircuitOpen):
		return "circuit_open"
	case errors.Is(err, ErrBulkheadFull):
		return "bulkhead_full"
	case errors.Is(err, ErrRetryExceeded):
		return "retry_exceeded"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return ""
	}
}

// ErrorTypeDownstream names a call that failed at the downstream rather than
// being refused by protection. Unlike [ClassifyErrorType]'s words it is not
// produced by any sentinel: it is the answer left when every gate word is ruled
// out, which is why it exists only at CALL level.
//
// It is the word that makes "was this failure ours or theirs" answerable from
// the final error alone: the gate words say the chain refused the call, this one
// says the call ran and the dependency did not serve it. A business answer the
// adapter declares as normal (see the client seams' Tolerate) never reaches here
// — it ends the call as success, because the dependency did serve it.
const ErrorTypeDownstream = "downstream"

// ErrorTypeCanceled names a call the CALLER abandoned or put a deadline on
// before the chain could finish. It is not a downstream failure: the dependency
// was never given the chance to answer, so counting it against the dependency
// would be wrong.
const ErrorTypeCanceled = "canceled"

// ClassifyOutcome names the FINAL outcome of one call, on a closed vocabulary
// the observability plane reports verbatim: the gate words above plus
// [ErrorTypeDownstream] and [ErrorTypeCanceled]. Empty means success.
//
// It is the CALL-level counterpart of [ClassifyErrorType], and totals where that
// one is partial: a round may simply run and say nothing, but a finished call
// that failed has exactly one reason, so the answer is never "unclassified".
// The distinction matters to a reader: `status=error` with no class cannot tell
// a broken downstream from a caller that walked away, and a failure rate that
// counts both is not a health signal.
//
// ctx is the context the call ran under, used only to tell a deadline the CALLER
// set from the budget the chain derived: in the first case err is ctx.Err() and
// the call was abandoned, in the second ctx is still live and the timeout is the
// chain's own gate.
func ClassifyOutcome(ctx context.Context, err error) string {
	if err == nil {
		return ""
	}
	if ctx != nil && ctx.Err() != nil && errors.Is(err, ctx.Err()) {
		return ErrorTypeCanceled
	}
	if t := ClassifyErrorType(err); t != "" {
		return t
	}
	return ErrorTypeDownstream
}
