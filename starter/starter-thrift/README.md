# starter-thrift

[English](README.md) | [中文](README_CN.md)

> The project has been officially released, welcome to use!

`starter-thrift` provides a lightweight [Apache Thrift](https://thrift.apache.org/)
server wrapper for Go-Spring applications: provide a `thrift.TProcessor`
bean and the starter takes care of listener setup, lifecycle, and
graceful shutdown via `TSimpleServer`.

## Installation

```bash
go get go-spring.org/starter-thrift
```

## Quick Start

### 1. Import the `starter-thrift` package

Refer to the [example.go](example/example.go) file.

```go
import _ "go-spring.org/starter-thrift"
```

### 2. Configure the Thrift server

Add Thrift configuration in your project's [configuration file](example/conf/app.properties):

```properties
spring.http.server.enabled=false
spring.thrift.server.addr=:9292

# Per-connection client timeout on the server socket (0 = no timeout).
spring.thrift.server.clientTimeout=30s

# Wire protocol: binary (default) / compact / json. Must match the client.
spring.thrift.server.protocol=binary

# Transport wrapper: none (raw socket, default) / buffered / framed.
# Must match the client. "framed" is required by many cross-language clients.
spring.thrift.server.transport=none

# Buffer / max frame size (bytes) for buffered/framed transports.
spring.thrift.server.bufferSize=4096

# TLS server transport: enable and point at a PEM cert/key pair.
spring.thrift.server.tls.enabled=false
spring.thrift.server.tls.cert-file=
spring.thrift.server.tls.key-file=
```

### 3. Register your processor

Refer to the [example.go](example/example.go) file.

```go
gs.Provide(&Controller{})
gs.Provide(func(c *Controller) thrift.TProcessor {
    return proto.NewEchoServiceProcessor(c)
})
```

## Core Features

The [example](example/example.go) demonstrates three core Thrift building
blocks, each asserted end-to-end by `runTest`:

1. **Echo RPC** — the client invokes `EchoService.Echo` with
   `"Hello, Thrift!"` and asserts the response body is identical,
   verifying the standard request/response path over the configured
   `compact` protocol + `framed` transport.
2. **TProcessor middleware/decorator** — `loggingProcessor` is a real
   `thrift.TProcessor` implementation that wraps the generated
   `EchoServiceProcessor`. On each RPC it reads the method name, logs
   it, then dispatches to the wrapped processor's per-method
   `TProcessorFunction`. This is the Thrift analogue of a gRPC
   `UnaryServerInterceptor`. The wrapped processor is what
   `gs.Provide` publishes, so `TSimpleServer` picks it up without any
   changes to the starter. A per-invocation atomic counter is checked
   at the end of `runTest` to prove the middleware fired once per RPC.
3. **Second round-trip with a distinct payload** — the client makes a
   second `Echo` call with `"Middleware works!"` and asserts the body
   again. Combined with feature 2, this proves the decorator forwards
   independent calls correctly and runs per RPC (counter == 2).

### A note on transports

The starter builds the server with `NewTSimpleServer4`, exposing both a
protocol factory and a transport factory:

- **Protocol** (`spring.thrift.server.protocol`): `binary` (default),
  `compact`, or `json`. The client's protocol factory must match.
- **Transport** (`spring.thrift.server.transport`): `none` (raw socket,
  the historical default), `buffered`, or `framed`. `framed` prepends a
  length prefix to each message and is required by many cross-language
  clients. The client's transport must match.

A client/server protocol or transport mismatch will deadlock or corrupt
the wire protocol. The [example](example/example.go) configures the
server with `compact` + `framed` and pairs the client accordingly:
`TFramedTransport` wrapping the socket + `TCompactProtocol`.

### A note on server models

Go's Thrift library only ships `TSimpleServer`. Despite the name, its
`AcceptLoop` spawns one goroutine per connection, so it is already a
concurrent server. The `THsHaServer` / `TThreadPoolServer` variants are
Java/C++ concepts and do not exist in the Go library — there is no
"multi-threaded server" to switch to.

## Notes

- The starter listens on `${spring.thrift.server.addr}` (default `:9292`).
- The Thrift server is enabled by default; disable it with
  `spring.thrift.server.enabled=false`.
- Only a `thrift.TProcessor` bean is required to activate the server.
