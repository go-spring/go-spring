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

package batch

import (
	"context"

	"go-spring.org/spring/resilience"
)

// DefaultChunkSize is used when [ChunkStep.ChunkSize] is not set.
const DefaultChunkSize = 100

// ChunkStep is a restartable, chunk-oriented [Step]: it reads items in batches
// of ChunkSize, processes each, writes the batch, then commits progress to the
// repository. The commit is the restart boundary — after a crash the step
// resumes from the last committed checkpoint and does not re-read committed
// chunks. See the package doc for the at-least-once/idempotency contract.
type ChunkStep[I, O any] struct {
	// Name identifies the step within its job. It must be non-empty and unique.
	Name string

	// Reader produces input items. Required. If it implements [Checkpointer] the
	// step resumes from the persisted position on restart; otherwise it replays
	// from the beginning.
	Reader Reader[I]

	// Processor transforms each read item into a written item. Optional: when I
	// and O are the same type, leave it nil and set a [Passthrough] via
	// [WithPassthrough], or supply your own. When Processor is nil the step
	// requires I == O and passes items through unchanged.
	Processor Processor[I, O]

	// Writer persists each processed chunk. Required. For exactly-once behaviour
	// across a restart it should be idempotent (see the package doc).
	Writer Writer[O]

	// ChunkSize is how many items are read, processed and written per commit.
	// Defaults to [DefaultChunkSize] when not positive.
	ChunkSize int

	// Retry, when any field is set, wraps each chunk (read+process+write) in a
	// retry/circuit-breaker via the resilience default driver, so a transient
	// failure retries the chunk instead of failing the step. Ignored when
	// Executor is set.
	Retry resilience.Policy

	// Executor, when set, is used to guard each chunk instead of building one
	// from Retry — plug in a production driver (e.g. sentinel) here. When both
	// Executor and Retry are unset, a chunk runs once with no retry.
	Executor resilience.Executor
}

// StepName implements [Step].
func (s *ChunkStep[I, O]) StepName() string { return s.Name }

// executor returns the resilience executor guarding each chunk, or nil when no
// protection is configured (the chunk then runs once).
func (s *ChunkStep[I, O]) executor() (resilience.Executor, error) {
	if s.Executor != nil {
		return s.Executor, nil
	}
	if s.Retry == (resilience.Policy{}) {
		return nil, nil
	}
	d, err := resilience.MustGetDriver("default")
	if err != nil {
		return nil, err
	}
	return d.NewExecutor(s.Retry)
}

// Run implements [Step]. It resumes from rc.StepExecution, drives the chunk
// loop, and commits after each chunk. It honours ctx: on cancellation it stops
// promptly leaving the step in StatusStopped so a later restart resumes.
func (s *ChunkStep[I, O]) Run(ctx context.Context, rc *StepContext) error {
	if s.Reader == nil {
		return ErrNoReader
	}
	if s.Writer == nil {
		return ErrNoWriter
	}
	chunkSize := s.ChunkSize
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	exec, err := s.executor()
	if err != nil {
		return err
	}
	if exec != nil {
		defer func() { _ = exec.Close() }()
	}

	se := rc.StepExecution

	// Resume the reader from the persisted checkpoint (nil on a fresh start).
	if cp, ok := s.Reader.(Checkpointer); ok {
		if err := cp.Open(ctx, se.Checkpoint); err != nil {
			return err
		}
	}
	if cl, ok := s.Reader.(Closer); ok {
		defer func() { _ = cl.Close(ctx) }()
	}
	if cl, ok := s.Writer.(Closer); ok {
		defer func() { _ = cl.Close(ctx) }()
	}

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Read one chunk of raw items. Reads advance the reader exactly once per
		// chunk and are deliberately outside the retry guard: the read items are
		// buffered so a retry replays process+write against the buffer instead of
		// re-reading (a reader cannot re-yield items it already advanced past).
		items := make([]I, 0, chunkSize)
		for len(items) < chunkSize {
			item, ok, err := s.Reader.Read(ctx)
			if err != nil {
				return err
			}
			if !ok {
				break
			}
			items = append(items, item)
		}
		if len(items) == 0 {
			// Source exhausted: nothing more to commit.
			return nil
		}
		read := int64(len(items))

		// Process and write the buffered chunk, guarded so a transient failure
		// retries process+write rather than failing the step. Processing is a
		// pure transform, so replaying it on retry is safe.
		var written int64
		writeChunk := func(ctx context.Context) error {
			out := make([]O, 0, len(items))
			for _, item := range items {
				o, keep, err := s.process(ctx, item)
				if err != nil {
					return err
				}
				if keep {
					out = append(out, o)
				}
			}
			if len(out) == 0 {
				written = 0
				return nil
			}
			if err := s.Writer.Write(ctx, out); err != nil {
				return err
			}
			written = int64(len(out))
			return nil
		}

		if exec != nil {
			err = exec.Execute(ctx, s.Name, writeChunk)
		} else {
			err = writeChunk(ctx)
		}
		if err != nil {
			return err
		}

		// Commit: persist progress and the reader's position. This is the durable
		// boundary — a restart resumes after the last successful commit.
		se.ReadCount += read
		se.WriteCount += written
		if cp, ok := s.Reader.(Checkpointer); ok {
			se.Checkpoint = cp.Checkpoint()
		}
		se.Status = StatusStarted
		if err := rc.Repo.SaveStepExecution(ctx, se); err != nil {
			return err
		}
	}
}

// process applies the processor, or passes the item through when no processor is
// set (which requires I == O; a type mismatch is a programming error surfaced by
// the type assertion).
func (s *ChunkStep[I, O]) process(ctx context.Context, item I) (O, bool, error) {
	if s.Processor != nil {
		return s.Processor.Process(ctx, item)
	}
	out, ok := any(item).(O)
	if !ok {
		var zero O
		return zero, false, errPassthroughType
	}
	return out, true, nil
}

// errPassthroughType is returned when a ChunkStep has no Processor but I is not
// assignable to O, so items cannot pass through unchanged.
var errPassthroughType = &passthroughTypeError{}

type passthroughTypeError struct{}

func (*passthroughTypeError) Error() string {
	return "batch: chunk step has no processor and input type is not assignable to output type"
}
