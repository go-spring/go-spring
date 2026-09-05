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
	"sync"
	"testing"

	"go-spring.org/cloud/experimental/batch"
	"go-spring.org/cloud/governance/resilience"
	"go-spring.org/stdlib/testing/assert"
)

// TestChunkStep_ContextCancelMarksStoppedAndResumes verifies that cancelling
// the run context mid-step leaves the step (and job) in StatusStopped — not
// Failed — and that a restart resumes from the last committed checkpoint
// without reprocessing committed items.
func TestChunkStep_ContextCancelMarksStoppedAndResumes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := batch.NewMemoryRepository()

	const total = 30
	var mu sync.Mutex
	recorded := map[int]int{}

	// The writer commits chunk 1 (10 items) then stalls until cancelled.
	writerStalled := make(chan struct{})
	var stallOnce sync.Once
	stopped := false
	makeStep := func() batch.Step {
		return &batch.ChunkStep[int, int]{
			Name:      "copy",
			Reader:    newSeqReader(total),
			Processor: batch.Passthrough[int](),
			Writer: batch.WriterFunc[int](func(wctx context.Context, items []int) error {
				if stopped { // restart run: write normally
					mu.Lock()
					defer mu.Unlock()
					for _, v := range items {
						recorded[v]++
					}
					return nil
				}
				mu.Lock()
				n := len(recorded)
				mu.Unlock()
				if n >= 10 {
					stallOnce.Do(func() { close(writerStalled) })
					<-wctx.Done() // stall until the engine cancels the run
					return wctx.Err()
				}
				mu.Lock()
				defer mu.Unlock()
				for _, v := range items {
					recorded[v]++
				}
				return nil
			}),
			ChunkSize: 10,
		}
	}

	// Run 1: cancelled while the second chunk is stalled.
	job1 := &batch.Job{Name: "stop-job", Steps: []batch.Step{makeStep()}}
	go func() {
		<-writerStalled
		cancel()
	}()
	je1, err := job1.Run(ctx, repo, nil)
	assert.Error(t, err).Is(context.Canceled)
	assert.That(t, je1.Status).Equal(batch.StatusStopped)

	se, ok, err := repo.FindStepExecution(context.Background(), je1.ID, "copy")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	assert.That(t, se.Status).Equal(batch.StatusStopped)
	assert.Number(t, se.ReadCount).Equal(int64(10), "one chunk committed before the stop")
	assert.That(t, string(se.Checkpoint)).Equal("10")

	// Run 2: restart against the same instance resumes past the checkpoint.
	stopped = true
	je2, err := (&batch.Job{Name: "stop-job", Steps: []batch.Step{makeStep()}}).
		Run(context.Background(), repo, nil)
	assert.Error(t, err).Nil()
	assert.That(t, je2.ID).Equal(je1.ID)
	assert.That(t, je2.Status).Equal(batch.StatusCompleted)

	mu.Lock()
	defer mu.Unlock()
	assert.That(t, len(recorded)).Equal(total)
	for i := 1; i <= total; i++ {
		assert.Number(t, recorded[i]).Equal(1, "item written exactly once across stop+restart")
	}
}

// TestChunkStep_RetryReplaysBufferWithoutRereading verifies the retry guard
// replays process+write against the buffered chunk: the reader advances exactly
// once per item even when the chunk fails several times before succeeding.
func TestChunkStep_RetryReplaysBufferWithoutRereading(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	reader := &countingReader{inner: newSeqReader(5)}
	attempts := 0
	step := &batch.ChunkStep[int, int]{
		Name:      "flaky",
		Reader:    reader,
		Processor: batch.Passthrough[int](),
		Writer: batch.WriterFunc[int](func(_ context.Context, _ []int) error {
			attempts++
			if attempts < 3 {
				return errors.New("transient")
			}
			return nil
		}),
		ChunkSize: 5,
		Retry:     resilience.Policy{MaxRetries: 3},
	}

	je, err := (&batch.Job{Name: "replay", Steps: []batch.Step{step}}).Run(ctx, repo, nil)
	assert.Error(t, err).Nil()
	assert.That(t, je.Status).Equal(batch.StatusCompleted)
	assert.That(t, attempts).Equal(3)
	assert.Number(t, reader.reads).Equal(int64(5), "reader must not be re-read during a chunk retry")
}

// countingReader wraps a Reader and counts how many items it yielded in total.
type countingReader struct {
	inner *seqReader
	reads int64
}

func (r *countingReader) Open(ctx context.Context, cp batch.Checkpoint) error {
	return r.inner.Open(ctx, cp)
}

func (r *countingReader) Read(ctx context.Context) (int, bool, error) {
	item, ok, err := r.inner.Read(ctx)
	if ok {
		r.reads++
	}
	return item, ok, err
}

func (r *countingReader) Checkpoint() batch.Checkpoint { return r.inner.Checkpoint() }

// TestChunkStep_ReaderWithoutCheckpointerRestartsFromBeginning verifies the
// documented fallback: a reader that does not implement Checkpointer replays
// from the beginning on restart (at-least-once for the writer).
func TestChunkStep_ReaderWithoutCheckpointerRestartsFromBeginning(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	const total = 10
	var writes int
	failing := true
	makeStep := func() batch.Step {
		r := newSeqReader(total) // no Checkpointer: restarts replay it from zero
		return &batch.ChunkStep[int, int]{
			Name:      "replay-all",
			Reader:    batch.ReaderFunc[int](r.Read),
			Processor: batch.Passthrough[int](),
			Writer: batch.WriterFunc[int](func(_ context.Context, items []int) error {
				writes += len(items)
				if failing {
					return errors.New("boom")
				}
				return nil
			}),
			ChunkSize: 10,
		}
	}

	_, err := (&batch.Job{Name: "replay-job", Steps: []batch.Step{makeStep()}}).Run(ctx, repo, nil)
	assert.Error(t, err).NotNil()

	failing = false
	je, err := (&batch.Job{Name: "replay-job", Steps: []batch.Step{makeStep()}}).Run(ctx, repo, nil)
	assert.Error(t, err).Nil()
	assert.That(t, je.Status).Equal(batch.StatusCompleted)
	assert.That(t, writes).Equal(20, "no checkpoint: the restart re-read and re-wrote everything")
}

// TestChunkStep_CloseCalledOnFailure verifies a Reader/Writer implementing
// Closer has Close invoked even when the step fails mid-way.
func TestChunkStep_CloseCalledOnFailure(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	readerClosed, writerClosed := false, false
	step := &batch.ChunkStep[int, int]{
		Name: "closer",
		Reader: &closingReader{
			Reader: batch.ReaderFunc[int](func(context.Context) (int, bool, error) { return 0, false, nil }),
			closed: &readerClosed,
		},
		Writer: &closingWriter{closed: &writerClosed},
	}
	_, err := (&batch.Job{Name: "close-job", Steps: []batch.Step{step}}).Run(ctx, repo, nil)
	assert.Error(t, err).Nil() // empty source is a clean finish, not an error
	assert.That(t, readerClosed).True()
	assert.That(t, writerClosed).True()
}

type closingReader struct {
	batch.Reader[int]
	closed *bool
}

func (r *closingReader) Close(context.Context) error { *r.closed = true; return nil }

type closingWriter struct {
	closed *bool
}

func (c *closingWriter) Write(context.Context, []int) error { return nil }
func (c *closingWriter) Close(context.Context) error        { *c.closed = true; return nil }

// TestJob_StepErrorMarksFailed verifies a non-cancellation step error marks both
// the step and the job StatusFailed with the failure message recorded.
func TestJob_StepErrorMarksFailed(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()
	job := &batch.Job{Name: "failing", Steps: []batch.Step{
		batch.Func("boom", func(context.Context) error { return errors.New("kaput") }),
	}}
	je, err := job.Run(ctx, repo, nil)
	assert.Error(t, err).Matches("kaput")
	assert.That(t, je.Status).Equal(batch.StatusFailed)
	assert.String(t, je.FailureMsg).Equal(`step "boom": kaput`)

	se, ok, err := repo.FindStepExecution(ctx, je.ID, "boom")
	assert.Error(t, err).Nil()
	assert.That(t, ok).True()
	assert.That(t, se.Status).Equal(batch.StatusFailed)
	assert.String(t, se.FailureMsg).Equal("kaput")
}

// TestChunkStep_ProcessorPanicPropagates documents the current contract: unlike
// the scheduling package, the batch engine does not guard a Processor against
// panics — a panic propagates to the caller of Job.Run. Callers that need
// isolation recover at their own boundary.
func TestChunkStep_ProcessorPanicPropagates(t *testing.T) {
	repo := batch.NewMemoryRepository()
	step := &batch.ChunkStep[int, int]{
		Name:   "panic",
		Reader: newSeqReader(3),
		Processor: batch.ProcessorFunc[int, int](func(context.Context, int) (int, bool, error) {
			panic("processor exploded")
		}),
		Writer:    batch.WriterFunc[int](func(context.Context, []int) error { return nil }),
		ChunkSize: 3,
	}

	called := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				called = true
				assert.String(t, r.(string)).Equal("processor exploded")
			}
		}()
		_, _ = (&batch.Job{Name: "panic-job", Steps: []batch.Step{step}}).Run(context.Background(), repo, nil)
	}()
	assert.That(t, called).True("processor panic must propagate to the caller")
}

// TestMemoryRepository_InstanceKeyIgnoresParamOrder verifies that two params
// maps with the same entries in different order identify the same instance.
func TestMemoryRepository_InstanceKeyIgnoresParamOrder(t *testing.T) {
	ctx := context.Background()
	repo := batch.NewMemoryRepository()

	je1, restart, err := repo.ObtainExecution(ctx, "job", batch.Params{"a": "1", "b": "2"})
	assert.Error(t, err).Nil()
	assert.That(t, restart).False()

	je2, restart, err := repo.ObtainExecution(ctx, "job", batch.Params{"b": "2", "a": "1"})
	assert.Error(t, err).Nil()
	assert.That(t, restart).True("same entries in a different order is the same instance")
	assert.That(t, je2.ID).Equal(je1.ID)
}
