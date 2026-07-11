# go-kratos(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [Kratos](https://go-kratos.dev/) `Greeter` 示例:从 `kratos` 工具链脚手架
(`kratos new`)生成的项目出发,再改造成 Go-Spring 的启动与配置方式——由
`gs.Run()` 驱动生命周期,各层通过 Go-Spring IoC 容器装配(取代 `google/wire`),
服务监听地址来自 `conf/app.properties`(取代 Kratos 的 YAML 配置)。

Kratos 完整的分层结构(`api` + `internal/{biz,data,service,server}`)被完整保留,
仅替换**启动、依赖注入、配置**三处引擎。示例同时暴露脚手架生成的 **HTTP**
(`:8000`)与 **gRPC**(`:9000`)两个 Greeter 端点。

这是一个可运行的示例,**不是**可复用的 starter 模块。

## 目录结构

```
contrib/go-kratos/
├── api/helloworld/v1/          # protoc 生成的 gRPC + HTTP stub(请勿编辑)
├── internal/biz/               # 业务逻辑(GreeterUsecase + GreeterRepo 接口)
├── internal/data/              # 数据层(Data、greeterRepo)+ 共享的 kratos logger bean
├── internal/service/           # 服务层(GreeterService)
├── internal/server/            # HTTPServer / GRPCServer 适配器(gs.Server)+ Config
├── main.go                     # gs.Run() + 自测客户端(HTTP 与 gRPC)
└── conf/app.properties         # 配置
```

## 如何生成

```bash
# 工具(一次性)
go install github.com/go-kratos/kratos/cmd/kratos/v2@latest

# 生成项目(会克隆 kratos-layout 模板)
kratos new go-kratos
```

脚手架会产出 `cmd/`(`wire` + `kratos.App` 启动)、`configs/config.yaml`、
`internal/conf/`(`conf.proto` 的 Bootstrap 消息)以及分层的 `internal/` 代码。
改造删除了 `cmd/`、`configs/`、`internal/conf/` 及 `wire` 文件,保留生成的
`api/` stub 原样不动,并重连其余部分。

## 改造对照:原生 Kratos → Go-Spring

| 关注点         | Kratos 脚手架                                          | Go-Spring 版本                                                             |
| -------------- | ------------------------------------------------------ | -------------------------------------------------------------------------- |
| 启动           | `kratos.New(...).Run()` 占有进程                       | 每个 server 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop                  |
| 依赖装配       | `google/wire` `ProviderSet` + 生成的 `wire_gen.go`     | 每层 `init()` + `gs.Provide`;`main.go` 用空导入触发注册                    |
| server 注册    | `wire.NewSet(NewGRPCServer, NewHTTPServer)`            | 每个 transport server 用 `gs.Provide(...).Export(gs.As[gs.Server]())`      |
| 接口绑定       | `NewGreeterRepo` 返回 `biz.GreeterRepo` 供 wire 使用   | 同一构造函数;容器按返回类型解析为 `biz.GreeterRepo` bean                   |
| 配置来源       | `configs/config.yaml` 扫描进 `conf.proto` `Bootstrap`  | `conf/app.properties`,经 `value:"${spring.kratos.http}"` 结构体绑定       |
| 关闭           | `kratos.App` 优雅停止                                  | 由 Go-Spring 协调优雅关闭(SIGTERM → `Stop()`)                            |

`internal/server/{http,grpc}.go` 里的适配器是关键:Kratos transport server 的
`Start(ctx)` 会绑定监听并阻塞,直到 `Stop(ctx)` 触发优雅关闭;因此 `Run` 只需在
`sig.TriggerAndWait()` 之后调用 `Start`,`Stop` 则委托给 Kratos server 的 `Stop`。

## 配置

```properties
# 关闭内置 HTTP server;本示例只运行两个 kratos server。
spring.http.server.enabled=false

# kratos HTTP transport server,经 ${spring.kratos.http} 前缀绑定。
spring.kratos.http.addr=0.0.0.0:8000
spring.kratos.http.timeout=1s

# kratos gRPC transport server,经 ${spring.kratos.grpc} 前缀绑定。
spring.kratos.grpc.addr=0.0.0.0:9000
spring.kratos.grpc.timeout=1s
```

## 运行

```bash
go run .
```

`main.go` 在启动 500ms 后拉起客户端,分别通过 HTTP(`GET /helloworld/Kratos`)
与 gRPC(`SayHello`)调用 Greeter,断言返回值后发送 SIGTERM 让 Go-Spring 优雅退出。
预期输出包含:

```
HTTP response from server: Hello Kratos
gRPC response from server: Hello Kratos
```
