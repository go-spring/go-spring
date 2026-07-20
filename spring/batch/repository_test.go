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
	"testing"

	"go-spring.org/spring/batch"
	"go-spring.org/stdlib/testing/assert"
)

func TestMemoryRepository_ObtainExecution(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	// First obtain for an instance is a fresh execution.
	je1, restart, err := repo.ObtainExecution(ctx, "job", batch.Params{"date": "2026-07-19"})
	assert.Error(t, err).Nil()
	assert.That(t, restart).False("first obtain is not a restart")
	assert.That(t, je1.JobName).Equal("job")

	// While it is not completed, the same instance resumes the same execution.
	je2, restart, err := repo.ObtainExecution(ctx, "job", batch.Params{"date": "2026-07-19"})
	assert.Error(t, err).Nil()
	assert.That(t, restart).True("uncompleted instance resumes")
	assert.That(t, je2.ID).Equal(je1.ID)

	// A different params value is a different instance → fresh execution.
	je3, restart, err := repo.ObtainExecution(ctx, "job", batch.Params{"date": "2026-07-20"})
	assert.Error(t, err).Nil()
	assert.That(t, restart).False("different params is a new instance")
	assert.That(t, je3.ID == je1.ID).False("new instance has a different id")

	// Once completed, the instance re-runs as a new execution.
	je1.Status = batch.StatusCompleted
	assert.Error(t, repo.SaveJobExecution(ctx, je1)).Nil()
	je4, restart, err := repo.ObtainExecution(ctx, "job", batch.Params{"date": "2026-07-19"})
	assert.Error(t, err).Nil()
	assert.That(t, restart).False("completed instance re-runs fresh")
	assert.That(t, je4.ID == je1.ID).False("re-run gets a new execution id")
}

func TestMemoryRepository_StepExecutions(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()
	je, _, _ := repo.ObtainExecution(ctx, "job", nil)

	// Not found before it is saved.
	_, ok, err := repo.FindStepExecution(ctx, je.ID, "step1")
	assert.Error(t, err).Nil()
	assert.That(t, ok).False("step not found before save")

	se := &batch.StepExecution{JobExecutionID: je.ID, StepName: "step1", ReadCount: 5, WriteCount: 5, Checkpoint: batch.Checkpoint("pos-5")}
	assert.Error(t, repo.SaveStepExecution(ctx, se)).Nil()

	got, ok, err := repo.FindStepExecution(ctx, je.ID, "step1")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True("step found after save")
	assert.Number(t, got.ReadCount).Equal(int64(5))
	assert.That(t, string(got.Checkpoint)).Equal("pos-5")

	// A returned pointer must not alias the stored state.
	got.ReadCount = 999
	again, _, _ := repo.FindStepExecution(ctx, je.ID, "step1")
	assert.Number(t, again.ReadCount).Equal(int64(5))

	list, err := repo.ListStepExecutions(ctx, je.ID)
	assert.Error(t, err).Nil()
	assert.That(t, len(list)).Equal(1)
}
