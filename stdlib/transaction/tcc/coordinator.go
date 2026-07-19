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

package tcc

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"go-spring.org/stdlib/resilience"
)

// Option configures the in-process [Coordinator] built by [NewCoordinator].
type Option func(*coordinator)

// WithStore sets the transaction-log [Store]. Without it the coordinator keeps
// no durable log and a crash strands an in-flight transaction; pass a durable
// Store in production.
func WithStore(s Store) Option { return func(c *coordinator) { c.store = s } }

// WithObserver sets the [Observer] used to instrument every participant phase,
// e.g. a starter that opens otel spans.
func WithObserver(o Observer) Option { return func(c *coordinator) { c.observer = o } }

// NewCoordinator returns the bundled in-process [Coordinator]: it tries every
// participant in order and then confirms all (on success) or cancels the tried
// ones in reverse (on any try failure). With no options it uses no store and no
// observer, which is a valid transparent setup for tests and development.
func NewCoordinator(opts ...Option) Coordinator {
	c := &coordinator{}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

type coordinator struct {
	store    Store
	observer Observer
}

// triedParticipant records a participant whose Try ran, with the value it
// returned, so the second phase can target exactly what was reserved.
type triedParticipant struct {
	participant Participant
	result      any
}

func (c *coordinator) Execute(ctx context.Context, t Transaction) (Result, error) {
	res := Result{Status: StatusTrying, TryResults: make(map[string]any, len(t.Participants))}

	// Fail fast on a malformed transaction before any side effect: unlike a Saga
	// step, a TCC participant with a missing phase is always a programming error.
	if err := validate(t); err != nil {
		res.Status = StatusCancelFailed
		res.Errors = append(res.Errors, ParticipantError{Phase: PhaseTry, Err: err})
		return res, err
	}

	tried := make([]triedParticipant, 0, len(t.Participants))
	for _, p := range t.Participants {
		// Record intent before running: a crashed Try may have partially reserved,
		// so a recovering process must know to cancel this participant.
		c.persist(ctx, t, StatusTrying, tried, p.Name)

		result, err := c.runPhase(ctx, t.ID, p, PhaseTry, func(ctx context.Context) (any, error) {
			return p.Try(ctx)
		})
		if err != nil {
			// The failing try's error leads Result.Errors so a cancelled transaction
			// still explains why it rolled back.
			res.Errors = append(res.Errors, ParticipantError{Participant: p.Name, Phase: PhaseTry, Err: err})
			if result != nil {
				res.TryResults[p.Name] = result
			}
			c.cancel(ctx, t, tried, &res)
			c.finish(ctx, t, &res, tried)
			return res, err
		}
		res.TryResults[p.Name] = result
		tried = append(tried, triedParticipant{participant: p, result: result})
		// Fold this participant into the log and clear the in-flight marker.
		c.persist(ctx, t, StatusTrying, tried, "")
	}

	// Every Try succeeded: durably record the commit decision, then confirm.
	c.persist(ctx, t, StatusConfirming, tried, "")
	c.confirm(ctx, t, tried, &res)
	c.finish(ctx, t, &res, tried)
	if res.Status == StatusCommitted {
		return res, nil
	}
	// A confirm failure is not a try error, so Execute returns nil error here; the
	// non-committed Status and Result.Errors carry the failure for the caller.
	return res, nil
}

// Recover resumes an interrupted transaction from its persisted log. A missing
// log (committed transactions delete theirs) or an already-terminal one is a
// no-op, so recovery is idempotent. The recorded decision drives the direction:
// StatusConfirming confirms forward; StatusTrying or StatusCancelling cancels
// backward. t supplies the participant definitions the log cannot.
func (c *coordinator) Recover(ctx context.Context, t Transaction) (Result, error) {
	if c.store == nil {
		return Result{}, errors.New("tcc: Recover requires a Store")
	}
	if err := validate(t); err != nil {
		return Result{}, err
	}

	snap, err := c.store.Load(ctx, t.ID)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			// Nothing persisted (committed transactions delete their log): done.
			return Result{Status: StatusCommitted}, nil
		}
		return Result{}, err
	}
	if isTerminal(snap.Status) {
		// Already reached a terminal outcome; report it without acting again.
		return Result{Status: snap.Status}, nil
	}

	byName := make(map[string]Participant, len(t.Participants))
	for _, p := range t.Participants {
		byName[p.Name] = p
	}

	res := Result{Status: snap.Status, TryResults: snap.TryResults}
	if res.TryResults == nil {
		res.TryResults = make(map[string]any)
	}

	// Rebuild the tried set in forward (try) order from the log.
	tried := make([]triedParticipant, 0, len(snap.Tried))
	for _, name := range snap.Tried {
		if p, ok := byName[name]; ok {
			tried = append(tried, triedParticipant{participant: p, result: snap.TryResults[name]})
		}
	}

	if snap.Status == StatusConfirming {
		// The commit decision was durable, so drive it forward: confirm every tried
		// participant (idempotent), regardless of how far the first confirm got.
		c.confirm(ctx, t, tried, &res)
		c.finish(ctx, t, &res, tried)
		return res, nil
	}

	// No commit decision was reached (StatusTrying) or rollback was already chosen
	// (StatusCancelling): cancel backward. An in-flight participant whose Try
	// result was never recorded is cancelled first, with a nil value (empty
	// rollback), which participants must tolerate.
	if snap.InProgress != "" {
		if p, ok := byName[snap.InProgress]; ok && !containsName(snap.Tried, snap.InProgress) {
			tried = append(tried, triedParticipant{participant: p, result: nil})
		}
	}
	c.cancel(ctx, t, tried, &res)
	c.finish(ctx, t, &res, tried)
	return res, nil
}

// confirm commits every tried participant in try order, collecting failures into
// res.Errors and downgrading the status on any failure.
func (c *coordinator) confirm(ctx context.Context, t Transaction, tried []triedParticipant, res *Result) {
	res.Status = StatusCommitted
	for _, tp := range tried {
		_, err := c.runPhase(ctx, t.ID, tp.participant, PhaseConfirm, func(ctx context.Context) (any, error) {
			return nil, tp.participant.Confirm(ctx, tp.result)
		})
		if err != nil {
			res.Status = StatusConfirmFailed
			res.Errors = append(res.Errors, ParticipantError{Participant: tp.participant.Name, Phase: PhaseConfirm, Err: err})
		}
	}
}

// cancel releases every tried participant in reverse try order, collecting
// failures into res.Errors and downgrading the status on any failure.
func (c *coordinator) cancel(ctx context.Context, t Transaction, tried []triedParticipant, res *Result) {
	res.Status = StatusCancelled
	for _, tp := range slices.Backward(tried) {
		_, err := c.runPhase(ctx, t.ID, tp.participant, PhaseCancel, func(ctx context.Context) (any, error) {
			return nil, tp.participant.Cancel(ctx, tp.result)
		})
		if err != nil {
			res.Status = StatusCancelFailed
			res.Errors = append(res.Errors, ParticipantError{Participant: tp.participant.Name, Phase: PhaseCancel, Err: err})
		}
	}
}

// runPhase runs one participant phase under its retry policy and observer. The
// observer end func always fires with the phase's final error so failures are
// observable.
func (c *coordinator) runPhase(ctx context.Context, txID string, p Participant, phase Phase,
	fn func(context.Context) (any, error)) (any, error) {

	end := func(error) {}
	if c.observer != nil {
		ctx, end = c.observer.Begin(ctx, txID, p.Name, phase)
	}

	var result any
	err := runWithPolicy(ctx, p.Retry, p.Name, func(ctx context.Context) error {
		var e error
		result, e = fn(ctx)
		return e
	})
	end(err)
	return result, err
}

// persist writes the in-flight transaction log while phases run, when a store is
// configured. A persistence error is intentionally swallowed: the transaction
// has already made progress and failing the whole operation on a log write would
// be worse than a gap in the log. Durable-store implementations should surface
// such problems through their own monitoring.
func (c *coordinator) persist(ctx context.Context, t Transaction, status Status, tried []triedParticipant, inProgress string) {
	if c.store == nil {
		return
	}
	_ = c.store.Save(ctx, t.ID, c.snapshot(t, status, tried, inProgress))
}

// finish writes the terminal transaction log. A committed transaction's log is
// deleted (the work is done and needs no recovery); a cancelled or failed
// transaction's log is kept so operators and recovery can inspect it.
func (c *coordinator) finish(ctx context.Context, t Transaction, res *Result, tried []triedParticipant) {
	if c.store == nil {
		return
	}
	if res.Status == StatusCommitted {
		_ = c.store.Delete(ctx, t.ID)
		return
	}
	_ = c.store.Save(ctx, t.ID, c.snapshot(t, res.Status, tried, ""))
}

func (c *coordinator) snapshot(t Transaction, status Status, tried []triedParticipant, inProgress string) Snapshot {
	names := make([]string, len(tried))
	results := make(map[string]any, len(tried))
	for i, tp := range tried {
		names[i] = tp.participant.Name
		results[tp.participant.Name] = tp.result
	}
	return Snapshot{
		ID:         t.ID,
		Method:     t.Method,
		Status:     status,
		Tried:      names,
		InProgress: inProgress,
		TryResults: results,
		UpdatedAt:  time.Now(),
	}
}

// validate rejects a transaction whose participants are not fully specified. A
// TCC participant needs all three phases and a name; a missing one is a
// programming error caught before any side effect.
func validate(t Transaction) error {
	seen := make(map[string]struct{}, len(t.Participants))
	for i, p := range t.Participants {
		if p.Name == "" {
			return fmt.Errorf("tcc: participant %d has no Name", i)
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("tcc: duplicate participant name %q", p.Name)
		}
		seen[p.Name] = struct{}{}
		if p.Try == nil || p.Confirm == nil || p.Cancel == nil {
			return fmt.Errorf("tcc: participant %q must define Try, Confirm and Cancel", p.Name)
		}
	}
	return nil
}

// isTerminal reports whether a status is a final outcome (no further action).
func isTerminal(s Status) bool {
	switch s {
	case StatusCommitted, StatusCancelled, StatusConfirmFailed, StatusCancelFailed:
		return true
	default:
		return false
	}
}

func containsName(names []string, name string) bool {
	return slices.Contains(names, name)
}

// runWithPolicy runs fn once when the policy is zero, or under the bundled
// resilience "default" executor (retry, per-attempt timeout, ...) otherwise. It
// reuses [resilience.Policy] so TCC phase retries and outbound resilience share
// one knob set instead of duplicating retry logic here.
func runWithPolicy(ctx context.Context, p RetryPolicy, resource string, fn func(context.Context) error) error {
	if p == (RetryPolicy{}) {
		return fn(ctx)
	}
	drv, err := resilience.MustGetDriver("default")
	if err != nil {
		return err
	}
	exec, err := drv.NewExecutor(p)
	if err != nil {
		return err
	}
	defer func() { _ = exec.Close() }()
	return exec.Execute(ctx, resource, fn)
}
