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

// Package batch defines a framework-agnostic, zero-dependency abstraction for
// batch processing and short-lived tasks — the Go-idiomatic equivalent of
// Spring Batch and Spring Cloud Task.
//
// It answers one question for bulk work (reconciliation, import/export, data
// cleansing, ...): "process a large data set in restartable chunks, persist how
// far I got, and — if the process dies mid-run — resume from where I stopped
// without reprocessing what already committed." Rather than reproducing Spring
// Batch's XML/annotation DSL, a job is plain Go: typed [Reader]/[Processor]/
// [Writer] interfaces composed into a [ChunkStep], one or more steps composed
// into a [Job], and a [JobRepository] that records progress.
//
// Two layers sit on the same [JobRepository]:
//
//   - Chunk processing (Spring Batch): a [ChunkStep] reads items in batches of
//     ChunkSize, processes and writes each batch, then commits progress. Commit
//     is the restart boundary — a committed chunk is never re-read.
//   - Short-lived tasks (Spring Cloud Task): [Func] wraps a one-shot function as
//     a single [Step], so a "run once and record the outcome" task is just a
//     one-step [Job].
//
// The abstraction is deliberately split from any backend, exactly like
// [go-spring.org/spring/lock]: the progress store is the [JobRepository] seam.
// A [NewMemoryRepository] ships built in for tests and single-process use; a
// durable backend (Redis, a database, ...) is a separate integration module
// that contributes a JobRepository bean. There is no global driver registry —
// a backend needs a live client (a Redis connection, a *sql.DB), not a
// declarative policy, so the seam is the bean type, following the lock package
// rather than the discovery/resilience string-registry pattern.
//
// Restart and idempotency. Progress is committed after a chunk's write
// succeeds. If the process crashes between the write and the commit, the restart
// replays that one chunk (at-least-once for the writer). The framework
// guarantees that a *committed* chunk is never reprocessed; exactly-once
// therefore requires an idempotent [Writer] (e.g. keyed upserts). This mirrors
// Spring Batch's chunk-commit semantics.
//
// The scope of the first version is single-process chunk processing. Remote
// partitioning (sharding a step across processes) is intentionally out of scope.
package batch

import (
	"context"
	"errors"
)

// Errors returned by the batch engine.
var (
	// ErrNoRepository is returned by [Job.Run] when it is called with a nil
	// [JobRepository]. Progress must be recorded somewhere, so a nil repository
	// is a programming error rather than a silent in-memory fallback.
	ErrNoRepository = errors.New("batch: nil job repository")

	// ErrNoWriter is returned when a [ChunkStep] is run without a Writer, and
	// ErrNoReader when it is run without a Reader — either makes the step unable
	// to do work, so it fails fast rather than looping to no effect.
	ErrNoReader = errors.New("batch: chunk step has no reader")
	ErrNoWriter = errors.New("batch: chunk step has no writer")
)

// BatchStatus is the lifecycle state of a [JobExecution] or [StepExecution].
type BatchStatus int

const (
	// StatusPending is the initial state before a job or step starts running.
	StatusPending BatchStatus = iota
	// StatusStarted means the job or step is currently running.
	StatusStarted
	// StatusCompleted means it finished successfully.
	StatusCompleted
	// StatusFailed means it ended with an error and can be restarted.
	StatusFailed
	// StatusStopped means it was halted (e.g. context cancelled) before finishing
	// and can be restarted.
	StatusStopped
)

// String returns the lower-case status name.
func (s BatchStatus) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusStarted:
		return "started"
	case StatusCompleted:
		return "completed"
	case StatusFailed:
		return "failed"
	case StatusStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// Params identifies a job instance together with the job name. Two runs with the
// same name and params are the same instance: launching the second one resumes
// the first if it did not complete. Params are also passed through to the job so
// a run can be parameterised (date to reconcile, file to import, ...).
type Params map[string]string

// Checkpoint is an opaque, serializable marker of a [Reader]'s position. The
// engine persists it after each committed chunk and hands it back to the reader
// on restart so the reader can resume. Its bytes are meaningful only to the
// reader that produced them (typically small JSON).
type Checkpoint []byte

// Reader produces items one at a time. Read returns ok=false when the source is
// exhausted; a non-nil error aborts the step. A reader that wants to support
// restart also implements [Checkpointer]; a reader that holds resources also
// implements [Closer]. Both are optional so a trivial in-memory reader stays a
// single method.
type Reader[T any] interface {
	Read(ctx context.Context) (item T, ok bool, err error)
}

// Processor transforms a read item into a written item. keep=false filters the
// item out (it is neither written nor counted as written). When no
// transformation is needed use [Passthrough].
type Processor[I, O any] interface {
	Process(ctx context.Context, item I) (out O, keep bool, err error)
}

// Writer persists a chunk of items. It receives the whole chunk at once so a
// backend can batch the write (a single multi-row INSERT, one pipeline, ...).
// A writer that holds resources also implements [Closer].
type Writer[T any] interface {
	Write(ctx context.Context, items []T) error
}

// Checkpointer is the optional restart interface a [Reader] implements to resume
// from a saved position. The engine calls Open once before reading — with the
// checkpoint persisted by the previous run, or nil on a fresh start — and reads
// Checkpoint after each chunk to persist the new position. A reader that does
// not implement it is replayed from the beginning on restart.
type Checkpointer interface {
	// Open positions the reader. cp is nil on a fresh start, or the value last
	// returned by Checkpoint on a restart.
	Open(ctx context.Context, cp Checkpoint) error
	// Checkpoint returns the current position, to be persisted after a chunk
	// commits. It must round-trip through Open.
	Checkpoint() Checkpoint
}

// Closer is the optional interface a [Reader] or [Writer] implements to release
// resources when the step finishes (successfully or not).
type Closer interface {
	Close(ctx context.Context) error
}

// ReaderFunc adapts a function to a [Reader].
type ReaderFunc[T any] func(ctx context.Context) (T, bool, error)

// Read implements [Reader].
func (f ReaderFunc[T]) Read(ctx context.Context) (T, bool, error) { return f(ctx) }

// ProcessorFunc adapts a function to a [Processor].
type ProcessorFunc[I, O any] func(ctx context.Context, item I) (O, bool, error)

// Process implements [Processor].
func (f ProcessorFunc[I, O]) Process(ctx context.Context, item I) (O, bool, error) {
	return f(ctx, item)
}

// WriterFunc adapts a function to a [Writer].
type WriterFunc[T any] func(ctx context.Context, items []T) error

// Write implements [Writer].
func (f WriterFunc[T]) Write(ctx context.Context, items []T) error { return f(ctx, items) }

// Passthrough returns a [Processor] that emits each item unchanged. Use it when
// a [ChunkStep] needs no transformation between reading and writing.
func Passthrough[T any]() Processor[T, T] {
	return ProcessorFunc[T, T](func(_ context.Context, item T) (T, bool, error) {
		return item, true, nil
	})
}
