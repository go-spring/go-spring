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

package StarterTransactionTCC

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
	"go-spring.org/spring/transaction/tcc"
)

func okParticipant(name string, confirmed, cancelled *[]string) tcc.Participant {
	return tcc.Participant{
		Name:    name,
		Try:     func(context.Context) (any, error) { return "tok:" + name, nil },
		Confirm: func(context.Context, any) error { *confirmed = append(*confirmed, name); return nil },
		Cancel:  func(context.Context, any) error { *cancelled = append(*cancelled, name); return nil },
	}
}

func TestNewCoordinator_TracingToggle(t *testing.T) {
	// Both variants must produce a usable coordinator; the tracing flag only
	// controls whether an observer is attached.
	assert.That(t, newCoordinator(Config{Tracing: true}, &tcc.MemoryStore{})).NotNil()
	assert.That(t, newCoordinator(Config{Tracing: false}, &tcc.MemoryStore{})).NotNil()
}

func TestOtelObserver_DrivesTCCWithoutPanic(t *testing.T) {
	// With no TracerProvider installed the otel global is a no-op, but the observer
	// must still return a usable context and an end func that records the outcome
	// without panicking — exercised here through a real cancel path.
	coord := tcc.NewCoordinator(tcc.WithObserver(otelObserver{}))

	var confirmed, cancelled []string
	res, err := coord.Execute(context.Background(), tcc.Transaction{
		ID: "tx-1",
		Participants: []tcc.Participant{
			okParticipant("a", &confirmed, &cancelled),
			{Name: "b",
				Try:     func(context.Context) (any, error) { return nil, errors.New("boom") },
				Confirm: func(context.Context, any) error { return nil },
				Cancel:  func(context.Context, any) error { cancelled = append(cancelled, "b"); return nil }},
		},
	})
	assert.Error(t, err).Matches("boom")
	assert.That(t, res.Status).Equal(tcc.StatusCancelled)
	assert.Slice(t, cancelled).Equal([]string{"a"})
}

func TestSpanName(t *testing.T) {
	assert.That(t, spanName(tcc.PhaseTry, "ReserveStock")).Equal("tcc.try ReserveStock")
	assert.That(t, spanName(tcc.PhaseConfirm, "ReserveStock")).Equal("tcc.confirm ReserveStock")
	assert.That(t, spanName(tcc.PhaseCancel, "ReserveStock")).Equal("tcc.cancel ReserveStock")
}

func TestRecoveryRunner_ConfirmsPendingCommit(t *testing.T) {
	ctx := context.Background()
	store := &tcc.MemoryStore{}
	// A transaction that crashed after recording the commit decision.
	assert.Error(t, store.Save(ctx, "tx-2", tcc.Snapshot{
		ID:         "tx-2",
		Method:     "OrderService.Place",
		Status:     tcc.StatusConfirming,
		Tried:      []string{"a"},
		TryResults: map[string]any{"a": "tok:a"},
	})).Nil()

	reg := tcc.NewParticipantRegistry()
	var confirmed, cancelled []string
	reg.Register("OrderService.Place", okParticipant("a", &confirmed, &cancelled))

	r := &recoveryRunner{
		Store:    store,
		Registry: reg,
		Coord:    tcc.NewCoordinator(tcc.WithStore(store)),
	}
	assert.Error(t, r.Run(ctx)).Nil()
	assert.Slice(t, confirmed).Equal([]string{"a"})
	assert.Slice(t, cancelled).Equal([]string(nil))
	// Committed transactions delete their log.
	_, loadErr := store.Load(ctx, "tx-2")
	assert.Error(t, loadErr).Matches("not found")
}

func TestRecoveryRunner_NoOpWhenNothingPending(t *testing.T) {
	r := &recoveryRunner{
		Store:    &tcc.MemoryStore{},
		Registry: tcc.NewParticipantRegistry(),
		Coord:    tcc.NewCoordinator(tcc.WithStore(&tcc.MemoryStore{})),
	}
	assert.Error(t, r.Run(context.Background())).Nil()
}
