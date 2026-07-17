# goframe — WebSocket (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [GoFrame](https://goframe.org) WebSocket **echo** service booted the
Go-Spring way. `gs.Run()` drives the lifecycle, the goframe `*ghttp.Server`
is an IoC bean, and the bind address comes from `conf/app.properties`
instead of `manifest/config/config.yaml`.

Like the sibling [`../http`](../http) module this example wires in an **etcd
registry** for real **service registration & discovery**: on startup the
provider registers `goframe.websocket.echo` into etcd; the consumer never
learns the provider's host:port and instead resolves a live endpoint from the
same etcd through goframe's `gsvc` layer, then dials `ws://<host>:<port>/echo`
with `gorilla/websocket`.

## Why WebSocket lives on top of `*ghttp.Server`

GoFrame has no standalone WebSocket server type. A WebSocket connection
starts as an HTTP `GET` whose `Upgrade` header the server rewrites, and in
goframe that upgrade is a one-liner inside a normal `ghttp` handler:

```go
s.BindHandler("/echo", func(r *ghttp.Request) {
    ws, err := r.WebSocket() // gorilla-backed upgrade
    if err != nil { return }
    defer ws.Close()
    for {
        t, data, err := ws.ReadMessage()
        if err != nil { return }
        _ = ws.WriteMessage(t, data)
    }
})
```

That means everything HTTP-shaped in the sibling `../http` module still
applies here — the same server bean, the same `gsvc.SetRegistry` call, the
same graceful shutdown path — with **only the `/echo` handler swapped** from
"write a response body" to "upgrade and echo frames".

The server lifecycle and glog log bridge are **not** hand-rolled here anymore:
they live in the reusable
[`starter-goframe/ws`](../../../starter/starter-goframe) module. This example
just imports that starter and supplies a `ServiceRegister` bean.

## Topology

```
                    ┌──────────────┐
   register         │     etcd     │   discover (gsvc.Search)
  ┌────────────────▶│  :2379       │◀────────────────┐
  │                 └──────────────┘                 │
  │ goframe.websocket.echo                           │ resolve provider host:port
  │ → ws://<host>:8002/echo                          │
┌─┴──────────┐                             ┌─────────┴──────┐
│  provider  │◀────── WebSocket ───────────│    consumer    │
│ gs.Run()   │       echo frames           │    one-shot    │
│ :8002/echo │────────────────────────────▶│ send+recv+exit │
└────────────┘                             └────────────────┘
```

## Layout

```
contrib/goframe/websocket/
├── provider/main.go              # gs.Run(); long-lived, registers into etcd
├── provider/handler.go           # provides starter-goframe/ws's ServiceRegister; /echo upgrade + frame-echo handler
├── consumer/main.go              # gsvc.Search → gorilla-websocket dial, asserts on echo, then exits
├── conf/app.properties           # provider configuration (${spring.goframe.ws.server})
├── scripts/gen-code.sh           # documented no-op (WS/HTTP handlers are hand-written)
├── docker-compose.yml            # local etcd
└── scripts/smoke-test.sh         # smoke test: bring up etcd+provider, run consumer, tear down
```

## Differences from the sibling protocols

| Concern    | `../http`                                                                           | `../grpc`                                                          | this module (WebSocket)                                         |
| ---------- | ----------------------------------------------------------------------------------- | ------------------------------------------------------------------ | --------------------------------------------------------------- |
| Server     | `*ghttp.Server` from `g.Server(name)`                                               | `grpcx.GrpcServer` from `grpcx.Server.New(cfg)`                    | `*ghttp.Server`, same as http (upgrade lives in the handler)    |
| IDL/gen    | `api/*/v*/` + `gf gen ctrl`                                                         | `idl/echo.proto` + `protoc`                                        | none — hand-written handler, `scripts/gen-code.sh` is a no-op                |
| Client API | `g.Client().Discovery(reg).Get(ctx, "http://<name>/hello")`                         | `grpcx.Client.MustNewGrpcClientConn(<name>)`                       | `registry.Search(...)` → `gorilla/websocket.Dial(ws://host:port/echo)` |
| Why the client differs | goframe's gclient discovery middleware rewrites HTTP `URL.Host` under the hood | grpcx registers a `gsvc://` resolver builder for gRPC | no ws-aware client in goframe; resolve then dial with gorilla directly |

The provider side is essentially the http sibling with a different handler.
The consumer side is the interesting bit: goframe's `gclient` discovery
middleware is HTTP-only, and grpcx's resolver builder is gRPC-only, so the
generic path — **use `gsvc.Discovery.Search()` to resolve an endpoint, then
hand its host:port to any transport-specific client** — is the one that
generalises to WS (and to TCP, MQTT, or any other protocol goframe does not
ship a wired-up client for).

## Configuration

```properties
# Disable Go-Spring's built-in HTTP server; the goframe *ghttp.Server owns the port.
spring.http.server.enabled=false

# HTTP bind address for the goframe *ghttp.Server (the /echo route upgrades to WS here).
spring.goframe.ws.server.address=:8002

# Service name the provider registers under; the consumer resolves this same
# name from etcd.
spring.goframe.ws.server.name=goframe.websocket.echo

# etcd registry address; matches docker-compose.yml. Leave empty for a plain
# server clients dial directly.
spring.goframe.ws.server.registry.etcd=127.0.0.1:2379
```

## Run

Bring up the registry first:

```bash
docker compose up -d      # or docker-compose up -d
```

Terminal A — start the provider (long-lived, registers into etcd):

```bash
go run ./provider
```

Terminal B — start the consumer (discovers via etcd and dials the upgrade):

```bash
go run ./consumer
```

Expected consumer output:

```
Dialing discovered provider: ws://127.0.0.1:8002/echo
Response from discovered provider: Hello, GoFrame WebSocket!
```

Or run the one-shot smoke test (brings up etcd + provider, runs the consumer,
tears everything down):

```bash
bash scripts/smoke-test.sh
```
