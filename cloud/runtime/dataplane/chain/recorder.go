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

	"go-spring.org/log"
	"go.opentelemetry.io/otel/attribute"
)

// Injection records a fault the chain set on a call. It is written by the fault
// layer and read by every other — observability labels an injected call so a
// fire is never mistaken for a real failure, and tests read it back to assert a
// rule fired. Without it an injected fault is indistinguishable from a genuine
// downstream failure, which is exactly the gap it closes.
//
// It is a write-back type: it is set on the [Recorder] through
// [Recorder.SetFault] as the call runs. Nothing declares it up front, because
// whether a rule fires is not known until the fault layer has matched the call
// and decided to fire it.
type Injection struct {
	// Kind is the injected failure kind ("generic", "timeout", "reset",
	// "refused"), empty when only latency was injected.
	Kind string

	// Rule identifies the fault rule the fire came from — the matched rule's
	// service (or the call's own service when the rule is a catch-all) plus its
	// index in the config, or the service alone when the direction's top-level
	// knobs governed. It mirrors the governance side's rule identity: it makes
	// "which rule set this call on fire" answerable from the call's own record,
	// including the pure-latency case where Kind is empty.
	Rule string

	// Delay is the pause the fault layer slept before the call — the configured
	// latency after jitter, as actually applied — zero when none was. Deliberately
	// not "Latency": that word belongs to what a call MEASURED (see
	// [Round.Duration] and the span's own timing); this is a delay INJECTED into
	// it.
	Delay time.Duration
}

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
	// "retry_budget_exceeded" (an admission gate refused the round before the
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

	// Err is the round's own error, nil on success. It is per-round: a
	// retried-then-succeeded call keeps the failed round's error here while
	// Execute returns nil.
	Err error
}

// Observed is the observability block the chain FILLS: what the call LEARNED,
// per signal group — the counterpart of [Observe], which is what the caller
// DECLARES. The two are deliberately different types rather than one: the
// declared block carries configuration that is settled before the call runs (the
// log tag, the quiet-success level, the span kind and links, the metric family
// name) and the produced block carries none of it, because none of it can still
// be chosen once the call is under way. What they share is only the SHAPE —
// three groups, addressed by signal type — so a value meant for the span is
// written to Span on either side and never has to be translated.
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
// CARDINALITY: [Observed.Metric] is bounded the same hard way [Metric.Attrs] is —
// the values must come from a small, closed set. This side is the easier one to
// get wrong, because a metric label is the natural place to want a key, a
// statement or a path, and the write-back happens far from where the family was
// declared. An open-set value belongs in [Observed.Span] or [Observed.Log].
type Observed struct {
	// Log holds access-log fields appended after the declared [Log.Fields]: the
	// RESPONSE half of the line — what the fn or the client's own executor found —
	// as opposed to the declared request half. Per CALL, not per round: the access
	// line is one line, so a field that differs per attempt has to be summarised
	// rather than appended once per attempt.
	Log []log.Field

	// Span holds attributes appended after the declared [Span.Attrs]. Unlike the
	// declared half, which is set when the span STARTS, these are set when the call
	// ends — which is the whole point: they are the values only the call could
	// produce.
	Span []attribute.KeyValue

	// Metric holds labels appended after the declared [Metric.Attrs], on the
	// CALL-level duration only: an attempt-level sample describes one round, and a
	// call-level supplement does not belong on it. See the cardinality rule above.
	Metric []attribute.KeyValue
}

// AddLog appends access-log fields to the call's produced observability block. A
// nil Recorder is a no-op, so a client's own executor writes unconditionally.
func (r *Recorder) AddLog(fields ...log.Field) {
	if r == nil {
		return
	}
	r.Observed.Log = append(r.Observed.Log, fields...)
}

// AddSpan appends span attributes to the call's produced observability block, set
// on the span when the call ends. A nil Recorder is a no-op.
func (r *Recorder) AddSpan(attrs ...attribute.KeyValue) {
	if r == nil {
		return
	}
	r.Observed.Span = append(r.Observed.Span, attrs...)
}

// AddMetric appends metric labels to the call's produced observability block,
// carried by the call-level duration. The values must be bounded — see
// [Observed]. A nil Recorder is a no-op.
func (r *Recorder) AddMetric(attrs ...attribute.KeyValue) {
	if r == nil {
		return
	}
	r.Observed.Metric = append(r.Observed.Metric, attrs...)
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
// shared one) and is only valid to READ after Execute returns. A nil Recorder is
// valid and means "record nothing" — every method is nil-safe, so a caller that
// does not want a record passes nil and a layer records unconditionally.
type Recorder struct {
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
	// Like [Injection] it is a write-back fact: a center may push the bound while
	// the call is in flight, so only the running chain knows which value applied.
	Timeout time.Duration

	// Observed is the observability block the chain FILLS — the counterpart of the
	// one the caller declares ([Operation.Observe]) and the only way a signal group
	// grows after the call starts. It is read by the emitter, which appends each
	// group after the declared one; a writer appends through [Recorder.AddLog],
	// [Recorder.AddSpan] and [Recorder.AddMetric]. See [Observed].
	Observed Observed

	// Fault is the fire the fault layer set on this call, nil when none was. It is
	// CALL-level, not per-round: a fault gate runs inside the retry loop, and the
	// recorder is the one thing every attempt shares, so the last fire wins —
	// which is the same answer a reader of the rounds would derive (the most
	// recent round that fired). See [Injection].
	Fault *Injection

	// rounds holds the call's rounds, oldest first. len() is the count; empty means
	// the call never entered the chain (a dead context on entry, or a call that
	// was never attempted at all) — NOT that it was rejected, which records a
	// round like everything else.
	rounds []Round
}

// SetFault records the fire the fault layer set on this call. A nil Recorder is a
// no-op, so a layer records unconditionally. The last fire wins: a retry that
// succeeds after a firing attempt leaves the first attempt's fire on the record,
// because that is what the call was subjected to.
func (r *Recorder) SetFault(f *Injection) {
	if r == nil {
		return
	}
	r.Fault = f
}

// AddRound appends one sealed round to the record. The caller supplies the facts
// only it can measure — when the round ran, how long it took, how it ended and
// its error (see [Round]). A nil Recorder is a no-op, so recording is always
// safe to attempt.
//
// It is called by the one place that knows a round ended — the governance retry
// loop, or the emitter for a call that ran with no governance layer.
func (r *Recorder) AddRound(a Round) {
	if r == nil {
		return
	}
	r.rounds = append(r.rounds, a)
}

// Rounds returns the rounds recorded so far, oldest first, or nil when the call
// made none. The result is owned by the Recorder and must not be modified.
func (r *Recorder) Rounds() []Round {
	if r == nil {
		return nil
	}
	return r.rounds
}

// LastRound returns the most recent round, zero when the call made none. It is
// the round the call's own status, peer and error derive from, so the derivation
// lives here rather than being spelled out again at every reader.
func (r *Recorder) LastRound() (Round, bool) {
	if r == nil || len(r.rounds) == 0 {
		return Round{}, false
	}
	return r.rounds[len(r.rounds)-1], true
}

// Reset clears the record so the same Recorder can serve another call — the
// supported way to pool them. It keeps nothing: a fresh call starts from zero.
func (r *Recorder) Reset() {
	if r == nil {
		return
	}
	r.Timeout = 0
	// The produced block is cleared the way the rounds are: a pooled Recorder must
	// not carry one call's business fields onto the next — nor its fire.
	r.Observed = Observed{}
	r.Fault = nil
	r.rounds = r.rounds[:0]
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
// "bulkhead_full", "retry_budget_exceeded", "timeout" — or "" for an ordinary
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
	case errors.Is(err, ErrRetryBudgetExceeded):
		return "retry_budget_exceeded"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	default:
		return ""
	}
}
