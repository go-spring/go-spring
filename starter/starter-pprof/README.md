# starter-pprof

[English](README.md) | [中文](README_CN.md)

`starter-pprof` is a Go-Spring starter that exposes the standard Go `net/http/pprof`
endpoints through a lightweight HTTP server managed by the Go-Spring IoC
container.

It is designed for Go-Spring applications that need a simple, configurable way
to inspect runtime behavior, collect CPU profiles, capture traces, and debug
goroutine, heap, thread, mutex, and block profiles.

## Features

- Registers a `gs.Server` bean automatically when imported.
- Exposes the standard `/debug/pprof/` endpoints from Go's `net/http/pprof`
  package.
- Starts on a dedicated HTTP address, separated from the main application
  server.
- Supports property-based enablement and address configuration.
- Works with a blank import, so no manual route wiring is required.

## Installation

```bash
go get go-spring.org/starter-pprof
```

## Usage

Import the starter for its side effects and run your Go-Spring application:

```go
package main

import (
	"go-spring.org/spring/gs"
	_ "go-spring.org/starter-pprof"
)

func main() {
	gs.Run()
}
```

With the default configuration, the pprof server listens on `:9981`:

```text
http://127.0.0.1:9981/debug/pprof/
```

## Configuration

The starter reads the following Go-Spring properties:

| Property | Default | Description |
| --- | --- | --- |
| `spring.pprof.enabled` | `true` | Enables or disables the pprof server. |
| `spring.pprof.addr` | `:9981` | Address used by the dedicated pprof HTTP server. |

Example:

```properties
spring.pprof.enabled=true
spring.pprof.addr=:9090
```

Then open:

```text
http://127.0.0.1:9090/debug/pprof/
```

## Available Endpoints

The starter registers the standard pprof handlers:

- `/debug/pprof/`
- `/debug/pprof/cmdline`
- `/debug/pprof/profile`
- `/debug/pprof/symbol`
- `/debug/pprof/trace`

Additional profile views such as goroutine, heap, allocs, mutex, block, and
threadcreate are served by the pprof index handler when available in the Go
runtime.

## License

This project is licensed under the Apache License 2.0.
