# starter-xxljob Design

[English](DESIGN.md) | [中文](DESIGN_CN.md)

A hand-rolled xxl-job executor starter: it speaks the executor half of the
xxl-job protocol (registry/heartbeat/run/kill/log) over plain HTTP, with no
third-party SDK.

## 1. Responsibilities & Boundaries

- **Owns**: the callback HTTP server (gs.Server) serving /run /beat /idleBeat
  /kill /log, the handler registry, task execution (goroutine + cancellable
  context), the admin registration/heartbeat loop, and task log files.
- **Does not own**: job scheduling (that is the admin's job), the admin UI /
  database, and job parameter parsing (the handler gets the raw string).

## 2. Key Abstractions & Seams

- **Executor implements gs.Server** — the callback server is a long-running,
  gracefully-stopped service, not a gs.Runner (the same seam starter-asynq
  learned the hard way; see its DESIGN).
- **Task goroutine + cancellable context** — /run starts the handler on a
  goroutine and registers a cancel keyed by jobId (logId only feeds /api/callback and /log); /kill cancels it. A panic
  is recovered through the shared `goutil.SafeRun` chain, converting to a 500
  callback.
- **Registration loop** — `register()` runs in the Executor.Run lifecycle:
  `registryOnce` POSTs to every admin's /api/registry on the interval, and the
  returned closer de-registers on shutdown.

## 3. Constraints

- Port is required (`expr:"$ > 0"`): the admin must dial back into the
  executor, so the callback port is an explicit operator decision (see the
  starter-server-port-must-be-configured convention).
- The protocol is a deliberate **subset**: registry/heartbeat/run/idleBeat/
  kill/log + callback. Broadcast/sharding (`broadcastIndex`/`broadcastTotal`)
  is parsed but not acted on; GLUE (shell/python) execution is out of scope.

## 4. Trade-offs / Alternatives Rejected

- **Hand-rolled protocol vs xxl-job-executor-go (v1.2.0, 2023-05, dormant)**:
  the community SDK is stale and thin; the wire format is ~5 JSON structs and
  6 endpoints, so owning it costs less than a dormant dependency. Same stance
  as starter-webhook (stdlib-only, protocol is the API).
- **Mock-admin example vs dockerized xxl-job-admin**: the official admin
  needs a MySQL schema and JVM; the protocol under test is the executor's
  side, so a self-contained mock admin exercises it without the orchestration
  (mirroring starter-webhook's self-contained example).
