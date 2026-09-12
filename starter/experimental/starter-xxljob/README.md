# starter-xxljob

[English](README.md) | [中文](README_CN.md)

`starter-xxljob` runs an [xxl-job](https://github.com/xuxueli/xxl-job)
executor: it speaks the executor protocol (registry / run / kill / log) to a
stock xxl-job admin over plain HTTP, hand-rolled with zero third-party
dependencies.

## Installation

```bash
go get go-spring.org/starter-xxljob
```

## Quick Start

### 1. Import

```go
import _ "go-spring.org/starter-xxljob"
```

### 2. Configure

```properties
spring.xxljob.instances.a.app-name=go-spring-demo
spring.xxljob.instances.a.admin-addresses=http://127.0.0.1:8080/xxl-job-admin
spring.xxljob.instances.a.port=9999
```

### 3. Inject & register a handler

```go
type Service struct {
    Executor *StarterXxljob.Executor `autowire:"a"`
}

func (s *Service) Init() error {
    s.Executor.RegisterHandler("demoJob", func(ctx context.Context, param string) error {
        return nil // return an error to report failure to the admin
    })
    return nil
}
```

## Core Features

- **Executor protocol** — /run /beat /idleBeat /kill /log callback server,
  plus the registry/heartbeat/remove loop against the admin.
- **Cancellable tasks** — /kill cancels a running handler's context; a panic
  is recovered through the shared panic chain.
- **Zero dependencies** — the protocol is the API, no SDK.

## Advanced Features

**Multiple executors** — additional `spring.xxljob.instances.<name>` entries, each with
its own callback port and handler registry.
