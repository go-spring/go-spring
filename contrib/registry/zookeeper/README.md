# ZooKeeper registry (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

Service registration & discovery through **ZooKeeper**, using a
[dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/)
`GreetService` over the **Triple** protocol (protobuf-over-HTTP/2, gRPC
wire-compatible). On startup the provider registers `greet.GreetService` into
ZooKeeper; the consumer resolves a live provider address from that same
ZooKeeper instead of dialing a hard-coded `host:port`.

ZooKeeper is the classic Dubbo registry — ZAB-consistent and battle-tested —
still widely deployed where a Dubbo ecosystem already runs one.

This is one of five sibling examples under [`..`](..) — see the top-level
[README](../README.md) for the registry overview. The four dubbo-go examples
(etcd / nacos / zookeeper / polaris) share **identical application code**; only
the registry block in `conf/app.properties` differs.

## Layout

```
zookeeper/
├── idl/greet.proto          # protobuf IDL
├── idl/greet.pb.go          # protoc-generated messages (DO NOT EDIT)
├── idl/greet.triple.go      # Triple-generated stubs (DO NOT EDIT)
├── idl/gen-code.sh          # regenerates idl/*.go from the IDL
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean
├── provider/main.go         # gs.Run(); long-lived, registers into ZooKeeper
├── provider/conf/app.properties  # provider config (registry + Triple port)
├── consumer/main.go         # discovers via ZooKeeper, calls, asserts, exits
├── consumer/conf/app.properties  # consumer config (registry + client protocol)
├── docker-compose.yml       # local ZooKeeper
└── scripts/smoke-test.sh    # smoke test: up zookeeper+provider, run consumer, tear down
```

## The registry configuration

Registries are declared once under `${spring.dubbo.registries}`; the map key is
a logical ID and `protocol` selects the driver. Switching to another registry is
just these two lines.

```properties
spring.dubbo.registries.zookeeper.protocol=zookeeper
spring.dubbo.registries.zookeeper.address=127.0.0.1:2181
```

The server publishes into every declared registry by default (no `registry-ids`
set); the consumer resolves `greet.GreetService` — the interface name baked into
the Triple stub — from the same registry.

## Run

```bash
docker compose up -d          # or docker-compose up -d
go run ./provider &           # long-lived, registers into ZooKeeper
go run ./consumer             # discovers via ZooKeeper and calls
```

Expected consumer output:

```
Response from discovered provider: Hello, Dubbo-Go!
```

Or the one-shot smoke test:

```bash
bash scripts/smoke-test.sh
```
