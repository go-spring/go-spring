# starter-lock-memory

[English](README.md) | [中文](README_CN.md)

`starter-lock-memory` wires [lock.MemoryLocker](../../../../../cloud/lock/)
into Go-Spring under the shared `spring.lock` prefix, so an application can be
wired exactly like a production backend — config keys, bean injection, observe
wrapping — while running with zero external services. Local development,
demos, and tests of lock-consuming code.

**Not for production multi-replica coordination**: the lock's scope is one
process. Importing it is a statement that this deployment runs a single
instance; for real coordination blank-import starter-lock-redis / -etcd /
-consul / -k8s instead.

## Configuration

| Key | Default | Description |
| --- | --- | --- |
| `spring.lock.instances.memory.<name>.observe.enabled` | `true` | Wrap the locker with the shared observe adapter (span + `lock.operation.*` metrics + access log, system=`memory`). |

Lock timing (TTL, renew, retry) is not configured here: it is carried by the
per-acquire `lock.Option` values, identical across every backend.

```properties
spring.lock.instances.memory.demo.observe.enabled=true
```

```go
type Demo struct {
    Locker lock.Locker `autowire:"memory.demo"`
}
```

See [cloud/lock/example](../../../../../cloud/lock/example/) for a complete
starter-shaped demo over this backend.
