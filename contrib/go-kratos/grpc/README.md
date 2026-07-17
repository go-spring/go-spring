# go-kratos — gRPC (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [go-kratos](https://github.com/go-kratos/kratos) `Greeter` example driven the
Go-Spring way: `gs.Run()` owns the lifecycle, the service is an IoC bean, and
the kratos gRPC transport server is contributed by the
[`starter-kratos/grpc`](../../../starter/starter-kratos) starter instead of
hand-wired in `main()`.

The provider registers itself into an **etcd registry** on startup; the
consumer never learns the provider's host:port and instead resolves a live
address from the same etcd via `discovery:///<name>` and dials over gRPC.

This is a runnable example, **not** a reusable starter module. The HTTP and
WebSocket halves of the same `Greeter` service live next door in
[`../http`](../http) and [`../ws`](../ws) — each an independent module wiring
its own kratos transport, so importing one never pulls the others' deps.

## Topology

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │  kratos-grpc └──────────────┘ kratos-grpc │
  │  (name)                        (name)     │ resolve provider addr
  │                                           │
┌─┴───────────┐        gRPC :9000        ┌────┴────────┐
│  provider   │◀─────────────────────────│  consumer   │
│ gs.Run()    │      SayHello("Kratos")  │ gs.Run()    │
└─────────────┘                          └─────────────┘
```

## Layout

```
grpc/
├── provider/            gs.Run() + ServiceRegister bean (handler.go)
│   └── conf/app.properties
├── consumer/            etcd-discovery gRPC client
│   └── conf/app.properties
├── idl/helloworld/v1/   proto + generated stubs (gen-code.sh regenerates)
├── docker-compose.yml   etcd only
└── scripts/smoke-test.sh
```

## Run

```bash
# 1. Start etcd.
docker compose up -d

# 2. Start the provider (registers kratos-grpc into etcd, serves gRPC :9000).
go run ./provider

# 3. In another shell, run the consumer (discovers + calls over gRPC).
go run ./consumer
# → Response from discovered provider (gRPC): Hello Kratos
```

Or run the whole loop end-to-end:

```bash
./scripts/smoke-test.sh
```

## Configuration

The kratos gRPC server binds from `${spring.kratos.grpc.server}` (see
`provider/conf/app.properties`):

| Key | Default | Meaning |
| --- | --- | --- |
| `spring.kratos.grpc.server.name` | `kratos-grpc` | service name published into etcd |
| `spring.kratos.grpc.server.addr` | `0.0.0.0:9000` | gRPC listen address |
| `spring.kratos.grpc.server.etcd.addr` | *(empty)* | etcd endpoint; empty = direct-connect, no registration |

Observability (tracing/metrics) is deferred to
[`starter-otel`](../../../starter/starter-otel): import it and the starter's
`tracing.Server()` middleware exports spans automatically. This lean example
ships neither.
