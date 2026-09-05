# starter-asynq Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A Client-archetype starter for [Asynq](https://github.com/hibiken/asynq), the
Redis-backed task queue. One instance yields a producer Client (enqueue) and,
opt-in, a worker Server (dequeue + run).

## 1. Responsibilities & Boundaries

- **Owns**: per-instance `spring.asynq.<name>` wiring, the Redis connection
  (shared by both roles), the guarded/observed enqueue path, the worker
  lifecycle (gs.Server), the handler mux, and the health indicator.
- **Does not own**: task serialization (payload is the app's `[]byte`),
  retry/queue policy (asynq's Options pass through), and scheduling (cron
  tasks are asynq's own, out of scope for this starter).

## 2. Key Abstractions & Seams

- **Two roles, one Config** — `Client` (always wired) and `Server` (only when
  `server.enabled=true`). The worker is opt-in: a long-running consumer is an
  explicit operator decision, matching the no-autoconfig-exclude stance
  (see spring/DESIGN.md §4).
- **`Client.Enqueue` guard** — the synchronous enqueue touches Redis and is
  the overload-sensitive path; it routes through the neutral
  `resilience.ExecutorFor` / `fault.InjectorFor` seams and the starter's own
  producer observation (observe.go), degrading to `Client.EnqueueContext`
  when governance is off.
- **`Server` implements `gs.Server`, not `gs.Runner`** — this is the
  load-bearing seam. gs.Runner is a startup-time, must-not-block interface
  (migrations, cache warm); gs.Server is the long-running, gracefully-stopped
  service interface. Using gs.Runner blocked startup and broke signal
  handling. `Server.Run` uses asynq's `Start` + wait-on-ctx, deliberately NOT
  `asynq.Server.Run`, whose `waitForSignals` installs its own signal handler
  that races gs's graceful shutdown.
- **Handler mux is lazily built** — `RegisterHandler` may run before `Init`
  (the app wires it from its own Init), so the mux is created on first use and
  shared with Init/Run.

## 3. Constraints

- `Queues` needs a `:=` default (an absent `spring.asynq.<n>.queues` key must
  bind to nil, not fail the module).
- asynq recovers handler panics itself (its processor guard); this starter
  deliberately does not double-wrap — the boundary is documented rather than
  enforced twice.

## 4. Trade-offs / Alternatives Rejected

- **asynq.Server.Run vs Start+ctx**: Run's built-in signal handling conflicts
  with gs's; rejected in favour of Start + wait-on-ctx.
- **gs.Runner vs gs.Server**: Runner blocks startup; Server is the correct
  long-running seam. This was the single hardest bug in the batch and is
  recorded here so no future starter repeats it.
