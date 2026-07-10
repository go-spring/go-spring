# starter-pprof

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-pprof` exposes the standard Go `net/http/pprof` endpoints through a
lightweight, dedicated HTTP server managed by the Go-Spring IoC container. It
gives Go-Spring applications a simple, configurable way to inspect runtime
behavior, collect CPU profiles, capture traces, and debug goroutine, heap,
thread, mutex, and block profiles.

## Installation

```bash
go get go-spring.org/starter-pprof
```

## Quick Start

### 1. Import the `starter-pprof` Package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-pprof"
```

### 2. Configure the pprof Server

Add pprof configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.pprof.enabled=true
spring.pprof.addr=:9981
```

### 3. Access the pprof Endpoints

With the default configuration, the pprof server listens on `:9981`:

```text
http://127.0.0.1:9981/debug/pprof/
```

## Core Features

The example exercises three representative pprof endpoints served on the
dedicated pprof HTTP server (`:9981` by default):

- **`GET /debug/pprof/`** — index page listing every available profile.
- **`GET /debug/pprof/heap`** — snapshot of the heap allocation profile.
- **`GET /debug/pprof/cmdline`** — the running program's command line, useful
  for correlating profiles with build/run parameters.

Each is asserted to return HTTP 200 before the example shuts itself down.

## Configuration

The starter reads the following Go-Spring properties:

| Property | Default | Description |
| --- | --- | --- |
| `spring.pprof.enabled` | `true` | Enables or disables the pprof server. |
| `spring.pprof.addr` | `:9981` | Address used by the dedicated pprof HTTP server. |

## Available Endpoints

The starter registers the standard pprof handlers:

- `/debug/pprof/` (also serves `/heap`, `/goroutine`, `/allocs`, `/block`,
  `/mutex`, `/threadcreate` via `pprof.Index`)
- `/debug/pprof/cmdline`
- `/debug/pprof/profile`
- `/debug/pprof/symbol`
- `/debug/pprof/trace`

## License

This project is licensed under the Apache License 2.0.
