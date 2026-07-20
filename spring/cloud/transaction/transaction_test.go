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

package transaction_test

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/spring/aspect"
	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/spring/cloud/transaction"
)

func act(v any) func(context.Context) (any, error) {
	return func(context.Context) (any, error) { return v, nil }
}

func TestExecute_AllStepsCommit(t *testing.T) {
	var order []string
	coord := transaction.NewCoordinator()

	saga := transaction.Saga{
		ID: "s1",
		Steps: []transaction.Step{
			{Name: "a", Action: func(context.Context) (any, error) { order = append(order, "a"); return 1, nil },
				Compensate: func(context.Context, any) error { order = append(order, "!a"); return nil }},
			{Name: "b", Action: func(context.Context) (any, error) { order = append(order, "b"); return "two", nil },
				Compensate: func(context.Context, any) error { order = append(order, "!b"); return nil }},
		},
	}

	res, err := coord.Execute(context.Background(), saga)
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(transaction.StatusCommitted)
	assert.That(t, res.StepResults["a"]).Equal(1)
	assert.That(t, res.StepResults["b"]).Equal("two")
	assert.Slice(t, res.Errors).Empty()
	// No compensation ran, and steps ran in order.
	assert.Slice(t, order).Equal([]string{"a", "b"})
}

func TestExecute_CompensatesInReverseOnFailure(t *testing.T) {
	var order []string
	failB := errors.New("b failed")
	coord := transaction.NewCoordinator()

	saga := transaction.Saga{
		ID: "s2",
		Steps: []transaction.Step{
			{Name: "a", Action: func(context.Context) (any, error) { order = append(order, "a"); return "ra", nil },
				Compensate: func(_ context.Context, r any) error { order = append(order, "!a:"+r.(string)); return nil }},
			{Name: "b", Action: func(context.Context) (any, error) { order = append(order, "b"); return "rb", nil },
				Compensate: func(_ context.Context, r any) error { order = append(order, "!b:"+r.(string)); return nil }},
			{Name: "c", Action: func(context.Context) (any, error) { order = append(order, "c"); return nil, failB }},
		},
	}

	res, err := coord.Execute(context.Background(), saga)
	assert.Error(t, err).Is(failB)
	assert.That(t, res.Status).Equal(transaction.StatusCompensated)
	// a and b were compensated in reverse, each with its own action result; c
	// never succeeded so it is not compensated.
	assert.Slice(t, order).Equal([]string{"a", "b", "c", "!b:rb", "!a:ra"})
	// The failing action's error leads the error list.
	assert.That(t, len(res.Errors)).Equal(1)
	assert.That(t, res.Errors[0].Step).Equal("c")
	assert.That(t, res.Errors[0].Phase).Equal(transaction.PhaseAction)
}

func TestExecute_CompensationFailureIsReported(t *testing.T) {
	failComp := errors.New("restore failed")
	coord := transaction.NewCoordinator()

	saga := transaction.Saga{
		ID: "s3",
		Steps: []transaction.Step{
			{Name: "a", Action: act("ra"),
				Compensate: func(context.Context, any) error { return failComp }},
			{Name: "b", Action: func(context.Context) (any, error) { return nil, errors.New("boom") }},
		},
	}

	res, err := coord.Execute(context.Background(), saga)
	assert.Error(t, err).NotNil()
	assert.That(t, res.Status).Equal(transaction.StatusCompensationFailed)
	// One action error (b) + one compensation error (a).
	assert.That(t, len(res.Errors)).Equal(2)
	assert.That(t, res.Errors[1].Step).Equal("a")
	assert.That(t, res.Errors[1].Phase).Equal(transaction.PhaseCompensate)
	assert.Error(t, res.Errors[1].Err).Is(failComp)
}

func TestExecute_IrreversibleStepDuringRollback(t *testing.T) {
	coord := transaction.NewCoordinator()

	saga := transaction.Saga{
		ID: "s4",
		Steps: []transaction.Step{
			{Name: "a", Action: act("ra")}, // no Compensate -> irreversible
			{Name: "b", Action: func(context.Context) (any, error) { return nil, errors.New("boom") }},
		},
	}

	res, err := coord.Execute(context.Background(), saga)
	assert.Error(t, err).NotNil()
	assert.That(t, res.Status).Equal(transaction.StatusCompensationFailed)
	assert.That(t, res.Errors[1].Step).Equal("a")
	assert.That(t, res.Errors[1].Phase).Equal(transaction.PhaseCompensate)
}

func TestExecute_RetriesActionUnderPolicy(t *testing.T) {
	attempts := 0
	coord := transaction.NewCoordinator()

	saga := transaction.Saga{
		ID: "s5",
		Steps: []transaction.Step{
			{
				Name: "flaky",
				// MaxRetries=2 -> up to 3 attempts; succeed on the third.
				Retry: transaction.RetryPolicy{MaxRetries: 2},
				Action: func(context.Context) (any, error) {
					attempts++
					if attempts < 3 {
						return nil, errors.New("transient")
					}
					return "ok", nil
				},
			},
		},
	}

	res, err := coord.Execute(context.Background(), saga)
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(transaction.StatusCommitted)
	assert.That(t, attempts).Equal(3)
	assert.That(t, res.StepResults["flaky"]).Equal("ok")
}

func TestExecute_StorePersistenceLifecycle(t *testing.T) {
	store := &transaction.MemoryStore{}
	coord := transaction.NewCoordinator(transaction.WithStore(store))

	// Committed saga: log is deleted after success.
	ok := transaction.Saga{ID: "done", Steps: []transaction.Step{{Name: "a", Action: act(1)}}}
	_, err := coord.Execute(context.Background(), ok)
	assert.Error(t, err).Nil()
	_, err = store.Load(context.Background(), "done")
	assert.Error(t, err).Is(transaction.ErrSnapshotNotFound)

	// Compensated saga: log is kept with the terminal status.
	bad := transaction.Saga{ID: "bad", Steps: []transaction.Step{
		{Name: "a", Action: act(1), Compensate: func(context.Context, any) error { return nil }},
		{Name: "b", Action: func(context.Context) (any, error) { return nil, errors.New("boom") }},
	}}
	_, err = coord.Execute(context.Background(), bad)
	assert.Error(t, err).NotNil()
	snap, err := store.Load(context.Background(), "bad")
	assert.Error(t, err).Nil()
	assert.That(t, snap.Status).Equal(transaction.StatusCompensated)
	assert.That(t, snap.ID).Equal("bad")
}

type recordObserver struct {
	begins []string
	ends   []bool // true when the phase ended with an error
}

func (o *recordObserver) Begin(ctx context.Context, sagaID, step string, phase transaction.Phase) (context.Context, func(error)) {
	o.begins = append(o.begins, step+":"+phase.String())
	return ctx, func(err error) { o.ends = append(o.ends, err != nil) }
}

func TestExecute_ObserverSeesEveryPhase(t *testing.T) {
	obs := &recordObserver{}
	coord := transaction.NewCoordinator(transaction.WithObserver(obs))

	saga := transaction.Saga{ID: "s6", Steps: []transaction.Step{
		{Name: "a", Action: act(1), Compensate: func(context.Context, any) error { return nil }},
		{Name: "b", Action: func(context.Context) (any, error) { return nil, errors.New("boom") }},
	}}
	_, err := coord.Execute(context.Background(), saga)
	assert.Error(t, err).NotNil()
	// a.Action, b.Action, then a.Compensate.
	assert.Slice(t, obs.begins).Equal([]string{"a:Action", "b:Action", "a:Compensate"})
	assert.Slice(t, obs.ends).Equal([]bool{false, true, false})
}

func TestGlobalTransactional_RunsRegisteredSaga(t *testing.T) {
	reg := transaction.NewStepRegistry()
	compensated := false
	reg.Register("OrderService.Place",
		transaction.Step{Name: "a", Action: act("ra"),
			Compensate: func(context.Context, any) error { compensated = true; return nil }},
		transaction.Step{Name: "b", Action: func(context.Context) (any, error) { return nil, errors.New("boom") }},
	)
	coord := transaction.NewCoordinator()
	chain := aspect.NewChain(transaction.GlobalTransactional(coord, reg))

	ctx := transaction.WithSagaID(context.Background(), "order-42")
	// The target must not run: it is replaced by the registered saga.
	targetRan := false
	_, err := chain.Run(ctx, "OrderService.Place", func(context.Context) (any, error) {
		targetRan = true
		return nil, nil
	})
	assert.Error(t, err).NotNil()
	assert.That(t, targetRan).False()
	assert.That(t, compensated).True()
}

func TestGlobalTransactional_PassthroughWhenUnregistered(t *testing.T) {
	reg := transaction.NewStepRegistry()
	coord := transaction.NewCoordinator()
	chain := aspect.NewChain(transaction.GlobalTransactional(coord, reg))

	ran := false
	v, err := chain.Run(context.Background(), "Unknown.Method", func(context.Context) (any, error) {
		ran = true
		return "target", nil
	})
	assert.Error(t, err).Nil()
	assert.That(t, ran).True()
	assert.That(t, v).Equal("target")
}

func TestSagaIDContextRoundTrip(t *testing.T) {
	_, ok := transaction.SagaIDFromContext(context.Background())
	assert.That(t, ok).False()

	ctx := transaction.WithSagaID(context.Background(), "abc")
	id, ok := transaction.SagaIDFromContext(ctx)
	assert.That(t, ok).True()
	assert.That(t, id).Equal("abc")
}

// spyStore records every snapshot passed to Save, in order, so tests can assert
// the persist timing (the in-flight marker before an Action, its clearing after).
type spyStore struct {
	transaction.MemoryStore
	saves []transaction.Snapshot
}

func (s *spyStore) Save(ctx context.Context, id string, snap transaction.Snapshot) error {
	s.saves = append(s.saves, snap)
	return s.MemoryStore.Save(ctx, id, snap)
}

func TestExecute_PersistsInProgressBeforeActionThenClears(t *testing.T) {
	store := &spyStore{}
	coord := transaction.NewCoordinator(transaction.WithStore(store))

	saga := transaction.Saga{ID: "p1", Method: "Svc.Do", Steps: []transaction.Step{
		{Name: "a", Action: act("ra"), Compensate: func(context.Context, any) error { return nil }},
		{Name: "b", Action: act("rb"), Compensate: func(context.Context, any) error { return nil }},
	}}
	_, err := coord.Execute(context.Background(), saga)
	assert.Error(t, err).Nil()

	// Two steps => four running Saves: intent(a), confirm(a), intent(b), confirm(b).
	assert.That(t, len(store.saves)).Equal(4)
	// Before a's Action: Running, InProgress=a, nothing completed.
	assert.That(t, store.saves[0].Status).Equal(transaction.StatusRunning)
	assert.That(t, store.saves[0].InProgress).Equal("a")
	assert.That(t, store.saves[0].Method).Equal("Svc.Do")
	assert.Slice(t, store.saves[0].Completed).Empty()
	// After a succeeds: InProgress cleared, a folded in.
	assert.That(t, store.saves[1].InProgress).Equal("")
	assert.Slice(t, store.saves[1].Completed).Equal([]string{"a"})
	// Before b's Action: b in progress, a already completed.
	assert.That(t, store.saves[2].InProgress).Equal("b")
	assert.Slice(t, store.saves[2].Completed).Equal([]string{"a"})
	// After b succeeds.
	assert.That(t, store.saves[3].InProgress).Equal("")
	assert.Slice(t, store.saves[3].Completed).Equal([]string{"a", "b"})
}

func TestPending_ReturnsOnlyRunning(t *testing.T) {
	store := &transaction.MemoryStore{}
	ctx := context.Background()
	assert.Error(t, store.Save(ctx, "run", transaction.Snapshot{ID: "run", Status: transaction.StatusRunning})).Nil()
	assert.Error(t, store.Save(ctx, "gone", transaction.Snapshot{ID: "gone", Status: transaction.StatusCompensated})).Nil()

	pending, err := store.Pending(ctx)
	assert.Error(t, err).Nil()
	assert.That(t, len(pending)).Equal(1)
	assert.That(t, pending[0].ID).Equal("run")
}

func TestRecover_CompensatesInProgressThenCompletedInReverse(t *testing.T) {
	store := &transaction.MemoryStore{}
	ctx := context.Background()
	// A saga that crashed while running step c, having completed a then b.
	assert.Error(t, store.Save(ctx, "r1", transaction.Snapshot{
		ID:          "r1",
		Method:      "Svc.Do",
		Status:      transaction.StatusRunning,
		Completed:   []string{"a", "b"},
		InProgress:  "c",
		StepResults: map[string]any{"a": "ra", "b": "rb"},
	})).Nil()

	var order []string
	coord := transaction.NewCoordinator(transaction.WithStore(store))
	saga := transaction.Saga{ID: "r1", Method: "Svc.Do", Steps: []transaction.Step{
		{Name: "a", Action: act("ra"), Compensate: func(_ context.Context, r any) error { order = append(order, "!a:"+r.(string)); return nil }},
		{Name: "b", Action: act("rb"), Compensate: func(_ context.Context, r any) error { order = append(order, "!b:"+r.(string)); return nil }},
		{Name: "c", Action: act("rc"), Compensate: func(_ context.Context, r any) error {
			// c was in-flight: its result was never recorded, so r is nil.
			order = append(order, "!c:nil")
			assert.That(t, r).Nil()
			return nil
		}},
	}}

	res, err := coord.Recover(ctx, saga)
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(transaction.StatusCompensated)
	// In-flight c first, then b, then a (backward recovery).
	assert.Slice(t, order).Equal([]string{"!c:nil", "!b:rb", "!a:ra"})
	// Terminal log kept for inspection.
	snap, err := store.Load(ctx, "r1")
	assert.Error(t, err).Nil()
	assert.That(t, snap.Status).Equal(transaction.StatusCompensated)
}

func TestRecover_NoLogIsNoOp(t *testing.T) {
	store := &transaction.MemoryStore{}
	coord := transaction.NewCoordinator(transaction.WithStore(store))
	res, err := coord.Recover(context.Background(), transaction.Saga{ID: "missing"})
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(transaction.StatusCommitted)
}

func TestRecover_TerminalSnapshotIsIdempotent(t *testing.T) {
	store := &transaction.MemoryStore{}
	ctx := context.Background()
	assert.Error(t, store.Save(ctx, "t1", transaction.Snapshot{ID: "t1", Status: transaction.StatusCompensated})).Nil()

	compensated := false
	coord := transaction.NewCoordinator(transaction.WithStore(store))
	saga := transaction.Saga{ID: "t1", Steps: []transaction.Step{
		{Name: "a", Action: act("ra"), Compensate: func(context.Context, any) error { compensated = true; return nil }},
	}}
	res, err := coord.Recover(ctx, saga)
	assert.Error(t, err).Nil()
	assert.That(t, res.Status).Equal(transaction.StatusCompensated)
	// Already terminal: no compensation replayed.
	assert.That(t, compensated).False()
}

func TestRecover_WithoutStoreErrors(t *testing.T) {
	coord := transaction.NewCoordinator()
	_, err := coord.Recover(context.Background(), transaction.Saga{ID: "x"})
	assert.Error(t, err).NotNil()
}
