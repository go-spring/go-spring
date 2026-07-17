# goframe — Raw TCP (Go-Spring style)

[English](README.md) | [中文](README_CN.md)

A [GoFrame](https://goframe.org) raw-TCP **line-echo** service booted the
Go-Spring way. `gs.Run()` drives the lifecycle, the goframe `*gtcp.Server`
is an IoC bean, and the bind address comes from `conf/app.properties`
instead of `manifest/config/config.yaml`.

Like the sibling [`../http`](../http) and [`../websocket`](../websocket)
modules this example wires in an **etcd registry** for real **service
registration & discovery**. Unlike those two, though, gtcp has *no built-in
gsvc integration*: the starter's adapter calls
`gsvc.Registrar.Register` / `Deregister` by hand around the gtcp lifecycle.
This is the point of the module — it is a worked example of how a non-HTTP
goframe transport plugs into the same etcd registry as its siblings.

## Why gtcp is the odd one out

| Server type            | Ships gsvc integration? | How registration happens                                    |
| ---------------------- | ----------------------- | ----------------------------------------------------------- |
| `*ghttp.Server`        | yes                     | snapshots `gsvc.GetRegistry()` at `g.Server(name)`; auto    |
| `grpcx.GrpcServer`     | yes                     | snapshots `gsvc.GetRegistry()` at `grpcx.Server.New(cfg)`   |
| `*gtcp.Server` (this)  | **no**                  | adapter calls `Register` / `Deregister` around Run/Close    |

The consumer side is symmetric: goframe has no framework-level TCP client
that understands `gsvc://<name>`, so it goes through the same primitive path
the WebSocket sibling uses — `gsvc.Discovery.Search()` to resolve an
endpoint, then hand its host:port to `gtcp.NewNetConn`.

The server lifecycle (including the manual gsvc register/deregister) and glog
log bridge are **not** hand-rolled here anymore: they live in the reusable
[`starter-goframe/tcp`](../../../starter/starter-goframe) module. This example
just imports that starter and supplies a `ServiceRegister` bean.

## Topology

```
                    ┌──────────────┐
   register         │     etcd     │   discover (gsvc.Search)
  ┌────────────────▶│  :2379       │◀────────────────┐
  │                 └──────────────┘                 │
  │ goframe.tcp.echo                                 │ resolve provider host:port
  │ → tcp://127.0.0.1:8003                           │
┌─┴──────────┐                             ┌─────────┴──────┐
│  provider  │◀───── raw TCP frames ───────│    consumer    │
│ gs.Run()   │       line-delimited echo   │    one-shot    │
│ :8003      │────────────────────────────▶│ send+recv+exit │
└────────────┘                             └────────────────┘
```

## Layout

```
contrib/goframe/tcp/
├── provider/main.go              # gs.Run(); long-lived, registers into etcd
├── provider/handler.go           # provides starter-goframe/tcp's ServiceRegister; bufio line-echo handler
├── consumer/main.go              # gsvc.Search → gtcp.NewNetConn dial, asserts on echo, then exits
├── conf/app.properties           # provider configuration (${spring.goframe.tcp.server})
├── scripts/gen-code.sh           # documented no-op (raw TCP has no IDL)
├── docker-compose.yml            # local etcd
└── scripts/smoke-test.sh         # smoke test: bring up etcd+provider, run consumer, tear down
```

## Ordering rules the adapter has to enforce

Two ordering constraints come out of doing gsvc by hand instead of leaning on
ghttp/grpcx's built-ins (all handled inside `starter-goframe/tcp`):

1. **Bind before register.** Registering into etcd before `gtcp.Server.Run()`
   is listening would let consumers dial into a not-yet-open port and see
   "connection refused". The starter polls `Server.GetListenedPort()` briefly
   after spawning `Run()` in a goroutine before it calls `Register`.
2. **Deregister before close.** On shutdown, deregistering first avoids a
   window where a fresh consumer resolves this instance from etcd just as
   its listener is going away. `Stop()` calls `Deregister` then `Close()`
   in that order.

`ghttp.Server` and `grpcx.GrpcServer` hide both of these inside
`Start()`/`Shutdown()`; in the starter they are explicit because the
transport does not.

## Advertised endpoint

gtcp does not detect a public IP for you the way ghttp's registration path
does, so an explicit `advertise.host` / `advertise.port` pair is published
into etcd. In real deployments this is the pod/host IP; the example defaults
to `127.0.0.1` and the same port as the bind address.

## Configuration

```properties
# Disable Go-Spring's built-in HTTP server; the goframe *gtcp.Server owns the port.
spring.http.server.enabled=false

# gtcp bind address.
spring.goframe.tcp.server.address=:8003

# Host:port the provider advertises into etcd.
spring.goframe.tcp.server.advertise.host=127.0.0.1
spring.goframe.tcp.server.advertise.port=8003

# Service name the provider registers under; the consumer resolves this same
# name from etcd.
spring.goframe.tcp.server.name=goframe.tcp.echo

# etcd registry address; matches docker-compose.yml. Leave empty for a plain
# TCP server with no registration.
spring.goframe.tcp.server.registry.etcd=127.0.0.1:2379
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

Terminal B — start the consumer (discovers via etcd and dials TCP):

```bash
go run ./consumer
```

Expected consumer output:

```
Dialing discovered provider: 127.0.0.1:8003
Response from discovered provider: Hello, GoFrame TCP!
```

Or run the one-shot smoke test:

```bash
bash scripts/smoke-test.sh
```
