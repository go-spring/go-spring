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
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// tracer records the order and phase of every participant call, so tests can
// assert the exact TCC protocol drove the participants.
type tracer struct{ calls []string }

func (tr *tracer) mark(s string) { tr.calls = append(tr.calls, s) }

// okParticipant builds a well-formed participant that records its calls.
func okParticipant(tr *tracer, name string) Participant {
	return Participant{
		Name:    name,
		Try:     func(context.Context) (any, error) { tr.mark("try:" + name); return "tok:" + name, nil },
		Confirm: func(_ context.Context, v any) error { tr.mark("confirm:" + name + ":" + toStr(v)); return nil },
		Cancel:  func(_ context.Context, v any) error { tr.mark("cancel:" + name + ":" + toStr(v)); return nil },
	}
}

func toStr(v any) string {
	if v == nil {
		return "<nil>"
	}
	return v.(string)
}

func TestExecute_CommitsWhenAllTrySucceed(t *testing.T) {
	tr := &tracer{}
	coord := NewCoordinator()
	res, err := coord.Execute(context.Background(), Transaction{
		ID:           "tx-1",
		Participants: []Participant{okParticipant(tr, "a"), okParticipant(tr, "b")},
	})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(StatusCommitted)
	// Try both in order, then confirm both in order — no cancels.
	assert.Slice(t, tr.calls).Equal([]string{"try:a", "try:b", "confirm:a:tok:a", "confirm:b:tok:b"})
	assert.That(t, res.TryResults["a"]).Equal("tok:a")
}

func TestExecute_CancelsTriedWhenATryFails(t *testing.T) {
	tr := &tracer{}
	failing := Participant{
		Name:    "b",
		Try:     func(context.Context) (any, error) { tr.mark("try:b"); return nil, errors.New("boom") },
		Confirm: func(context.Context, any) error { return nil },
		Cancel:  func(_ context.Context, v any) error { tr.mark("cancel:b:" + toStr(v)); return nil },
	}
	coord := NewCoordinator()
	res, err := coord.Execute(context.Background(), Transaction{
		ID:           "tx-2",
		Participants: []Participant{okParticipant(tr, "a"), failing},
	})
	assert.Error(t, err).Matches("boom")
	assert.That(t, res.Status).Equal(StatusCancelled)
	// a tried, b tried (failed), then only the successfully-tried a is cancelled.
	assert.Slice(t, tr.calls).Equal([]string{"try:a", "try:b", "cancel:a:tok:a"})
	assert.That(t, res.Errors[0].Phase).Equal(PhaseTry)
}

func TestExecute_RejectsMissingPhaseBeforeSideEffect(t *testing.T) {
	tried := false
	coord := NewCoordinator()
	res, err := coord.Execute(context.Background(), Transaction{
		ID: "tx-3",
		Participants: []Participant{{
			Name: "a",
			Try:  func(context.Context) (any, error) { tried = true; return nil, nil },
			// Confirm/Cancel deliberately nil.
		}},
	})
	assert.Error(t, err).Matches("must define Try, Confirm and Cancel")
	assert.That(t, res.Status).Equal(StatusCancelFailed)
	assert.That(t, tried).False() // validated before any Try ran
}

func TestExecute_ConfirmFailureIsSurfaced(t *testing.T) {
	coord := NewCoordinator()
	res, err := coord.Execute(context.Background(), Transaction{
		ID: "tx-4",
		Participants: []Participant{{
			Name:    "a",
			Try:     func(context.Context) (any, error) { return "t", nil },
			Confirm: func(context.Context, any) error { return errors.New("confirm-broke") },
			Cancel:  func(context.Context, any) error { return nil },
		}},
	})
	// A confirm failure is not a try error, so Execute returns nil error but a
	// non-committed status carrying the failure.
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(StatusConfirmFailed)
	assert.That(t, res.Errors[0].Phase).Equal(PhaseConfirm)
}

func TestRecover_ConfirmsForwardWhenDecisionWasCommit(t *testing.T) {
	ctx := context.Background()
	store := &MemoryStore{}
	// A transaction that crashed after recording the commit decision.
	assert.Error(t, store.Save(ctx, "tx-5", Snapshot{
		ID:         "tx-5",
		Method:     "OrderService.Place",
		Status:     StatusConfirming,
		Tried:      []string{"a", "b"},
		TryResults: map[string]any{"a": "tok:a", "b": "tok:b"},
	})).Nil()

	tr := &tracer{}
	coord := NewCoordinator(WithStore(store))
	res, err := coord.Recover(ctx, Transaction{
		ID:           "tx-5",
		Method:       "OrderService.Place",
		Participants: []Participant{okParticipant(tr, "a"), okParticipant(tr, "b")},
	})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(StatusCommitted)
	assert.Slice(t, tr.calls).Equal([]string{"confirm:a:tok:a", "confirm:b:tok:b"})
	// Committed transactions delete their log.
	_, loadErr := store.Load(ctx, "tx-5")
	assert.Error(t, loadErr).Matches("not found")
}

func TestRecover_CancelsBackwardWhenNoCommitDecision(t *testing.T) {
	ctx := context.Background()
	store := &MemoryStore{}
	// Crashed mid-Try: a done, b in flight, no commit decision.
	assert.Error(t, store.Save(ctx, "tx-6", Snapshot{
		ID:         "tx-6",
		Method:     "OrderService.Place",
		Status:     StatusTrying,
		Tried:      []string{"a"},
		InProgress: "b",
		TryResults: map[string]any{"a": "tok:a"},
	})).Nil()

	tr := &tracer{}
	coord := NewCoordinator(WithStore(store))
	res, err := coord.Recover(ctx, Transaction{
		ID:           "tx-6",
		Method:       "OrderService.Place",
		Participants: []Participant{okParticipant(tr, "a"), okParticipant(tr, "b")},
	})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(StatusCancelled)
	// In-flight b cancelled first (empty rollback, nil result), then a in reverse.
	assert.Slice(t, tr.calls).Equal([]string{"cancel:b:<nil>", "cancel:a:tok:a"})
}

func TestRecover_NoOpWhenTerminalOrMissing(t *testing.T) {
	ctx := context.Background()
	store := &MemoryStore{}
	coord := NewCoordinator(WithStore(store))

	// Missing log => treated as committed no-op.
	res, err := coord.Recover(ctx, Transaction{ID: "gone",
		Participants: []Participant{okParticipant(&tracer{}, "a")}})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(StatusCommitted)

	// Already-terminal log => reported without acting.
	assert.Error(t, store.Save(ctx, "tx-7", Snapshot{ID: "tx-7", Status: StatusCancelled})).Nil()
	res, err = coord.Recover(ctx, Transaction{ID: "tx-7",
		Participants: []Participant{okParticipant(&tracer{}, "a")}})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(StatusCancelled)
}

func TestRecover_RequiresStore(t *testing.T) {
	_, err := NewCoordinator().Recover(context.Background(), Transaction{ID: "x",
		Participants: []Participant{okParticipant(&tracer{}, "a")}})
	assert.Error(t, err).Matches("requires a Store")
}

func TestStore_PendingReturnsOnlyNonTerminal(t *testing.T) {
	ctx := context.Background()
	store := &MemoryStore{}
	assert.Error(t, store.Save(ctx, "run", Snapshot{ID: "run", Status: StatusConfirming})).Nil()
	assert.Error(t, store.Save(ctx, "done", Snapshot{ID: "done", Status: StatusCancelled})).Nil()
	pending, err := store.Pending(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(pending)).Equal(1)
	assert.That(t, pending[0].ID).Equal("run")
}

func TestStatusString(t *testing.T) {
	assert.That(t, StatusTrying.String()).Equal("Trying")
	assert.That(t, StatusCommitted.String()).Equal("Committed")
	assert.That(t, StatusCancelFailed.String()).Equal("CancelFailed")
	assert.That(t, PhaseConfirm.String()).Equal("Confirm")
}
