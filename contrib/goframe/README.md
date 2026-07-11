# goframe — GoFrame converted to Go-Spring

[English](README.md) | [中文](README_CN.md)

A stock [GoFrame](https://goframe.org) project, generated with `gf init`, then converted from
GoFrame's native config loading and startup to **Go-Spring**'s configuration binding and lifecycle.

The generated business code (`api/`, `internal/controller`, `internal/consts`, …) is left
untouched — only *how the service is configured and started* changes.

This is a runnable example, **not** a reusable starter module.

## What changed

| Concern | Stock GoFrame | Converted to Go-Spring |
| --- | --- | --- |
| Config source | `manifest/config/config.yaml` loaded implicitly by `g.Cfg()` | `conf/app.properties` bound via `value:"${...}"` tags |
| Config struct | none (server reads YAML directly) | `internal/config.Config` with `value` tags |
| Server creation | `g.Server()` inline in `internal/cmd` | `internal/server.NewGoFrameServer`, a `gs.Server` bean |
| Route registration | `s.Group(...)` inline in `internal/cmd` | done inside the server bean constructor |
| Startup | `cmd.Main.Run()` → `s.Run()` blocks in `main()` | `gs.Run()` drives the container lifecycle |
| Shutdown | `s.Run()`'s own signal handling | `GoFrameServer.Stop()` calls `s.Shutdown()` via Go-Spring |

## Layout

```
contrib/goframe/
├── conf/app.properties          # Go-Spring config (replaces the server: section of config.yaml)
├── main.go                      # main(): bean registration + gs.Run()
├── api/hello/                   # generated API definition, unchanged
└── internal/
    ├── config/config.go         # value-tag binding (new)
    ├── server/server.go         # gs.Server adapter around *ghttp.Server (new)
    └── controller/hello/        # generated, unchanged
```

`internal/cmd` — GoFrame's scaffold startup — is removed: its `g.Server()` + route binding +
`s.Run()` responsibilities move into the server bean.

## How it was generated

```bash
# tool (once)
go install github.com/gogf/gf/cmd/gf/v2@latest

# scaffold the single-repo template
gf init goframe -g go-spring.org/goframe
```

## How it works

### 1. Config bound from properties

`config.Config` is bound from `conf/app.properties` under the `${goframe}` prefix instead of
GoFrame's `manifest/config/config.yaml`:

```go
type Config struct {
    Address string `value:"${address:=:8000}"`
}
```

```properties
spring.http.server.enabled=false   # let goframe own the port
goframe.address=:8000
```

### 2. GoFrame *ghttp.Server as a gs.Server

`GoFrameServer` wraps `g.Server()`, sets the address from the bound config, and binds the same
routes the scaffold registered in `internal/cmd`. GoFrame's `Start()` is non-blocking, so `Run`
blocks until `Stop` calls `Shutdown()`:

```go
func (s *GoFrameServer) Run(ctx context.Context, sig gs.ReadySignal) error {
    <-sig.TriggerAndWait()
    if err := s.svr.Start(); err != nil {
        return err
    }
    <-s.done
    return nil
}

func (s *GoFrameServer) Stop() error {
    err := s.svr.Shutdown()
    close(s.done)
    return err
}
```

### 3. Wiring + startup

```go
func init() {
    gs.Provide(server.NewGoFrameServer, gs.IndexArg(0, gs.TagArg("${goframe}"))).
        Export(gs.As[gs.Server]())
}

func main() { gs.Run() }
```

## Run

```bash
go run .
```

The example starts the server, self-tests `GET /hello`, prints the response, then triggers a
graceful shutdown:

```
Response from server: Hello World!
```

Or call it yourself while the server is up:

```bash
curl http://localhost:8000/hello
# Hello World!
```
