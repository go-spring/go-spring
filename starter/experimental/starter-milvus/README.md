# starter-milvus

[English](README.md) | [中文](README_CN.md)

`starter-milvus` provides [Milvus](https://milvus.io) (vector database) support:
multi-instance `client.Client` beans, an optional fail-fast startup probe and a
per-instance health indicator, built on the official
[milvus-sdk-go](https://github.com/milvus-io/milvus-sdk-go) v2 (gRPC).

## Installation

```bash
go get go-spring.org/starter-milvus
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-milvus"
```

### 2. Configure

```properties
spring.milvus.instances.a.addr=127.0.0.1:19530
spring.milvus.instances.a.database=default
```

### 3. Inject

```go
type Service struct {
    Client *StarterMilvus.Client `autowire:"a"`
}
```

### 4. Use

```go
err := s.Client.NewCollection(ctx, "docs", 768)
_, err = s.Client.Insert(ctx, "docs", "", idCol, vecCol)
```

The wrapper embeds the SDK's `client.Client` interface, so every method
(collections / insert / search / index / partitions / …) is promoted as-is.

## Core Features

- **Multi-instance clients** — one bean per `spring.milvus.instances.<name>` entry.
- **Fail-fast probe + health indicator** — `HealthCheck` (`ListCollections`) verifies
  connectivity and credentials at startup (opt-in via `ping=true`; off by default),
  doubling as the readiness probe (skip with `health=false`).
- **Transparent per-RPC resilience** — the governance guard is installed on the SDK's
  gRPC dial options (unary + stream interceptors), so rate-limit / breaker / bulkhead /
  retry / timeout apply to every RPC with no call-site change; governance is consumed
  through the shared `spring.governance.*` rules.

## Observability

The starter only **declares** what each RPC is (`observe.go`: the span name, the
`db.client` metric prefix, the `db.system`/`db.operation` labels, and the full method
path as span/log detail); the **resilience layer emits** — the one point on the
executor chain that sees a whole call, retries included. So every call reports a
call-level `db.client.operation.duration` histogram, an attempt-level
`db.client.attempt.duration` histogram, and an access log tagged `_app_milvus_access`.
Disabling governance stops protection, not observability; metrics additionally need
`starter-otel` to install providers.

## Advanced Features

**Multiple clients** — more entries, each with its own connection.

## Design Notes

* **Schema and search semantics belong to the application.** The starter owns only
  per-instance wiring, the per-RPC guard, the fail-fast probe, the health indicator
  and teardown; collection/field/index design and search/query semantics are yours.
* **gRPC-only.** There is no HTTP transport to swap.
* **No dial-option escape hatch.** `Config` exposes no way to append custom dial
  options, so an app needing extra interceptors (auth tokens, custom tracing) must
  fork `newClient`.
