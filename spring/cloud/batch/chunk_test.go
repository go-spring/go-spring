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
	"strconv"
	"sync"
	"testing"

	"go-spring.org/spring/cloud/batch"
	"go-spring.org/spring/cloud/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// seqReader emits the integers 1..n and is a batch.Checkpointer: its checkpoint
// is the count already read, so Open(cp) resumes just past cp.
type seqReader struct {
	n   int
	pos int
}

func newSeqReader(n int) *seqReader { return &seqReader{n: n} }

func (r *seqReader) Open(_ context.Context, cp batch.Checkpoint) error {
	if len(cp) == 0 {
		r.pos = 0
		return nil
	}
	p, err := strconv.Atoi(string(cp))
	if err != nil {
		return err
	}
	r.pos = p
	return nil
}

func (r *seqReader) Read(_ context.Context) (int, bool, error) {
	if r.pos >= r.n {
		return 0, false, nil
	}
	r.pos++
	return r.pos, true, nil
}

func (r *seqReader) Checkpoint() batch.Checkpoint {
	return batch.Checkpoint(strconv.Itoa(r.pos))
}

// TestChunkStep_RestartNoDoubleProcessing is the acceptance scenario: a chunk
// job crashes mid-run (a write fails after some chunks have committed), then a
// restart against the same repository resumes from the last checkpoint and
// finishes without reprocessing any committed item.
func TestChunkStep_RestartNoDoubleProcessing(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	const total = 50
	var mu sync.Mutex
	recorded := map[int]int{}
	failing := true // first run fails once two chunks have committed

	makeStep := func() batch.Step {
		return &batch.ChunkStep[int, int]{
			Name:      "copy",
			Reader:    newSeqReader(total),
			Processor: batch.Passthrough[int](),
			Writer: batch.WriterFunc[int](func(_ context.Context, items []int) error {
				mu.Lock()
				defer mu.Unlock()
				// Crash atomically before recording, simulating a process dying
				// between committed chunks. 20 items (two chunks of 10) have
				// already committed at this point.
				if failing && len(recorded) >= 20 {
					return errors.New("boom")
				}
				for _, v := range items {
					recorded[v]++
				}
				return nil
			}),
			ChunkSize: 10,
		}
	}

	// Run 1: fails midway.
	job1 := &batch.Job{Name: "copy-job", Steps: []batch.Step{makeStep()}}
	je1, err := job1.Run(ctx, repo, nil)
	assert.Error(t, err).NotNil()
	assert.That(t, je1.Status).Equal(batch.StatusFailed)

	// The step committed exactly two chunks before the crash.
	se, ok, _ := repo.FindStepExecution(ctx, je1.ID, "copy")
	assert.That(t, ok).True("step recorded")
	assert.Number(t, se.ReadCount).Equal(int64(20))
	assert.That(t, string(se.Checkpoint)).Equal("20")

	// Run 2: resumes from the checkpoint and completes.
	failing = false
	job2 := &batch.Job{Name: "copy-job", Steps: []batch.Step{makeStep()}}
	je2, err := job2.Run(ctx, repo, nil)
	assert.Error(t, err).Nil()
	assert.That(t, je2.ID).Equal(je1.ID) // same instance resumed
	assert.That(t, je2.Status).Equal(batch.StatusCompleted)

	// Every item was written exactly once — no committed chunk reprocessed.
	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(recorded)).Equal(total)
	for i := 1; i <= total; i++ {
		assert.Number(t, recorded[i]).Equal(1, "item written exactly once")
	}

	final, _, _ := repo.FindStepExecution(ctx, je2.ID, "copy")
	assert.Number(t, final.ReadCount).Equal(int64(total))
	assert.Number(t, final.WriteCount).Equal(int64(total))
}

// TestChunkStep_RetryRecoversChunk verifies a transient chunk failure is retried
// via the reused resilience policy instead of failing the step.
func TestChunkStep_RetryRecoversChunk(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	var attempts int
	step := &batch.ChunkStep[int, int]{
		Name:      "flaky",
		Reader:    newSeqReader(5),
		Processor: batch.Passthrough[int](),
		Writer: batch.WriterFunc[int](func(_ context.Context, _ []int) error {
			attempts++
			if attempts < 3 { // fail the first two attempts of the first chunk
				return errors.New("transient")
			}
			return nil
		}),
		ChunkSize: 5,
		Retry:     resilience.Policy{MaxRetries: 3},
	}

	job := &batch.Job{Name: "flaky-job", Steps: []batch.Step{step}}
	je, err := job.Run(ctx, repo, nil)
	assert.Error(t, err).Nil()
	assert.That(t, je.Status).Equal(batch.StatusCompleted)
	assert.That(t, attempts >= 3).True("chunk retried until it succeeded")
}
