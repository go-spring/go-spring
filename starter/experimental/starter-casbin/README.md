# starter-casbin

[English](README.md) | [中文](README_CN.md)

`starter-casbin` provides a [Casbin](https://casbin.org) access-control wrapper for
Go-Spring applications. The enforcer is registered as a bean and consumed by
injection.

## Installation

```bash
go get go-spring.org/starter-casbin
```

## Quick Start

### 1. Import the `starter-casbin` Package

```go
import _ "go-spring.org/starter-casbin"
```

### 2. Provide a Model and a Policy

Casbin needs a [model file](example/conf/model.conf) (the matching rules) and a
[policy file](example/conf/policy.csv) (the rules themselves). Then declare an enforcer instance in your
[configuration file](example/conf/app.properties):

```properties
spring.casbin.instances.rbac.model=./conf/model.conf
spring.casbin.instances.rbac.policy=./conf/policy.csv
```

The last key segment (`rbac`) is the bean name.

### 3. Inject the Enforcer

```go
import StarterCasbin "go-spring.org/starter-casbin"

type Service struct {
    Enforcer *StarterCasbin.Enforcer `autowire:"rbac"`
}
```

### 4. Enforce a Request

```go
ok, err := s.Enforcer.Enforce("alice", "/data", "write")
```

## Configuration

| Key        | Description                                                                 | Default |
|------------|-----------------------------------------------------------------------------|---------|
| `model`    | Path to the Casbin model file                                               | —       |
| `policy`   | Path to the file-backed policy; mutually exclusive with `adapter`          | —       |
| `autoSave` | Persist policy mutations back to the storage                                | `true`  |
| `adapter`  | Bean name of the `persist.Adapter` this enforcer uses (DB/file/...); empty → file policy | — |
| `watcher`  | Bean name of the `persist.Watcher` enabling hot reload; empty → none                  | — |

## Core Features

The [example.go](example/example.go) program builds an RBAC enforcer and asserts:

* **role inheritance** — `alice` (admin) may `read`/`write`, `bob` (viewer) may only `read`.
* **default deny** — unknown subjects and unlisted actions are rejected.
* **hot reload** — a peer appends a grant and signals the watcher; the enforcer reloads
  its policy and the new grant takes effect without a restart.

## Advanced Features

* **Multiple enforcers**: define several instances under `spring.casbin.instances.*` (one per
  domain) and inject each by its bean name.
* **Pluggable persistence**: the default file adapter keeps this starter database-free.
  To back policies with GORM, Redis, etc., provide a [Casbin adapter](https://casbin.org/docs/adapters)
  as an ordinary bean whose name the instance points at:

  ```go
  func init() {
      gs.Provide(func() persist.Adapter { return gormAdapter }).
          Name("gorm").Export(gs.As[persist.Adapter]())
  }
  ```
  ```properties
  spring.casbin.instances.rbac.adapter=gorm
  ```

  The starter stays free of any storage driver on purpose — the adapter lives in your
  application, so projects that only need the file policy drag in no GORM/Redis/etcd. A
  per-instance `gs.Module` injects the named adapter/watcher bean into each enforcer, so
  there is no package-level registry.
* **Hot reload / multi-instance sync**: provide a [Casbin watcher](https://casbin.org/docs/watchers)
  as a bean named by `spring.casbin.instances.<inst>.watcher`. When a peer signals a policy change,
  the enforcer automatically calls `LoadPolicy`. The watcher's background resources are
  released on shutdown via the starter's destroy callback.
