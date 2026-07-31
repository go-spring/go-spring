# starter-pprof

[English](README.md) | [中文](README_CN.md)

`starter-pprof` exposes the standard Go `net/http/pprof` endpoints through a
dedicated HTTP server managed by the Go-Spring IoC container. Use it to inspect
runtime behavior, collect CPU profiles, capture traces, and debug goroutine, heap,
thread, mutex, and block profiles.

## Installation

```bash
go get go-spring.org/starter-pprof
```

## Quick Start

### 1. Import the `starter-pprof` Package

```go
import _ "go-spring.org/starter-pprof"
```

### 2. Configure the pprof Server

Add pprof configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.pprof.enabled=true
spring.pprof.addr=:9981
# Bind to loopback only, or configure authentication for off-host exposure:
# spring.pprof.addr=127.0.0.1:9981
spring.pprof.token=s3cr3t
```

### 3. Access the pprof Endpoints

With the default configuration, the pprof server binds to all interfaces
(`:9981`):

```text
http://<host>:9981/debug/pprof/
```

When a token is configured, every request must present it as either an
`Authorization: Bearer <token>` header or a `?token=<token>` query parameter:

```bash
curl -H 'Authorization: Bearer s3cr3t' http://127.0.0.1:9981/debug/pprof/
curl 'http://127.0.0.1:9981/debug/pprof/heap?token=s3cr3t'
```

## Core Features

The example exercises three representative pprof endpoints served on the
dedicated pprof HTTP server (the example pins `127.0.0.1:9981`):

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
| `spring.pprof.addr` | `:9981` | Listen address. Defaults to all interfaces; use `127.0.0.1:9981` to restrict access to the local host. |
| `spring.pprof.token` | `` | When set, every request must present the token via `Authorization: Bearer <token>` or `?token=<token>`. Takes precedence over basic auth. |
| `spring.pprof.username` | `` | Username for HTTP Basic authentication (used together with `password`). |
| `spring.pprof.password` | `` | Password for HTTP Basic authentication (used together with `username`). |

pprof endpoints expose sensitive runtime internals (goroutine stacks, heap, CPU
profiles), so they must not be reachable unauthenticated off-host. The default
address binds to all interfaces for operational convenience; when that is used
without any authentication configured, the starter logs a warning at startup —
set a token or username/password, or restrict the bind to loopback.

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
