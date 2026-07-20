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

package batch_test

import (
	"context"
	"errors"
	"testing"

	"go-spring.org/spring/batch"
	"go-spring.org/stdlib/testing/assert"
)

func TestJob_NilRepository(t *testing.T) {
	job := &batch.Job{Name: "j", Steps: []batch.Step{batch.Func("s", func(context.Context) error { return nil })}}
	_, err := job.Run(context.Background(), nil, nil)
	assert.Error(t, err).Is(batch.ErrNoRepository)
}

func TestJob_StepsRunInOrder(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	var order []string
	job := &batch.Job{Name: "ordered", Steps: []batch.Step{
		batch.Func("first", func(context.Context) error { order = append(order, "first"); return nil }),
		batch.Func("second", func(context.Context) error { order = append(order, "second"); return nil }),
	}}

	je, err := job.Run(ctx, repo, nil)
	assert.Error(t, err).Nil()
	assert.That(t, je.Status).Equal(batch.StatusCompleted)
	assert.That(t, order).Equal([]string{"first", "second"})
}

// TestJob_RestartSkipsCompletedSteps verifies that on restart a job does not
// re-run steps that already completed; it resumes at the failed step.
func TestJob_RestartSkipsCompletedSteps(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	var firstRuns, secondRuns int
	failSecond := true

	build := func() *batch.Job {
		return &batch.Job{Name: "resume", Steps: []batch.Step{
			batch.Func("first", func(context.Context) error { firstRuns++; return nil }),
			batch.Func("second", func(context.Context) error {
				secondRuns++
				if failSecond {
					return errors.New("boom")
				}
				return nil
			}),
		}}
	}

	// Run 1: first completes, second fails.
	_, err := build().Run(ctx, repo, nil)
	assert.Error(t, err).NotNil()
	assert.That(t, firstRuns).Equal(1)
	assert.That(t, secondRuns).Equal(1)

	// Run 2: first is skipped (already completed), only second re-runs.
	failSecond = false
	je, err := build().Run(ctx, repo, nil)
	assert.Error(t, err).Nil()
	assert.That(t, je.Status).Equal(batch.StatusCompleted)
	assert.That(t, firstRuns).Equal(1, "completed step not re-run on restart")
	assert.That(t, secondRuns).Equal(2)
}

func TestFunc_PanicsOnBadInput(t *testing.T) {
	assert.Panic(t, func() { batch.Func("", func(context.Context) error { return nil }) }, "name must not be empty")
	assert.Panic(t, func() { batch.Func("x", nil) }, "function must not be nil")
}
