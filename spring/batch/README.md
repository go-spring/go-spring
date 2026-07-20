# batch
[English](README.md) | [中文](README_CN.md)

`batch` is a zero-dependency abstraction for batch processing and short-lived
tasks — the Go-idiomatic equivalent of Spring Batch and Spring Cloud Task. A
[`Job`](job.go) is an ordered list of [`Step`](job.go)s; a [`ChunkStep`](chunk.go)
reads/processes/writes items in restartable chunks; a [`JobRepository`](repository.go)
persists progress so a crashed run resumes from the last committed chunk.

## Features

- Typed generic `Reader[T]` / `Processor[I,O]` / `Writer[T]` — plain interfaces,
  no XML, no annotations.
- `ChunkStep[I, O]` — reads a chunk, processes it, writes it, commits progress;
  a committed chunk is never re-read on restart.
- `Func(name, fn)` — wraps a one-shot function as a single-step Cloud Task job.
- `JobRepository` seam with an in-process `NewMemoryRepository()` for tests;
  durable backends contribute a bean (see `starter-batch-redis`).
- Optional `Checkpointer` on a reader to persist its resume position; optional
  `Closer` on reader/writer for resource cleanup.
- Optional resilience: set `ChunkStep.Retry` or plug in a `resilience.Executor`
  to wrap each chunk with retry / circuit-breaker.

## Usage

```go
package main

import (
    "context"
    "fmt"

    "go-spring.org/spring/batch"
)

func main() {
    items := []int{1, 2, 3, 4, 5}
    i := 0

    step := &batch.ChunkStep[int, int]{
        Name:      "double",
        ChunkSize: 2,
        Reader: batch.ReaderFunc[int](func(ctx context.Context) (int, bool, error) {
            if i >= len(items) {
                return 0, false, nil
            }
            v := items[i]
            i++
            return v, true, nil
        }),
        Processor: batch.ProcessorFunc[int, int](func(_ context.Context, v int) (int, bool, error) {
            return v * 2, true, nil
        }),
        Writer: batch.WriterFunc[int](func(_ context.Context, out []int) error {
            fmt.Println("wrote:", out)
            return nil
        }),
    }

    job := &batch.Job{Name: "doubler", Steps: []batch.Step{step}}
    _, _ = job.Run(context.Background(), batch.NewMemoryRepository(), batch.Params{"date": "2026-07-19"})
}
```

## Restart Semantics

- Progress commits AFTER a chunk's write succeeds. A crash between write and
  commit replays that chunk on restart — writers must be idempotent for
  exactly-once effect.
- A completed step is skipped on restart; an interrupted step resumes from its
  persisted `Checkpoint` (if the reader implements `Checkpointer`).
- Two runs with the same `(job name, params)` are the same instance; the second
  call resumes the first.
