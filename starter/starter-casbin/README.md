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

```go
import "github.com/casbin/casbin/v2"

type Service struct {
    Enforcer *casbin.Enforcer `autowire:"rbac"`
}
```

### 4. Enforce a Request

```go
ok, err := s.Enforcer.Enforce("alice", "/data", "write")
```

## Configuration

| Key        | Description                                   | Default |
|------------|-----------------------------------------------|---------|
| `model`    | Path to the Casbin model file                 | —       |
| `policy`   | Path to the file-backed policy                | —       |
| `autoSave` | Persist policy mutations back to the CSV file | `true`  |

## Core Features

The [example.go](example/example.go) program builds an RBAC enforcer and asserts:

* **role inheritance** — `alice` (admin) may `read`/`write`, `bob` (viewer) may only `read`.
* **default deny** — unknown subjects and unlisted actions are rejected.

## Advanced Features

* **Multiple enforcers**: define several instances under `spring.casbin.*` (one per
  domain) and inject each by its bean name.
* **Custom persistence**: the default file adapter keeps this starter database-free.
  To back policies with GORM, Redis, etc., build your own `*casbin.Enforcer` with the
  matching [Casbin adapter](https://casbin.org/docs/adapters) and register it as a bean
  instead of using this group.
