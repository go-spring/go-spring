# starter-casbin

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-casbin` provides a [Casbin](https://casbin.org) access-control wrapper,
making it easy to build authorization (RBAC/ABAC/ACL) into Go-Spring applications.
The enforcer is registered as a bean and consumed purely by injection.

## Installation

```bash
go get go-spring.org/starter-casbin
```

## Quick Start

### 1. Import the `starter-casbin` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-casbin"
```

### 2. Provide a Model and a Policy

Casbin needs a [model file](example/conf/model.conf) (the matching rules) and a
[policy file](example/conf/policy.csv) (the rules themselves). Then declare an
enforcer instance in your [configuration file](example/conf/app.properties):

```properties
spring.casbin.rbac.model=./conf/model.conf
spring.casbin.rbac.policy=./conf/policy.csv
```

The last key segment (`rbac`) is the bean name.

### 3. Inject the Enforcer

Refer to the [example.go](example/example.go) file. Bind by the instance name.
The bean is a `*StarterCasbin.Enforcer` that embeds `*casbin.Enforcer`, so all the
usual methods (`Enforce`, `AddPolicy`, ...) are available directly.

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
| `policy`   | Path to the file-backed policy; ignored when `adapter` is set               | —       |
| `autoSave` | Persist policy mutations back to the storage                                | `true`  |
| `adapter`  | Name of a `persist.Adapter` registered via `RegisterAdapter` (DB/file/...)  | —       |
| `watcher`  | Name of a `persist.Watcher` registered via `RegisterWatcher` (hot reload)   | —       |

## Core Features

The [example.go](example/example.go) program builds an RBAC enforcer and asserts:

* **role inheritance** — `alice` (admin) may `read`/`write`, `bob` (viewer) may only `read`.
* **default deny** — unknown subjects and unlisted actions are rejected.
* **hot reload** — a peer appends a grant and signals the watcher; the enforcer reloads
  its policy and the new grant takes effect without a restart.

## Advanced Features

* **Multiple enforcers**: define several instances under `spring.casbin.*` (one per
  domain) and inject each by its bean name.
* **Pluggable persistence**: the default file adapter keeps this starter database-free.
  To back policies with GORM, Redis, etc., register a [Casbin adapter](https://casbin.org/docs/adapters)
  by name during bootstrap and point the instance at it:

  ```go
  func init() {
      StarterCasbin.RegisterAdapter("gorm", gormAdapter)
  }
  ```
  ```properties
  spring.casbin.rbac.adapter=gorm
  ```

  The starter stays free of any storage driver on purpose — registering the adapter in
  your application avoids dragging GORM/Redis/etcd into projects that only need the file
  policy. The gs.Group factory cannot inject other beans, so adapters/watchers are looked
  up by name from a package-level registry.
* **Hot reload / multi-instance sync**: register a [Casbin watcher](https://casbin.org/docs/watchers)
  with `RegisterWatcher` and set `spring.casbin.<inst>.watcher=<name>`. When a peer signals
  a policy change, the enforcer automatically calls `LoadPolicy`. The watcher's background
  resources are released on shutdown via the starter's destroy callback.
