# go-kratos — WebSocket (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [go-kratos](https://github.com/go-kratos/kratos) `Greeter` example driven the
Go-Spring way: `gs.Run()` owns the lifecycle, the service is an IoC bean, and
the kratos WebSocket transport server (via
[`github.com/tx7do/kratos-transport`](https://github.com/tx7do/kratos-transport),
pinned to v1.3.1) is contributed by the
[`starter-kratos/ws`](../../../starter/starter-kratos) starter instead of
hand-wired in `main()`.

Unlike the HTTP and gRPC halves, kratos-transport WebSocket carries
**application-defined framed messages, not proto RPCs**, and its client has
**no discovery hook**. So although the provider still registers into an
**etcd registry** on startup (proving the lifecycle end-to-end), the consumer
dials the `ws://` URL directly rather than resolving via `discovery:///<name>`.

This is a runnable example, **not** a reusable starter module. The HTTP and
gRPC halves of the same `Greeter` service live next door in
[`../http`](../http) and [`../grpc`](../grpc) — each an independent module
wiring its own kratos transport, so importing one never pulls the others'
deps.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │
  ┌────────────▶│  :2379       │
  │  kratos-ws  └──────────────┘
  │  (name)
  │
┌─┴───────────┐        WS :9002          ┌─────────────┐
│  provider   │◀─────────────────────────│  consumer   │
│ gs.Run()    │      SayHello("Kratos")  │ gs.Run()    │
└─────────────┘   direct ws:// dial      └─────────────┘
```

## Layout

```
ws/
├── provider/            gs.Run() + ServiceRegister bean (handler.go binds
│   │                    message-type 1 to a handler, not a proto RPC)
│   └── conf/app.properties
├── consumer/            direct-dial WebSocket client (no etcd discovery)
│   └── conf/app.properties
├── idl/helloworld/v1/   proto + generated stubs (gen-code.sh regenerates)
├── docker-compose.yml   etcd only
└── scripts/smoke-test.sh
```

Wire format: every frame is `<4-byte little-endian uint32 messageType><JSON
payload>` (PayloadTypeBinary). Server and client agree on message type `1`
out of band.

## Run

```bash
# 1. Start etcd.
docker compose up -d

# 2. Start the provider (registers kratos-ws into etcd, serves WS :9002).
go run ./provider

# 3. In another shell, run the consumer (dials ws://127.0.0.1:9002/ directly).
go run ./consumer
# → Response from discovered provider (WebSocket): Hello Kratos-WS
```

Or run the whole loop end-to-end:

```bash
./scripts/smoke-test.sh
```

## Configuration

The kratos WebSocket server binds from `${spring.kratos.ws.server}` (see
`provider/conf/app.properties`):

| Key | Default | Meaning |
| --- | --- | --- |
| `spring.kratos.ws.server.name` | `kratos-ws` | service name published into etcd |
| `spring.kratos.ws.server.addr` | `0.0.0.0:9002` | WebSocket listen address |
| `spring.kratos.ws.server.path` | `/` | WebSocket upgrade path |
| `spring.kratos.ws.server.etcd.addr` | *(empty)* | etcd endpoint; empty = no registration |

Observability (tracing/metrics) for HTTP and gRPC is deferred to
[`starter-otel`](../../../starter/starter-otel). WebSocket is intentionally
**not** instrumented here: kratos-transport's WS server has no middleware
chain to hook, so this transport is a blind spot until an application-level
tracer is wired in.
