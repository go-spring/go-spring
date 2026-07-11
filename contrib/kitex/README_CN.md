# kitex(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [Kitex](https://www.cloudwego.io/docs/kitex/) `EchoService` 示例:先用
`kitex` 脚手架生成,再改造成 Go-Spring 的启动与配置方式 —— 由 `gs.Run()`
驱动生命周期,handler 作为 IoC bean 注入,监听地址取自
`conf/app.properties`,而不是写死在 `main()` 里。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 目录结构

```
contrib/kitex/
├── idl/echo.thrift        # Thrift IDL
├── kitex_gen/echo/...     # Kitex 生成代码(请勿手改)
├── kitex_info.yaml        # 重新生成用的元数据
├── handler.go             # EchoServiceImpl,导出为 echo.EchoService bean
├── server.go              # KitexServer 适配器(gs.Server)+ Config
├── main.go                # gs.Run() + 自测客户端
└── conf/app.properties    # 配置
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/cloudwego/thriftgo@latest
go install github.com/cloudwego/kitex/tool/cmd/kitex@latest

# 从 IDL 生成脚手架
kitex -module go-spring.org/kitex -service echo idl/echo.thrift
```

脚手架会产出 `kitex_gen/`、一个空的 `handler.go`,以及直接调用 `svr.Run()`
的 `main.go`。重新执行该命令只会再生成 `kitex_gen/`,不会覆盖改造后的
`handler.go` / `server.go` / `main.go`。

## 改造:原生 Kitex → Go-Spring

| 关注点   | Kitex 脚手架                     | Go-Spring 版本                                                     |
| -------- | -------------------------------- | ------------------------------------------------------------------ |
| 启动     | `main()` 中 `svr.Run()` 阻塞     | `KitexServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop        |
| handler  | 手动 `new(EchoServiceImpl)`      | `gs.Provide(&EchoServiceImpl{}).Export(gs.As[echo.EchoService]())` |
| 是否启用 | 总是开启                         | 通过 `gs.OnBean` 条件依赖 `echo.EchoService` bean                  |
| 地址     | 写死的默认值                     | 取自 `conf/app.properties` 的 `${spring.kitex.server.addr}`        |
| 关停     | 进程自持                         | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`)                    |

`server.go` 里的适配器是关键:Kitex 的 `server.Run()` 会阻塞并占用进程,因此
将其包装为在 `sig.TriggerAndWait()` 之后才启动,并暴露 `Stop()` 供 Go-Spring
的关停流程调用。

## 配置

```properties
# 关闭内置 HTTP server,本示例只暴露 Kitex 端点。
spring.http.server.enabled=false

# Kitex 监听地址,经 ${spring.kitex.server} 前缀读取,默认 :8888。
spring.kitex.server.addr=:8888
```

## 运行

```bash
go run .
```

`main.go` 会在启动 500ms 后拉起客户端,调用 `Echo("Hello, Kitex!")`,断言回显
内容,然后发送 SIGTERM 让 Go-Spring 优雅关停。预期输出包含:

```
Response from server: Hello, Kitex!
```
