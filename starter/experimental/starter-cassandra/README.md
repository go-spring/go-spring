# starter-cassandra

[English](README.md) | [中文](README_CN.md)

`starter-cassandra` provides Cassandra / ScyllaDB support for Go-Spring on
the official [gocql](https://github.com/gocql/gocql) driver (both speak the
CQL native protocol): multi-instance session beans with fail-fast startup
probes, a resilience-guarded `Exec` helper, per-instance health indicators,
optional PasswordAuthenticator and TLS.

## Installation

```bash
go get go-spring.org/starter-cassandra
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-cassandra"
```

### 2. Configure

```properties
spring.cassandra.instances.a.hosts=127.0.0.1
spring.cassandra.instances.a.keyspace=demo
spring.cassandra.instances.a.consistency=local-quorum

# Auth + TLS (optional)
# spring.cassandra.instances.a.username=cassandra
# spring.cassandra.instances.a.password=cassandra
# spring.cassandra.instances.a.tls.enabled=true
# spring.cassandra.instances.a.tls.ca-file=/etc/certs/ca.pem
```

### 3. Inject

```go
type Service struct {
    Client *StarterCassandra.Client `autowire:"a"`
}
```

### 4. Use

```go
// Guarded path: resilience (rate limit / circuit breaking) + observation
err := s.Client.Exec(ctx, "INSERT INTO demo.greetings (id, message) VALUES (?, ?)", 1, "hello")

// Full query power through the embedded session
var msg string
err = s.Client.Query("SELECT message FROM demo.greetings WHERE id = ?", 1).
    WithContext(ctx).Scan(&msg)
```

## Core Features

- **Multi-instance clients** — every `spring.cassandra.instances.<name>` entry is its
  own bean with independent settings.
- **Fail-fast startup probe + health indicator** — a `system.local` scan at
  boot and a `cassandra:<name>` indicator for `starter-actuator`.
- **Guarded Exec** — synchronous statements route through the governance
  executor; iterator/paging queries use the embedded session directly
  (unguarded by design, like the MQ starters' async paths).
- **Cluster discovery** — the contact-point list bootstraps the driver's own
  topology discovery; entries may carry ports (`host:9042`).

## Advanced Features

**Multiple clients** — configure additional entries and inject by name:

```properties
spring.cassandra.instances.main.hosts=10.0.0.1,10.0.0.2
spring.cassandra.instances.main.keyspace=prod
spring.cassandra.instances.analytics.hosts=10.0.1.1
```

**Custom driver** — replace session assembly (e.g. to pin a
HostSelectionPolicy or shard-aware Scylla driver) by providing your own
`Driver` as an optional container bean. Every client under `spring.cassandra`
is built through it; when none is present the starter falls back to its bundled
`DefaultDriver`:

```go
func init() {
    gs.Provide(func() StarterCassandra.Driver { return scyllaDriver{} })
}
```
