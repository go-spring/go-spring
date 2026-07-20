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

// Package tcc defines a framework-agnostic, zero-dependency abstraction for the
// TCC (Try / Confirm / Cancel) distributed-transaction pattern. It is the
// Go-idiomatic equivalent of Seata TCC, reached without replicating Seata's
// TC/TM/RM roles or requiring bytecode/proxy magic.
//
// # Why a separate pattern from Saga
//
// [go-spring.org/spring/transaction] provides Saga: each step's forward Action
// takes real effect immediately, and a later failure undoes the earlier steps by
// running a compensating business function. That gives eventual consistency but
// no isolation — a committed-then-compensated value is briefly visible.
//
// TCC targets short, strongly-consistent transactions. Each participant splits
// its work into two phases: a reversible Try that *reserves* the resource
// (freeze balance, hold stock) without exposing a final result, followed by a
// Confirm that commits the reservation or a Cancel that releases it. Because the
// resource is only reserved — never business-visible as committed — during the
// Try phase, other readers do not see a half-applied transaction. This is the
// key difference: Saga compensates real effects; TCC confirms/cancels tentative
// reservations.
//
// # The protocol
//
// A [Coordinator] runs a [Transaction] as follows:
//
//   - Try every [Participant] in order. Each Try returns a value (a reservation
//     token) recorded for the second phase.
//   - If every Try succeeds, the coordinator records a commit decision and then
//     Confirms every participant (in order).
//   - If any Try fails, the coordinator records a rollback decision and Cancels
//     every participant that was tried (in reverse order).
//
// Confirm and Cancel MUST be idempotent and are expected to eventually succeed —
// they are retried per [Participant.Retry]. A Confirm/Cancel that ultimately
// fails leaves the transaction in a non-terminal-consistent state and is
// surfaced through the [Result] for manual intervention, not silently dropped.
//
// # Participant obligations
//
// Because a crash can interrupt any phase and recovery replays the second phase,
// participants must handle the three classic TCC hazards themselves — this
// package cannot enforce them across process boundaries:
//
//   - Idempotence: Confirm/Cancel may be called more than once for the same
//     transaction id; the second call must be a no-op.
//   - Empty rollback: Cancel may arrive for a participant whose Try never ran (or
//     crashed before recording a result), in which case its result is nil; Cancel
//     must tolerate that and do nothing.
//   - Anti-hanging: a delayed Try that arrives after a Cancel must not re-reserve
//     the resource. Keying reservations by the transaction id lets a participant
//     detect this.
//
// # Layering
//
// The orchestration is backend-neutral and lives here with zero dependencies.
// The only pluggable seam is [Store], the persistence of the transaction log:
// the bundled [MemoryStore] keeps the framework standalone, while a starter may
// register a durable Store for crash recovery — without changing the
// [Coordinator]. Observability is contributed through the [Observer] seam so a
// starter can attach otel spans without pulling otel into stdlib.
package tcc

import (
	"context"

	"go-spring.org/spring/cloud/resilience"
)

// RetryPolicy governs how a [Participant] phase (try, confirm or cancel) is
// retried on failure. It reuses [resilience.Policy] rather than inventing a
// second knob set, so the same declarative fields (MaxRetries, Timeout, ...)
// govern both outbound resilience and TCC phase retries. The zero value means a
// single attempt with no retry. Confirm and Cancel in particular benefit from a
// non-zero policy, since the TCC contract requires them to eventually succeed.
type RetryPolicy = resilience.Policy

// Participant is one TCC resource that splits its work into three phases: a
// reversible Try that reserves the resource, a Confirm that commits the
// reservation, and a Cancel that releases it.
//
// All three functions are required; a nil one is a programming error rejected by
// the coordinator before any side effect runs (unlike a Saga step, whose
// compensation may legitimately be absent). Confirm and Cancel MUST be
// idempotent and must tolerate a nil result (empty rollback) — see the package
// doc's participant obligations.
type Participant struct {
	// Name identifies the participant for logging, the transaction log and
	// observation. It must be unique within a Transaction; results are keyed by it.
	Name string

	// Try reserves the resource tentatively and returns a value (typically a
	// reservation token / id) that is recorded and handed to Confirm or Cancel so
	// they can target exactly what Try reserved. It may return nil when the
	// reservation is fully identified by the transaction id.
	Try func(ctx context.Context) (any, error)

	// Confirm commits the reservation Try made, receiving Try's returned value. It
	// runs only when every participant's Try succeeded. It must be idempotent.
	Confirm func(ctx context.Context, tried any) error

	// Cancel releases the reservation Try made, receiving Try's returned value (or
	// nil if Try never recorded one). It runs when any Try failed, for every
	// participant that was tried. It must be idempotent and tolerate a nil value
	// (empty rollback).
	Cancel func(ctx context.Context, tried any) error

	// Retry optionally governs retrying this participant's phases. The zero value
	// means a single attempt. A non-zero policy is recommended for Confirm/Cancel,
	// which the TCC contract requires to eventually succeed.
	Retry RetryPolicy
}

// Transaction is an ordered set of [Participant]s executed as one TCC unit. If
// every Try succeeds the participants are confirmed; if any Try fails the tried
// participants are cancelled.
type Transaction struct {
	// ID is the idempotency key of this transaction instance, typically derived
	// from a business/request id. It scopes the transaction log and lets a
	// recovering Coordinator find in-flight work. Callers pass it explicitly so it
	// aligns with their own idempotency key and with each participant's
	// reservation key (which is how empty-rollback and anti-hanging are detected).
	ID string

	// Method is the logical method name this transaction belongs to — the key
	// under which its participants are registered in a [ParticipantRegistry]. It is
	// optional for a plain Execute, but crash recovery needs it: the persisted log
	// stores only progress data, so a recovering process re-supplies the
	// participant definitions by looking this name up. [GlobalTCC] fills it
	// automatically from the joinpoint method.
	Method string

	// Participants are tried in slice order. Confirm runs in the same order;
	// Cancel runs in reverse.
	Participants []Participant
}

// Status is the terminal outcome of executing a [Transaction], plus the
// in-flight states a persisted log can hold for recovery.
type Status int

const (
	// StatusTrying means the transaction is in its Try phase and no global
	// decision has been made yet. A crash here recovers by cancelling whatever was
	// tried, since commit was never chosen.
	StatusTrying Status = iota

	// StatusConfirming means every Try succeeded and the coordinator has decided to
	// commit, but not all Confirms have completed. A crash here recovers forward by
	// confirming the participants (Confirm is idempotent).
	StatusConfirming

	// StatusCommitted means every participant was confirmed; the transaction is
	// done. A committed log is deleted, so a stored snapshot is never Committed.
	StatusCommitted

	// StatusCancelling means a Try failed and the coordinator has decided to roll
	// back, but not all Cancels have completed. A crash here recovers backward by
	// cancelling the tried participants.
	StatusCancelling

	// StatusCancelled means a Try failed and every required Cancel succeeded,
	// leaving the system consistent (rolled back).
	StatusCancelled

	// StatusConfirmFailed means the commit decision was made but at least one
	// Confirm ultimately failed. The system may be left inconsistent; [Result.Errors]
	// carries the failures for alerting / manual intervention.
	StatusConfirmFailed

	// StatusCancelFailed means the rollback decision was made but at least one
	// Cancel ultimately failed. The system may be left inconsistent; [Result.Errors]
	// carries the failures.
	StatusCancelFailed
)

// String renders the status for logs and spans.
func (s Status) String() string {
	switch s {
	case StatusTrying:
		return "Trying"
	case StatusConfirming:
		return "Confirming"
	case StatusCommitted:
		return "Committed"
	case StatusCancelling:
		return "Cancelling"
	case StatusCancelled:
		return "Cancelled"
	case StatusConfirmFailed:
		return "ConfirmFailed"
	case StatusCancelFailed:
		return "CancelFailed"
	default:
		return "Unknown"
	}
}

// Phase distinguishes a participant's three operations, used by [ParticipantError]
// and [Observer] to label what was running.
type Phase int

const (
	// PhaseTry is the reserve operation.
	PhaseTry Phase = iota
	// PhaseConfirm is the commit-the-reservation operation.
	PhaseConfirm
	// PhaseCancel is the release-the-reservation operation.
	PhaseCancel
)

// String renders the phase for logs and spans.
func (p Phase) String() string {
	switch p {
	case PhaseTry:
		return "Try"
	case PhaseConfirm:
		return "Confirm"
	case PhaseCancel:
		return "Cancel"
	default:
		return "Unknown"
	}
}

// ParticipantError attributes an error to a specific participant and phase, so a
// caller inspecting a failed [Result] knows exactly which try, confirm or cancel
// broke.
type ParticipantError struct {
	Participant string
	Phase       Phase
	Err         error
}

// Error implements error.
func (e *ParticipantError) Error() string {
	return "tcc: participant " + e.Participant + " " + e.Phase.String() + ": " + e.Err.Error()
}

// Unwrap exposes the underlying error for errors.Is/As.
func (e *ParticipantError) Unwrap() error { return e.Err }

// Result is the outcome of [Coordinator.Execute]. It always reports the final
// [Status] and every try result gathered, so the caller can tell whether the
// transaction committed or rolled back and, if a confirm/cancel failed, exactly
// where.
type Result struct {
	// Status is the terminal outcome.
	Status Status

	// TryResults maps each tried participant's Name to its Try return value. It
	// includes the participant that failed only if its Try produced a value before
	// erroring.
	TryResults map[string]any

	// Errors collects, in the order they occurred, the failing try followed by any
	// confirm/cancel failures. It is empty when Status is StatusCommitted. The try
	// error is present whenever the cancel path ran, so a StatusCancelled result
	// still explains why it rolled back.
	Errors []ParticipantError
}

// Coordinator orchestrates one [Transaction]. The bundled in-process
// implementation (see [NewCoordinator]) runs the phases synchronously. A future
// external-store implementation satisfies the same interface, so switching to
// crash-recoverable orchestration is a change of construction, not of business
// code. Implementations must be safe for concurrent use.
type Coordinator interface {
	// Execute runs t to a committed or cancelled outcome. It returns the failing
	// try's error (nil when committed) together with a fully populated [Result]; a
	// confirm/cancel failure never masks the original try error, which is why both
	// are surfaced.
	Execute(ctx context.Context, t Transaction) (Result, error)

	// Recover resumes an interrupted transaction after a crash. The caller
	// re-supplies t's participant definitions (typically from a
	// [ParticipantRegistry] keyed by the persisted [Snapshot.Method]); the
	// coordinator loads the log by t.ID to learn how far it got and which decision,
	// if any, was made. A transaction with no persisted log, or one already
	// terminal, is a no-op — recovery is idempotent. If the log shows the Try phase
	// never reached a commit decision it is cancelled (backward); if it shows a
	// commit decision it is confirmed (forward). Either way the replayed phase must
	// be idempotent.
	Recover(ctx context.Context, t Transaction) (Result, error)
}

// Observer is the observability seam. The coordinator calls [Observer.Begin]
// around every participant phase; the returned end function is invoked with the
// phase's error (nil on success). A starter implements this to open an otel span
// per phase without stdlib depending on otel. A nil Observer disables
// observation entirely.
type Observer interface {
	// Begin is called just before a phase runs. The returned context is used for
	// that phase (so a span can propagate through ctx); the returned end func is
	// called exactly once when the phase finishes.
	Begin(ctx context.Context, txID, participant string, phase Phase) (context.Context, func(err error))
}
