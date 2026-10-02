# singleton

[English](README.md) | [中文](README_CN.md)

## Introduction

`singleton` holds a value that is constructed on first use, together with the error returned by the constructor.

## Usage

```go
import "go-spring.org/stdlib/singleton"

var db singleton.Singleton[*sql.DB]

func Init(ctx context.Context) (*sql.DB, error) {
    return db.Init(func() (*sql.DB, error) {
        return sql.Open("mysql", dsn)
    })
}
```

The constructor runs on the first call; its value and error are cached, so later calls return the same pair without
running the constructor again.

## Concurrency

`Init` is safe for concurrent use and runs the constructor at most once. The lock is held while the constructor runs,
so the constructor must not call `Init` on the same `Singleton`, directly or indirectly — the lock is not reentrant
and that deadlocks.

## License

`singleton` is distributed under the Apache License 2.0. See [LICENSE](../../LICENSE) for details.
