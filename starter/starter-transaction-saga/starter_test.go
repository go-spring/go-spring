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

package StarterTransactionSaga

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/spring/transaction"
)

func TestNewCoordinator_TracingToggle(t *testing.T) {
	// Both variants must produce a usable coordinator; the tracing flag only
	// controls whether an observer is attached.
	assert.That(t, newCoordinator(Config{Tracing: true}, &transaction.MemoryStore{})).NotNil()
	assert.That(t, newCoordinator(Config{Tracing: false}, &transaction.MemoryStore{})).NotNil()
}

func TestOtelObserver_DrivesSagaWithoutPanic(t *testing.T) {
	// With no TracerProvider installed the otel global is a no-op, but the
	// observer must still return a usable context and an end func that records the
	// outcome without panicking — exercised here through a real compensation path.
	coord := transaction.NewCoordinator(transaction.WithObserver(otelObserver{}))

	compensated := false
	res, err := coord.Execute(context.Background(), transaction.Saga{
		ID: "order-1",
		Steps: []transaction.Step{
			{Name: "a", Action: func(context.Context) (any, error) { return "ra", nil },
				Compensate: func(context.Context, any) error { compensated = true; return nil }},
			{Name: "b", Action: func(context.Context) (any, error) { return nil, errors.New("boom") }},
		},
	})
	assert.Error(t, err).NotNil()
	assert.That(t, res.Status).Equal(transaction.StatusCompensated)
	assert.That(t, compensated).True()
}

func TestSpanName(t *testing.T) {
	assert.That(t, spanName(transaction.PhaseAction, "DeductInventory")).Equal("saga.action DeductInventory")
	assert.That(t, spanName(transaction.PhaseCompensate, "DeductInventory")).Equal("saga.compensate DeductInventory")
}

func TestRecoveryRunner_CompensatesPendingSaga(t *testing.T) {
	ctx := context.Background()
	store := &transaction.MemoryStore{}
	// A saga interrupted while running step b, after completing a.
	assert.Error(t, store.Save(ctx, "order-1", transaction.Snapshot{
		ID:          "order-1",
		Method:      "OrderService.Place",
		Status:      transaction.StatusRunning,
		Completed:   []string{"a"},
		InProgress:  "b",
		StepResults: map[string]any{"a": "ra"},
	})).Nil()

	reg := transaction.NewStepRegistry()
	var order []string
	reg.Register("OrderService.Place",
		transaction.Step{Name: "a", Action: func(context.Context) (any, error) { return "ra", nil },
			Compensate: func(context.Context, any) error { order = append(order, "!a"); return nil }},
		transaction.Step{Name: "b", Action: func(context.Context) (any, error) { return "rb", nil },
			Compensate: func(context.Context, any) error { order = append(order, "!b"); return nil }},
	)

	r := &recoveryRunner{
		Store:    store,
		Registry: reg,
		Coord:    transaction.NewCoordinator(transaction.WithStore(store)),
	}
	assert.Error(t, r.Run(ctx)).Nil()
	// In-flight b compensated first, then completed a.
	assert.Slice(t, order).Equal([]string{"!b", "!a"})
	snap, err := store.Load(ctx, "order-1")
	assert.Error(t, err).Nil()
	assert.That(t, snap.Status).Equal(transaction.StatusCompensated)
}

func TestRecoveryRunner_NoOpWhenNothingPending(t *testing.T) {
	r := &recoveryRunner{
		Store:    &transaction.MemoryStore{},
		Registry: transaction.NewStepRegistry(),
		Coord:    transaction.NewCoordinator(transaction.WithStore(&transaction.MemoryStore{})),
	}
	assert.Error(t, r.Run(context.Background())).Nil()
}
