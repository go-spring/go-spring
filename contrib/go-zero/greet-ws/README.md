# go-zero — WebSocket (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [go-zero](https://go-zero.dev) `Greet` example served over **WebSocket**,
booted and configured the Go-Spring way: `gs.Run()` drives the lifecycle,
the business logic is an IoC bean, and the bind address comes from
`conf/app.properties` instead of hard-coded `main()` wiring.

This is the third protocol under `contrib/go-zero/`, alongside
[`../greet-api`](../greet-api) (HTTP) and [`../greet-rpc`](../greet-rpc)
(zRPC/gRPC). WebSocket is worth its own example because it is the only
non-HTTP server paradigm that go-zero exposes on top of `rest.Server` — and
it forces a shape (long-lived connection, per-frame loop) that the .api
DSL cannot describe.

Two consequences follow, both intentional:

- **No etcd, no service discovery.** WS rides on `rest.Server`, exactly like
  `greet-api`. go-zero's registry story lives only in the zRPC layer, so the
  consumer dials a fixed `host:port`.
- **No goctl-generated files.** goctl's `.api` DSL only understands
  request/response HTTP endpoints; there is no way to declare a WS route or
  a WS frame type in it. Everything under `internal/` here is hand-written.
  `scripts/gen-code.sh` is a documented no-op that exists only to keep the entry point
  shape consistent with the two sibling projects.

This is a runnable example, **not** a reusable starter module.

## Topology

```
┌────────────┐        WS /greet             ┌────────────┐
│  provider  │◀─────────────────────────────│  consumer  │
│ gs.Run()   │  {"name":"Hello, go-zero!"}  │ one-shot   │
│  :8890     │──────────────────────────────▶│ assert+exit│
└────────────┘   {"greeting":"Hello, ..."}  └────────────┘
        (persistent WebSocket, single frame each way)
```

## Layout

```
contrib/go-zero/greet-ws/
├── scripts/gen-code.sh                 # documented no-op (WS has no IDL in go-zero)
├── internal/types/types.go             # hand-written; WS frame payloads (JSON)
├── internal/handler/wshandler.go       # hand-written; upgrade + read/write loop
├── internal/svc/servicecontext.go      # hand-written; injected Logic surface
├── internal/logic/greetlogic.go        # hand-written; GreetLogic IoC bean
├── provider/handler.go                 # HandlerRegister bean; attaches WS route
├── provider/server.go                  # RestServer adapter (gs.Server) + Config
├── provider/main.go                    # gs.Run(); long-lived process
├── consumer/main.go                    # WS dialer, asserts on echo, exits
├── conf/app.properties                 # provider configuration
└── scripts/smoke-test.sh               # smoke test: build+run provider, run consumer, tear down
```

## Why WebSocket differs from `greet-api` / `greet-rpc`

| Concern         | `greet-api` (.api HTTP)                        | `greet-rpc` (zRPC/gRPC)                     | `greet-ws` (this)                                                          |
| --------------- | ---------------------------------------------- | ------------------------------------------- | -------------------------------------------------------------------------- |
| Server type     | `rest.Server`                                  | `zrpc.RpcServer`                            | `rest.Server` (same as greet-api)                                          |
| IDL / codegen   | `greet.api` + `goctl api go`                   | `greet.proto` + `goctl rpc protoc`          | none — WS has no IDL in go-zero                                            |
| Transport shape | one HTTP request → one response, connection may pool | one gRPC call → one response, HTTP/2 multiplex | one TCP conn upgraded, N frames in each direction until close             |
| Handler shape   | parse → call logic → render JSON               | proto-generated method                      | upgrade → for-loop over `conn.ReadMessage`, dispatch each frame            |
| Discovery       | none (rest.Server has no registry)             | etcd via zRPC's `EtcdConf`                  | none (WS inherits rest.Server's lack of registry)                          |
| Consumer        | `http.Post` + JSON decode                      | zRPC client, resolver `etcd://…`            | `websocket.Dialer.Dial` + one frame exchange                               |
| Startup         | `RestServer` implements `gs.Server`; `gs.Run()` drives Run/Stop | `RpcServer` implements `gs.Server`; `gs.Run()` drives Run/Stop | identical to greet-api — same adapter code |

The adapter code in `provider/server.go` is therefore deliberately identical
to `greet-api`'s: WebSocket is *served* by the same `rest.Server` binary; the
only thing that changes is what the registered `HandlerRegister` bean does
inside the request — call `httpx.OkJsonCtx` (HTTP) or upgrade to WS.

## Configuration

```properties
# Disable the Go-Spring built-in HTTP server; the provider exposes only the
# go-zero rest.Server bound below.
spring.http.server.enabled=false

# go-zero rest.Server settings; read via the ${spring.rest.server} prefix.
# Port 8890 (rather than greet-api's 8888) so the two examples can run side
# by side without colliding.
spring.rest.server.name=greet-ws
spring.rest.server.host=0.0.0.0
spring.rest.server.port=8890
```

## Run

Terminal A — start the provider (long-lived):

```bash
go run ./provider
```

Terminal B — start the consumer (WS dial, one frame, self-asserts):

```bash
go run ./consumer
```

Expected consumer output:

```
Response from provider: Hello, go-zero!
```

Or run the one-shot smoke test (starts the provider, runs the consumer, tears
it down):

```bash
bash scripts/smoke-test.sh
```

## About `scripts/gen-code.sh`

`scripts/gen-code.sh` is intentionally a no-op — it prints a note and exits. WebSocket
cannot be described in go-zero's `.api` DSL, so `goctl api go` has nothing to
say about the route or the frame types. Compare `../greet-api/scripts/gen-code.sh` (drives
`goctl api go`) and `../greet-rpc/scripts/gen-code.sh` (drives `goctl rpc protoc`). To
change a WS field or add a route, edit the files under `internal/` directly.
