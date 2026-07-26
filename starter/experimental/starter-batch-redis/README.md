# starter-batch-redis

[English](README.md) | [中文](README_CN.md)

`starter-batch-redis` contributes a Redis-backed
[`batch.JobRepository`](../../spring/batch) bean to a Go-Spring application, so
chunk jobs and short-lived tasks persist their progress in Redis and survive a
process restart from the last committed chunk.

It follows the *Contributor* archetype (see
[starter/DESIGN.md](../DESIGN.md)): the starter exports no port and holds no
client of its own. It reuses the `*redis.Client` bean registered by
`starter-go-redis` and contributes a bean behind the framework-neutral
`batch.JobRepository` seam. Switching the repository backend to a SQL database
is a blank-import swap — no business code changes.

## Installation

```bash
go get go-spring.org/starter-batch-redis
```

## Quick Start

### 1. Import the two starters (plus the runner if you use one)

```go
import (
    _ "go-spring.org/starter-go-redis"
    _ "go-spring.org/starter-batch-redis"
    // _ "go-spring.org/starter-batch"        // the batch runner, if used
)
```

### 2. Configure a Redis client, then a JobRepository that references it

```properties
# A Redis client managed by starter-go-redis.
spring.go-redis.cache.addr=127.0.0.1:6379

# A JobRepository bound to that client. `client` is the redis instance name.
spring.batch-repository.jobs.client=cache
spring.batch-repository.jobs.key-prefix=myapp:batch:
spring.batch-repository.jobs.ttl=168h

# The batch runner picks up the repository by name.
spring.batch.repository=jobs
```

The `client` property is **required**. Booting without it fails fast — the
starter refuses to silently default to some arbitrary Redis instance.

### 3. Inject `batch.JobRepository`

```go
import "go-spring.org/spring/cloud/batch"

type Service struct {
    Repo batch.JobRepository `autowire:"jobs"`
}
```

Most application code does not touch `JobRepository` directly — the batch
runner does. Injecting it is useful for progress dashboards and admin tools
that call `ListStepExecutions`.

## Configuration

All keys sit under `spring.batch-repository.<name>`. The prefix is deliberately
distinct from `spring.batch.*`, which the batch runner owns for job / step /
chunk configuration.

| Key          | Default | Description                                                                                     |
|--------------|---------|-------------------------------------------------------------------------------------------------|
| `client`     | —       | **Required.** Name of the `*redis.Client` bean under `spring.go-redis.<client>`.                |
| `key-prefix` | *empty* | Prepended to every key so multiple apps can share a Redis instance without colliding.           |
| `ttl`        | `0`     | Optional `EXPIRE` applied to job / step keys on every write. `0` keeps records forever.         |

## Key layout

For a repository configured with `key-prefix=myapp:batch:`:

| Redis key                            | Type    | Contents                                             |
|--------------------------------------|---------|------------------------------------------------------|
| `myapp:batch:job:<instanceKey>`      | string  | JSON-encoded `batch.JobExecution` for one instance.  |
| `myapp:batch:steps:<jobExecutionID>` | hash    | Field per step name → JSON-encoded `StepExecution`.  |
| `myapp:batch:seq`                    | counter | `INCR` source for monotonic execution IDs.           |

`<instanceKey>` is a SHA-1 of the job name and its sorted params — the same
scheme the in-memory repository uses — so two runs with the same
`(name, params)` share a `JobExecution` and the second run resumes the first
when it did not complete.

## Guarantees

* **Restart across process crashes** — every committed chunk writes the
  `StepExecution` via `HSET`, so restart resumes from the last successful
  commit and never re-reads a committed chunk.
* **Fail-fast configuration** — missing `client` refuses to boot instead of
  surfacing on the first `SaveStepExecution`.
* **Client lifecycle isolation** — the repository never closes the injected
  `*redis.Client`; `starter-go-redis` owns that.
* **TTL bookkeeping** — when `ttl > 0` each write refreshes the key's
  expiration, so a long-running job keeps its records alive for the full
  duration.

## Switching backends

Because the repository is contributed behind the `batch.JobRepository`
interface, dropping in a different backend is a blank-import swap and a
config-prefix change — the batch runner references the repository by name via
`spring.batch.repository`, and all application code stays put.
