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
spring.pprof.addr=127.0.0.1:9981
# Optional authentication for off-host exposure:
spring.pprof.token=s3cr3t
```

### 3. Access the pprof Endpoints

With the default configuration, the pprof server binds to loopback only
(`127.0.0.1:9981`):

```text
http://127.0.0.1:9981/debug/pprof/
```

When a token is configured, every request must present it as either an
`Authorization: Bearer <token>` header or a `?token=<token>` query parameter:

```bash
curl -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/
curl 'http://127.0.0.1:9981/debug/pprof/heap?token=s3cr3t'
```

## Core Features

The example exercises three representative pprof endpoints served on the
dedicated pprof HTTP server (`127.0.0.1:9981` by default):

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
| `spring.pprof.addr` | `127.0.0.1:9981` | Listen address. Defaults to a loopback-only bind so the endpoints are not reachable off-host unless you opt in. |
| `spring.pprof.token` | `` | When set, every request must present the token via `Authorization: Bearer <token>` or `?token=<token>`. Takes precedence over basic auth. |
| `spring.pprof.username` | `` | Username for HTTP Basic authentication (used together with `password`). |
| `spring.pprof.password` | `` | Password for HTTP Basic authentication (used together with `username`). |

pprof endpoints expose sensitive runtime internals (goroutine stacks, heap, CPU
profiles), so the defaults are deliberately conservative: the server binds to
loopback only, and callers opt into remote exposure explicitly. When a
non-loopback address is used without any authentication configured, the starter
logs a warning at startup — set a token or username/password, or keep the bind
on loopback.

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
