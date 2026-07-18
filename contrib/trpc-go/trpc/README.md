# trpc-go — trpc (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [tRPC-Go](https://github.com/trpc-group/trpc-go) `GreetService` example
generated from a **protobuf** IDL and refactored to boot and be configured the
Go-Spring way: `gs.Run()` drives the lifecycle, the handler is an IoC bean,
and every knob comes from `conf/app.properties` instead of tRPC's native
`trpc_go.yaml`.

The deliberate design point being showcased here: unify tRPC-Go's
configuration into Go-Spring's property system. A contrib-local adapter
package `trpcgs/` translates properties under the `spring.trpc.server`
prefix into a tRPC `*Config`, then calls `trpc.NewServerWithConfig(cfg)` —
**no `trpc_go.yaml` file is used**. The server lifecycle is wrapped as a
`gs.Server` so it participates in Go-Spring's start/stop pipeline, and the
handler `GreetServiceImpl` is registered as a Go-Spring IoC bean (a
`trpcgs.ServiceRegister` bean), mirroring how `contrib/kitex` wires its
service.

This first example uses **direct-connect** dialing (`ip://127.0.0.1:8000`);
there is no service registry, so no docker/etcd is required to run it.

This is a runnable example, **not** a reusable starter module.

## Topology

```
┌────────────────┐                              ┌────────────────┐
│    provider    │                              │    consumer    │
│  gs.Run()      │◀──── tRPC / protobuf ────────│  one-shot      │
│  :8000         │       ip://127.0.0.1:8000    │  assert+exit   │
└────────────────┘                              └────────────────┘
       ▲                                                │
       │ trpcgs.SimpleTrpcServer                        │ NewGreetServiceClientProxy
       │ (gs.Server adapter)                            │ (client.WithTarget)
       │ builds *Config from                            │
       │ spring.trpc.server.*                           │
```

## Layout

```
contrib/trpc-go/trpc/
├── idl/greet.proto              # protobuf IDL (package trpc.helloworld.greet)
├── idl/greet.pb.go              # generated (DO NOT EDIT)
├── idl/greet.trpc.go            # generated tRPC stub (DO NOT EDIT)
├── idl/gen-code.sh              # regenerates greet.pb.go / greet.trpc.go
├── trpcgs/config.go             # Config + ServiceRegister (properties → tRPC *Config)
├── trpcgs/server.go             # SimpleTrpcServer: gs.Server adapter around trpc.NewServerWithConfig
├── trpcgs/logbridge.go          # log.SetLogger bridge from tRPC log into go-spring log
├── provider/handler.go          # GreetServiceImpl, exported as a trpcgs.ServiceRegister bean
├── provider/main.go             # gs.Run()
├── provider/conf/app.properties # provider configuration
├── consumer/main.go             # direct-connect client, asserts response, sends SIGTERM to exit
├── consumer/conf/app.properties # consumer configuration
└── scripts/smoke-test.sh        # smoke test: build+start provider, run consumer, tear down
```

## How it was generated

```bash
# tool (once)
go install trpc.group/trpc-go/trpc-cmdline/trpc@latest

# scaffold stubs from the IDL (or just run ./idl/gen-code.sh)
cd idl && trpc create --rpconly --protofile greet.proto -o .
```

The `--rpconly` flag asks the tRPC CLI to emit **only** the RPC stubs
(`greet.pb.go`, `greet.trpc.go`) — not a full project scaffold with its own
`main.go` / `trpc_go.yaml`. Those two files are shared by provider and
consumer. Re-running `./idl/gen-code.sh` regenerates them without touching
the refactored provider/consumer/`trpcgs` code.

## Configuration

Provider (`provider/conf/app.properties`):

```properties
# Disable the built-in HTTP server; the provider exposes only tRPC.
spring.http.server.enabled=false

# tRPC bind address; read via the ${spring.trpc.server} prefix.
spring.trpc.server.addr=127.0.0.1:8000

# Service name that the tRPC server registers (matches proto package.Service).
spring.trpc.server.service.name=trpc.helloworld.greet.GreetService
```

`trpcgs/logbridge.go` installs a `log.SetLogger` hook so tRPC-Go's internal
logs are forwarded into go-spring's log module. This first example ships no
observability backend (it is direct-connect, no docker), so no `FileLogger`
sink is configured: bridged tRPC log lines flow to go-spring's default
`ConsoleLogger`, proving the bridge visibly on stdout. An observability variant
would add a `logging.logger.root` `FileLogger` here to route them to a
file → Promtail → Loki pipeline instead.

Consumer (`consumer/conf/app.properties`):

```properties
spring.http.server.enabled=false

# Direct-connect target; no registry involved in this first example.
spring.trpc.consumer.target=ip://127.0.0.1:8000
```

## Signal handling — a caveat

tRPC-Go's `server.Serve()` installs its own OS signal handlers
(`SIGINT`, `SIGTERM`, `SIGSEGV`, `SIGUSR2`). It co-exists with Go-Spring's
lifecycle: when Go-Spring shuts down it calls `SimpleTrpcServer.Stop()`,
which in turn invokes `server.Close(nil)` to unblock `Serve()` cleanly.
Be aware of this signal co-ownership when embedding tRPC-Go inside another
lifecycle owner.

## Run

Terminal A — start the provider (long-lived):

```bash
go run ./provider
```

Terminal B — start the consumer (dials `127.0.0.1:8000` directly, asserts, exits):

```bash
go run ./consumer
```

Expected consumer output:

```
response from provider: Hello, Go-Spring!
```

Or run the one-shot smoke test (builds and starts the provider, waits for
port `8000`, runs the consumer, and tears everything down — no docker
required):

```bash
bash scripts/smoke-test.sh
```
