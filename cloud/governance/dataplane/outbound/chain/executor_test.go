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

package chain

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/stdlib/testing/assert"
)

// stubExecutor records that it was reached and then calls fn, the terminal
// behavior a real innermost executor has.
type stubExecutor struct {
	reached *bool
	err     error
}

func (s stubExecutor) Execute(ctx context.Context, _ Operation, rec *Recorder, fn RoundFunc) error {
	*s.reached = true
	if s.err != nil {
		return s.err
	}
	// No loop above in this stub, so no round is open: open one, the way the
	// terminal's owner would, so the fn it calls runs in a round.
	return fn(ctx, rec.NewRound())
}

func (stubExecutor) Close() error { return nil }

func TestNextNilCallsFn(t *testing.T) {
	boom := errors.New("boom")
	called := false
	rec := new(Recorder)
	rec.NewRound() // the loop opens the round the terminal hands to fn
	err := Next(nil, context.Background(), Operation{}, rec, func(context.Context, *Round) error {
		called = true
		return boom
	})
	assert.That(t, called).True()
	assert.That(t, errors.Is(err, boom)).True()
}

func TestNextDelegatesToNext(t *testing.T) {
	reached := false
	called := false
	rec := &Recorder{}
	err := Next(stubExecutor{reached: &reached}, context.Background(), Operation{}, rec,
		func(context.Context, *Round) error {
			called = true
			return nil
		})
	assert.That(t, reached).True()
	assert.That(t, called).True()
	assert.That(t, err).Nil()
}

func TestNextReturnsNextErrorWithoutCallingFn(t *testing.T) {
	reached := false
	refused := errors.New("refused")
	called := false
	err := Next(stubExecutor{reached: &reached, err: refused}, context.Background(), Operation{}, new(Recorder),
		func(context.Context, *Round) error {
			called = true
			return nil
		})
	assert.That(t, reached).True()
	assert.That(t, called).False()
	assert.That(t, errors.Is(err, refused)).True()
}
