# greet —— 从 go-zero 改造为 Go-Spring

[English](README.md) | [中文](README_CN.md)

一个用 `goctl api new greet` 生成的原生 [go-zero](https://github.com/zeromicro/go-zero) API 项目，
随后把 go-zero 原生的**配置加载与启动方式**改造为 **Go-Spring** 的配置绑定与生命周期管理。

生成的业务代码（`internal/handler`、`internal/logic`、`internal/svc`、`internal/types`）保持不动，
只改变**服务如何被配置和启动**。

## 改造了什么

| 关注点 | 原生 go-zero | 改造为 Go-Spring |
| --- | --- | --- |
| 配置来源 | `etc/greet-api.yaml`，`conf.MustLoad` 加载 | `conf/app.properties`，`value:"${...}"` 标签绑定 |
| 配置结构 | `config.Config` 内嵌 `rest.RestConf` | `config.Config` 用 `value` 标签 + `RestConf()` 构造器 |
| Server 创建 | `main()` 内 `rest.MustNewServer` | `internal/server.NewGreetServer`，注册为 `gs.Server` bean |
| 路由注册 | `main()` 内 `handler.RegisterHandlers` | 在 server bean 构造器内完成 |
| ServiceContext | `main()` 内 `svc.NewServiceContext` | 注册为 Go-Spring bean，配置自动注入 |
| 启动 | `flag.Parse()` → `conf.MustLoad` → `server.Start()` | `gs.Run()` 驱动容器生命周期 |
| 关闭 | go-zero 内部信号处理 | `GreetServer.Stop()` 优雅关闭 `*http.Server` |

## 目录结构

```
greet/
├── conf/app.properties          # Go-Spring 配置（替代 etc/greet-api.yaml）
├── greet.go                     # main()：bean 注册 + gs.Run()
├── greet.api                    # 原始 goctl API 定义（保留供参考）
└── internal/
    ├── config/config.go         # value 标签绑定 + RestConf() 构造器
    ├── server/server.go         # 包裹 rest.Server 的 gs.Server 适配器
    ├── handler/                 # 生成代码，未改动
    ├── logic/greetlogic.go      # 生成代码（补全为返回问候语）
    ├── svc/servicecontext.go    # 生成代码，未改动
    └── types/types.go           # 生成代码，未改动
```

## 工作原理

### 1. 从 properties 绑定配置

`config.Config` 以 `${greet}` 前缀从 `conf/app.properties` 绑定，并适配为 go-zero 需要的
`rest.RestConf`：

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
spring.http.server.enabled=false   # 让 go-zero 独占端口
greet.name=greet-api
greet.host=0.0.0.0
greet.port=8888
```

### 2. 把 go-zero rest.Server 适配为 gs.Server

`GreetServer` 包裹 `rest.Server`。`StartWithOpts` 会回传底层 `*http.Server`，从而让 `Stop()`
能通过 Go-Spring 生命周期优雅关闭：

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

### 3. 装配与启动

```go
func init() {
    gs.Provide(svc.NewServiceContext, gs.IndexArg(0, gs.TagArg("${greet}")))
    gs.Provide(server.NewGreetServer, gs.IndexArg(0, gs.TagArg("${greet}"))).
        Export(gs.As[gs.Server]())
}

func main() { gs.Run() }
```

## 运行

```bash
go run .
```

示例会启动服务，自测 `GET /from/you`，打印响应，然后触发优雅关闭：

```
Response from server: {"message":"Hello, you"}
```

也可在服务运行时自行调用：

```bash
curl http://localhost:8888/from/you
# {"message":"Hello, you"}
```
