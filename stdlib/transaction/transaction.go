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

// Package transaction defines a framework-agnostic, zero-dependency abstraction
// for cross-resource / cross-service consistency, with the Saga pattern as its
// first (and currently only) form.
//
// It is the Go-idiomatic equivalent of Seata Saga and Spring's
// @GlobalTransactional, reached without replicating Seata's TC/TM/RM roles. A
// long-running business operation is expressed as an ordered list of compensable
// [Step]s: each step runs a forward [Step.Action] and, if a later step fails,
// the already-succeeded steps are undone in reverse order by their
// [Step.Compensate] functions. The result is eventual consistency with an
// explicit compensation path, covering everything a single-database
// @Transactional cannot — MQ publishes, outbound HTTP calls, cache writes and
// other non-SQL downstreams.
//
// # What it is not
//
//   - It provides no isolation. Intermediate states of a Saga are visible to
//     other readers between steps, so business code must guard against dirty
//     reads itself (a "processing" status flag, a per-key [go-spring.org/stdlib/lock]).
//     This is a Saga, not XA — do not treat it as one.
//   - It does not parse SQL or generate undo logs (that would be the AT model,
//     deliberately left out — see the design note). Compensation is a business
//     function the caller supplies.
//
// # Layering
//
// The Saga orchestration is backend-neutral, so it lives entirely here with zero
// dependencies. The only pluggable seam is [Store], the persistence of the saga
// log: the bundled in-memory implementation keeps the framework standalone, while
// a starter may register a gorm/redis/etcd-backed Store to support crash
// recovery — without changing the [Coordinator]. Observability is contributed
// through the [Observer] seam so a starter can attach otel spans without pulling
// otel into stdlib.
package transaction

import (
	"context"

	"go-spring.org/stdlib/resilience"
)

// RetryPolicy describes how an individual [Step] phase (action or compensation)
// is retried on failure. It reuses [resilience.Policy] rather than inventing a
// second knob set, so the same declarative fields (MaxRetries, Timeout, ...)
// govern both outbound resilience and Saga step retries. The zero value means a
// single attempt with no retry.
type RetryPolicy = resilience.Policy

// Step is one compensable unit of a [Saga]. Action is the forward operation;
// Compensate undoes it if a later step fails.
//
// Compensate MUST be idempotent: it can be retried (per Retry) and, after crash
// recovery, replayed against a resource that may already be in the compensated
// state. It receives the value Action returned so it can target exactly what was
// created (an order id, a reservation token), and must not rely on any process
// state beyond that value and its own dependencies.
type Step struct {
	// Name identifies the step for logging, the saga log and observation. It must
	// be unique within a Saga; results are keyed by it.
	Name string

	// Action is the forward operation. Its return value is recorded in
	// [Result.StepResults] and handed to Compensate; it may be nil when the step
	// produces nothing to undo by value.
	Action func(ctx context.Context) (any, error)

	// Compensate undoes a previously successful Action. It receives that Action's
	// result. A nil Compensate marks the step as irreversible: if compensation
	// reaches it, that is recorded as a compensation failure so the operator is
	// alerted rather than silently losing consistency.
	Compensate func(ctx context.Context, result any) error

	// Retry optionally governs retrying this step's Action and Compensate. The
	// zero value means a single attempt. Compensation reuses the same policy so a
	// transient downstream error does not immediately escalate to manual
	// intervention.
	Retry RetryPolicy
}

// Saga is an ordered set of [Step]s executed as a single logical operation. If
// any step fails, the steps that already succeeded are compensated in reverse
// order.
type Saga struct {
	// ID is the idempotency key of this saga instance, typically derived from a
	// business/request id. It scopes the saga log and lets a recovering
	// Coordinator find in-flight work. Callers are expected to pass it explicitly
	// so it aligns with their own idempotency-key.
	ID string

	// Method is the logical method name this saga belongs to — the key under
	// which its steps are registered in a [StepRegistry]. It is optional for a
	// plain Execute, but crash recovery needs it: the persisted saga log stores
	// only progress data, so a recovering process re-supplies the step
	// definitions by looking this name up in the registry.
	// [GlobalTransactional] fills it automatically from the joinpoint method.
	Method string

	// Steps are executed in slice order. Compensation runs in reverse.
	Steps []Step
}

// Status is the terminal outcome of executing a [Saga].
type Status int

const (
	// StatusRunning means the saga is in flight: at least one step has started
	// but the saga has not reached a terminal outcome. A persisted snapshot in
	// this state is what crash recovery scans for.
	StatusRunning Status = iota

	// StatusCommitted means every step's Action succeeded; no compensation ran.
	StatusCommitted

	// StatusCompensated means a step failed and every required compensation
	// succeeded, leaving the system in a consistent (rolled-back) state.
	StatusCompensated

	// StatusCompensationFailed means a step failed and at least one compensation
	// also failed (or the step was irreversible). The system may be left
	// inconsistent; [Result.Errors] carries the failures for alerting / manual
	// intervention.
	StatusCompensationFailed
)

// String renders the status for logs and spans.
func (s Status) String() string {
	switch s {
	case StatusRunning:
		return "Running"
	case StatusCommitted:
		return "Committed"
	case StatusCompensated:
		return "Compensated"
	case StatusCompensationFailed:
		return "CompensationFailed"
	default:
		return "Unknown"
	}
}

// Phase distinguishes a step's forward action from its compensation, used by
// [StepError] and [Observer] to label what was running.
type Phase int

const (
	// PhaseAction is the forward operation.
	PhaseAction Phase = iota
	// PhaseCompensate is the reverse (undo) operation.
	PhaseCompensate
)

// String renders the phase for logs and spans.
func (p Phase) String() string {
	switch p {
	case PhaseAction:
		return "Action"
	case PhaseCompensate:
		return "Compensate"
	default:
		return "Unknown"
	}
}

// StepError attributes an error to a specific step and phase, so a caller
// inspecting a failed [Result] knows exactly which action or compensation broke.
type StepError struct {
	Step  string
	Phase Phase
	Err   error
}

// Error implements error.
func (e *StepError) Error() string {
	return "transaction: step " + e.Step + " " + e.Phase.String() + ": " + e.Err.Error()
}

// Unwrap exposes the underlying error for errors.Is/As.
func (e *StepError) Unwrap() error { return e.Err }

// Result is the outcome of [Coordinator.Execute]. It always reports the final
// [Status] and every step result gathered so far, so the caller can tell whether
// the compensation path ran and, if compensation failed, exactly where.
type Result struct {
	// Status is the terminal outcome.
	Status Status

	// StepResults maps each executed step's Name to its Action return value. It
	// includes the step that failed only if its Action produced a value before
	// erroring.
	StepResults map[string]any

	// Errors collects, in the order they occurred, the failing action followed by
	// any compensation failures. It is empty when Status is StatusCommitted. The
	// action error is present whenever compensation ran, so a StatusCompensated
	// result still explains why.
	Errors []StepError
}

// Coordinator orchestrates one [Saga]. The bundled in-process implementation
// (see [NewCoordinator]) runs steps synchronously and, on failure, compensates
// in reverse. A future external-store implementation satisfies the same
// interface, so switching to crash-recoverable orchestration is a change of
// construction, not of business code. Implementations must be safe for
// concurrent use.
type Coordinator interface {
	// Execute runs s to completion or compensation. It returns the failing
	// action's error (nil when committed) together with a fully populated
	// [Result]; a compensation failure never masks the original action error,
	// which is why both are surfaced.
	Execute(ctx context.Context, s Saga) (Result, error)

	// Recover resumes an interrupted saga after a crash by compensating whatever
	// it had made progress on, in reverse order (backward recovery). The caller
	// re-supplies s's step definitions (typically from a [StepRegistry] keyed by
	// the persisted [Snapshot.Method]); the coordinator loads the saga log by
	// s.ID to learn how far it got. A saga with no persisted log, or one already
	// in a terminal state, is a no-op — recovery is idempotent. Because the crash
	// point is unknown (a step's Action may or may not have committed), every
	// possibly-effected step is compensated, which is why compensation must be
	// idempotent.
	Recover(ctx context.Context, s Saga) (Result, error)
}

// Observer is the observability seam. The coordinator calls [Observer.Begin]
// around every step phase; the returned end function is invoked with the phase's
// error (nil on success). A starter implements this to open an otel span per
// step — one child span for the action, one for each compensation — without
// stdlib depending on otel. A nil Observer disables observation entirely.
type Observer interface {
	// Begin is called just before a phase runs. The returned context is used for
	// that phase (so a span can propagate through ctx); the returned end func is
	// called exactly once when the phase finishes.
	Begin(ctx context.Context, sagaID, step string, phase Phase) (context.Context, func(err error))
}
