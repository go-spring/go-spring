# goframe — GoFrame 改造为 Go-Spring

[English](README.md) | [中文](README_CN.md)

一个用 `gf init` 生成的标准 [GoFrame](https://goframe.org) 项目,把它从 GoFrame 原生的配置加载与启动方式,
改造为 **Go-Spring** 的配置绑定与生命周期管理。

生成的业务代码(`api/`、`internal/controller`、`internal/consts` 等)保持不动 —— 只改变*服务如何被配置和启动*。

这是一个可运行的示例,**不是**可复用的 starter 模块。

## 改造对照

| 关注点 | 原生 GoFrame | 改造为 Go-Spring |
| --- | --- | --- |
| 配置来源 | `manifest/config/config.yaml`,由 `g.Cfg()` 隐式加载 | `conf/app.properties`,通过 `value:"${...}"` tag 绑定 |
| 配置结构 | 无(服务直接读 YAML) | `internal/config.Config`,使用 `value` tag |
| 服务创建 | `internal/cmd` 内联 `g.Server()` | `internal/server.NewGoFrameServer`,一个 `gs.Server` bean |
| 路由注册 | `internal/cmd` 内联 `s.Group(...)` | 放进 server bean 构造函数 |
| 启动 | `cmd.Main.Run()` → `s.Run()` 在 `main()` 里阻塞 | `gs.Run()` 驱动容器生命周期 |
| 关闭 | `s.Run()` 自带的信号处理 | 由 Go-Spring 调用 `GoFrameServer.Stop()` → `s.Shutdown()` |

## 目录结构

```
contrib/goframe/
├── conf/app.properties          # Go-Spring 配置(替代 config.yaml 的 server: 段)
├── main.go                      # main():bean 注册 + gs.Run()
├── api/hello/                   # 生成的 API 定义,未改动
└── internal/
    ├── config/config.go         # value-tag 绑定(新增)
    ├── server/server.go         # 包装 *ghttp.Server 的 gs.Server 适配器(新增)
    └── controller/hello/        # 生成的业务代码,未改动
```

`internal/cmd` —— GoFrame 脚手架的启动入口 —— 已删除:它的 `g.Server()`、路由绑定与 `s.Run()`
职责都搬进了 server bean。

## 如何生成

```bash
# 安装工具(一次)
go install github.com/gogf/gf/cmd/gf/v2@latest

# 生成 single-repo 模板
gf init goframe -g go-spring.org/goframe
```

## 工作原理

### 1. 配置从 properties 绑定

`config.Config` 从 `conf/app.properties` 的 `${goframe}` 前缀绑定,替代 GoFrame 的
`manifest/config/config.yaml`:

```go
type Config struct {
    Address string `value:"${address:=:8000}"`
}
```

```properties
spring.http.server.enabled=false   # 让 goframe 独占端口
goframe.address=:8000
```

### 2. 把 GoFrame *ghttp.Server 变成 gs.Server

`GoFrameServer` 包装 `g.Server()`,从绑定的配置里设置地址,并注册脚手架原本在 `internal/cmd`
里的路由。GoFrame 的 `Start()` 是非阻塞的,所以 `Run` 会阻塞到 `Stop` 调用 `Shutdown()`:

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

### 3. 装配与启动

```go
func init() {
    gs.Provide(server.NewGoFrameServer, gs.IndexArg(0, gs.TagArg("${goframe}"))).
        Export(gs.As[gs.Server]())
}

func main() { gs.Run() }
```

## 运行

```bash
go run .
```

示例会启动服务,自测 `GET /hello`,打印响应,然后触发优雅关闭:

```
Response from server: Hello World!
```

也可以在服务运行时自己调用:

```bash
curl http://localhost:8000/hello
# Hello World!
```
