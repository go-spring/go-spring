# go-kratos (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Kratos](https://go-kratos.dev/) `Greeter` example that starts from a project
the `kratos` toolchain scaffolds (`kratos new`) and is then refactored to boot
and be configured the Go-Spring way: `gs.Run()` drives the lifecycle, every
layer is wired through the Go-Spring IoC container instead of `google/wire`, and
the server bind addresses come from `conf/app.properties` instead of Kratos'
YAML config.

The full Kratos layered layout (`api` + `internal/{biz,data,service,server}`) is
kept intact — only the **bootstrap, dependency injection, and config** engines
are swapped. It exposes both the **HTTP** (`:8000`) and **gRPC** (`:9000`)
Greeter endpoints the scaffold generates.

This is a runnable example, **not** a reusable starter module.

## Layout

```
contrib/go-kratos/
├── api/helloworld/v1/          # protoc-generated gRPC + HTTP stubs (DO NOT EDIT)
├── internal/biz/               # business logic (GreeterUsecase + GreeterRepo interface)
├── internal/data/              # data layer (Data, greeterRepo) + shared kratos logger bean
├── internal/service/           # service layer (GreeterService)
├── internal/server/            # HTTPServer / GRPCServer adapters (gs.Server) + Config
├── main.go                     # gs.Run() + a self-test client (HTTP & gRPC)
└── conf/app.properties         # configuration
```

## How it was generated

```bash
# tool (once)
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest

# scaffold the project (clones the kratos-layout template)
kratos new go-kratos
```

The scaffold produces `cmd/` (a `wire` + `kratos.App` bootstrap), `configs/config.yaml`,
`internal/conf/` (a `conf.proto` Bootstrap message), and the layered `internal/`
code. The refactor drops `cmd/`, `configs/`, `internal/conf/`, and the `wire`
files, keeps the generated `api/` stubs untouched, and rewires the rest.

## The refactor: native Kratos → Go-Spring

| Concern            | Kratos scaffold                                             | Go-Spring version                                                                   |
| ------------------ | ----------------------------------------------------------- | ----------------------------------------------------------------------------------- |
| Startup            | `kratos.New(...).Run()` owns the process                    | each server implements `gs.Server`; `gs.Run()` drives Run/Stop                      |
| Dependency wiring  | `google/wire` `ProviderSet` + generated `wire_gen.go`       | `init()` + `gs.Provide` per layer; blank imports in `main.go` trigger registration  |
| Server registration| `wire.NewSet(NewGRPCServer, NewHTTPServer)`                 | `gs.Provide(...).Export(gs.As[gs.Server]())` for each transport server             |
| Interface binding  | `NewGreeterRepo` returns `biz.GreeterRepo` for wire         | same constructor; the container resolves it as the `biz.GreeterRepo` bean          |
| Config source      | `configs/config.yaml` scanned into `conf.proto` `Bootstrap` | `conf/app.properties` bound via `value:"${spring.kratos.http}"` structs            |
| Shutdown           | `kratos.App` graceful stop                                  | graceful shutdown coordinated by Go-Spring (SIGTERM → `Stop()`)                    |

The adapters in `internal/server/{http,grpc}.go` are the crux: a Kratos
transport server's `Start(ctx)` binds the listener and blocks until `Stop(ctx)`
triggers a graceful shutdown, so `Run` simply calls `Start` after
`sig.TriggerAndWait()` and `Stop` delegates to the Kratos server's `Stop`.

## Configuration

```properties
# Disable the built-in HTTP server; this example runs only the two kratos servers.
spring.http.server.enabled=false

# kratos HTTP transport server, bound via the ${spring.kratos.http} prefix.
spring.kratos.http.addr=0.0.0.0:8000
spring.kratos.http.timeout=1s

# kratos gRPC transport server, bound via the ${spring.kratos.grpc} prefix.
spring.kratos.grpc.addr=0.0.0.0:9000
spring.kratos.grpc.timeout=1s
```

## Run

```bash
go run .
```

`main.go` launches a client 500ms after startup, calls the Greeter over both
HTTP (`GET /helloworld/Kratos`) and gRPC (`SayHello`), asserts the replies, then
sends SIGTERM so Go-Spring shuts down cleanly. Expected output includes:

```
HTTP response from server: Hello Kratos
gRPC response from server: Hello Kratos
```
