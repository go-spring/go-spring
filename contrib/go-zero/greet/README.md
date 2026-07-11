# greet — go-zero converted to Go-Spring

[English](README.md) | [中文](README_CN.md)

A stock [go-zero](https://github.com/zeromicro/go-zero) API project, generated with
`goctl api new greet`, then converted from go-zero's native config loading and startup to
**Go-Spring**'s configuration binding and lifecycle.

The generated business code (`internal/handler`, `internal/logic`, `internal/svc`,
`internal/types`) is left untouched — only *how the service is configured and started* changes.

## What changed

| Concern | Stock go-zero | Converted to Go-Spring |
| --- | --- | --- |
| Config source | `etc/greet-api.yaml` loaded via `conf.MustLoad` | `conf/app.properties` bound via `value:"${...}"` tags |
| Config struct | `config.Config` embeds `rest.RestConf` | `config.Config` uses `value` tags + a `RestConf()` builder |
| Server creation | `rest.MustNewServer` inline in `main()` | `internal/server.NewGreetServer`, a `gs.Server` bean |
| Route registration | `handler.RegisterHandlers` inline in `main()` | done inside the server bean constructor |
| ServiceContext | `svc.NewServiceContext` inline in `main()` | registered as a Go-Spring bean, config injected |
| Startup | `flag.Parse()` → `conf.MustLoad` → `server.Start()` | `gs.Run()` drives the container lifecycle |
| Shutdown | go-zero's internal signal handling | `GreetServer.Stop()` gracefully shuts down the `*http.Server` |

## Layout

```
greet/
├── conf/app.properties          # Go-Spring config (replaces etc/greet-api.yaml)
├── greet.go                     # main(): bean registration + gs.Run()
├── greet.api                    # original goctl API definition (kept for reference)
└── internal/
    ├── config/config.go         # value-tag binding + RestConf() builder
    ├── server/server.go         # gs.Server adapter around rest.Server
    ├── handler/                 # generated, unchanged
    ├── logic/greetlogic.go      # generated (fleshed out to return a greeting)
    ├── svc/servicecontext.go    # generated, unchanged
    └── types/types.go           # generated, unchanged
```

## How it works

### 1. Config bound from properties

`config.Config` is bound from `conf/app.properties` under the `${greet}` prefix, and adapted to the
`rest.RestConf` that go-zero expects:

```go
type Config struct {
    Name string `value:"${name:=greet-api}"`
    Host string `value:"${host:=0.0.0.0}"`
    Port int    `value:"${port:=8888}"`
}

func (c Config) RestConf() rest.RestConf {
    var rc rest.RestConf
    rc.Name, rc.Host, rc.Port = c.Name, c.Host, c.Port
    return rc
}
```

```properties
spring.http.server.enabled=false   # let go-zero own the port
greet.name=greet-api
greet.host=0.0.0.0
greet.port=8888
```

### 2. go-zero rest.Server as a gs.Server

`GreetServer` wraps `rest.Server`. `StartWithOpts` hands back the underlying `*http.Server` so
`Stop()` can shut it down gracefully through the Go-Spring lifecycle:

```go
func (s *GreetServer) Run(ctx context.Context, sig gs.ReadySignal) error {
    <-sig.TriggerAndWait()
    s.svr.StartWithOpts(func(svr *http.Server) { s.httpSvr = svr })
    return nil
}

func (s *GreetServer) Stop() error {
    if s.httpSvr != nil {
        _ = s.httpSvr.Shutdown(context.Background())
    }
    s.svr.Stop()
    return nil
}
```

### 3. Wiring + startup

```go
func init() {
    gs.Provide(svc.NewServiceContext, gs.IndexArg(0, gs.TagArg("${greet}")))
    gs.Provide(server.NewGreetServer, gs.IndexArg(0, gs.TagArg("${greet}"))).
        Export(gs.As[gs.Server]())
}

func main() { gs.Run() }
```

## Run

```bash
go run .
```

The example starts the server, self-tests `GET /from/you`, prints the response, then triggers a
graceful shutdown:

```
Response from server: {"message":"Hello, you"}
```

Or call it yourself while the server is up:

```bash
curl http://localhost:8888/from/you
# {"message":"Hello, you"}
```
