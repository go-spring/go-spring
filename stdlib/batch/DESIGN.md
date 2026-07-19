# batch Design
[English](DESIGN.md) | [中文](DESIGN_CN.md)

`batch` is the zero-dependency abstraction for batch processing and short-lived
tasks in the stdlib layer. It gives Go the *effect* of Spring Batch + Spring
Cloud Task — restartable chunk processing and one-shot task recording — without
copying the XML/annotation DSL. A backend (Redis, a database) is contributed as
a separate starter that supplies a `JobRepository` bean.

## 1. Responsibilities and Boundaries

- Model bulk work as `Reader` -> `Processor` -> `Writer` composed inside a
  `ChunkStep`, driven by a `Job` that owns the step order and instance identity.
- Persist progress via `JobRepository` so a crash + restart resumes from the
  last committed chunk instead of reprocessing from zero.
- Fold short-lived tasks into the same shape via `Func(name, fn)` — a one-step
  job whose outcome is recorded in the same repository.
- Refuse to be a job scheduler, a distributed executor, or a message consumer.
  Triggering a job (cron, HTTP, message) is the caller's problem; remote
  partitioning across processes is intentionally out of scope for v1.

## 2. Key Abstractions and Seams

- **`JobRepository` interface as the backend seam.** There is no global driver
  registry. A backend needs a live client (a Redis conn, a `*sql.DB`), not a
  declarative policy, so the seam is the bean type — the same choice
  `stdlib/lock` makes. `NewMemoryRepository()` ships for tests and single-shot
  runs; a durable starter contributes a real bean.
- **Instance identity = `(Name, Params)`.** `ObtainExecution` derives a stable
  `instanceKey` (sha1 of sorted params) so re-running with the same params
  resumes the prior execution; changing any param starts a new instance.
- **Optional `Checkpointer` on the reader.** A reader that can resume implements
  it; the engine hands back the last committed `Checkpoint` on `Open` and reads
  a new one after each committed chunk. Readers that cannot resume simply
  replay from the beginning on restart.
- **Read is outside the retry guard; process+write is inside.** Reads advance
  once per chunk into a buffer; a `resilience.Executor` (built from
  `ChunkStep.Retry` or supplied directly) wraps process+write of the buffered
  chunk. A reader cannot re-yield items it already advanced past, so retrying
  the read would corrupt state.
- **`Step` interface erases generics** so a `Job` can hold steps of differing
  item types in one slice.

## 3. Constraints

- **Commit is the durable boundary.** Progress is persisted after a chunk's
  write succeeds. A crash between write and commit replays that one chunk on
  restart, so the writer must be idempotent (keyed upserts, dedup key) for
  exactly-once behaviour — the framework guarantees at-least-once only.
- **Nil repository fails fast.** `Job.Run` returns `ErrNoRepository` on nil
  rather than silently swallowing progress; the same principle applies to a
  `ChunkStep` with no `Reader` or no `Writer` (`ErrNoReader` / `ErrNoWriter`).
- **Context cancellation is a clean stop, not a failure.** A step interrupted
  by `ctx.Err() != nil` is recorded as `StatusStopped` (not `StatusFailed`), so
  a deliberate shutdown is restartable and telemetry does not treat it as an
  incident.
- **Repository implementations must be safe for concurrent use** and must
  return deep-enough copies so callers cannot mutate stored state via a
  returned pointer (see `cloneJob`/`cloneStep`).

## 4. Trade-offs and Alternatives Rejected

- **No driver-string registry.** Batch is not a config-time choice like
  `discovery` or `resilience`; you cannot express "the backend I want" without
  a live client. Bean-type seam beats registry indirection here.
- **No XML/annotation DSL.** Job wiring is plain Go generics — the value of
  Spring Batch is its restart semantics, not its DSL.
- **No remote partitioning in v1.** Sharding a step across processes changes
  the checkpoint model (per-partition, coordination on completion) and would
  bake distributed coordination into stdlib. It stays out until a concrete
  use case forces the design.
- **Reader replay-on-retry over read-inside-retry.** A read side-effect that
  a retry cannot undo (offset advance, cursor move) makes retrying the read
  broken by construction; buffering the chunk keeps the retry semantics
  simple and safe.
