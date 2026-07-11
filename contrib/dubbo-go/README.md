# dubbo-go (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [Dubbo-go](https://dubbo.apache.org/en/overview/mannual/golang-sdk/) `GreetService`
example that starts from code the Dubbo-go toolchain generates and is then
refactored to boot and be configured the Go-Spring way: `gs.Run()` drives the
lifecycle, the provider is an IoC bean, and the bind port comes from
`conf/app.properties` instead of hard-coded `main()` wiring.

It uses the **Triple** protocol with a **direct (registry-less)** connection, so
the example is self-contained — no ZooKeeper/Nacos required.

This is a runnable example, **not** a reusable starter module.

## Layout

```
contrib/dubbo-go/
├── proto/greet.proto        # Protobuf IDL
├── proto/greet.pb.go        # protoc-generated messages (DO NOT EDIT)
├── proto/greet.triple.go    # Triple-generated stubs (DO NOT EDIT)
├── handler.go               # GreetProvider, exported as a greet.GreetServiceHandler bean
├── server.go                # DubboServer adapter (gs.Server) + Config
├── main.go                  # gs.Run() + a self-test client
└── conf/app.properties      # configuration
```

## How it was generated

```bash
# tools (once)
go install github.com/dubbogo/protoc-gen-go-triple/v3@latest

# generate messages + Triple stubs from the IDL
protoc --proto_path=proto \
  --go_out=paths=source_relative:./proto \
  --go-triple_out=paths=source_relative:./proto \
  proto/greet.proto
```

The generator produces `greet.pb.go` and `greet.triple.go` in `proto/`.
Re-running the command regenerates only those files without touching the
refactored `handler.go` / `server.go` / `main.go`.

> Note: on a go1.26 toolchain whose `runtime.Version()` carries an experiment
> suffix (e.g. `go1.26.1-X:jsonv2`), `protoc-gen-go-triple` v3.0.3 panics while
> parsing the version. Rebuild it from source with the version string truncated
> to its numeric part.

## The refactor: native Dubbo-go → Go-Spring

| Concern        | Dubbo-go scaffold                          | Go-Spring version                                                              |
| -------------- | ------------------------------------------ | ------------------------------------------------------------------------------ |
| Startup        | `srv.Serve()` blocks in `main()`           | `DubboServer` implements `gs.Server`; `gs.Run()` drives Run/Stop               |
| Handler wiring | `RegisterGreetServiceHandler(srv, &impl)`  | `gs.Provide(&GreetProvider{}).Export(gs.As[greet.GreetServiceHandler]())`      |
| Server enable  | always on                                  | conditional on a `greet.GreetServiceHandler` bean via `gs.OnBean`              |
| Port           | hard-coded default                         | `${spring.dubbo.server.port}` from `conf/app.properties`                       |
| Shutdown       | process-owned                              | graceful shutdown coordinated by Go-Spring (SIGTERM → `Stop()`)                |

The adapter in `server.go` is the crux: Dubbo-go's `Serve()` binds the listener
and then blocks forever, so it runs in a goroutine started only after
`sig.TriggerAndWait()`, while `Run` parks on a done channel that `Stop()` closes
to hand control back to Go-Spring's shutdown sequence.

## Configuration

```properties
# Disable the built-in HTTP server; this example exposes only Dubbo.
spring.http.server.enabled=false

# Dubbo Triple bind port; read via the ${spring.dubbo.server} prefix, default 20000.
spring.dubbo.server.port=20000
```

## Run

```bash
go run .
```

`main.go` launches a client 500ms after startup, calls `Greet("Hello, Dubbo-Go!")`,
asserts the returned greeting, then sends SIGTERM so Go-Spring shuts down
cleanly. Expected output includes:

```
Response from server: Hello, Dubbo-Go!
```
