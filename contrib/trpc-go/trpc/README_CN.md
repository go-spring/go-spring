# trpc-go — trpc(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个由 **protobuf** IDL 生成的 [tRPC-Go](https://github.com/trpc-group/trpc-go)
`GreetService` 示例,并改造成 Go-Spring 的启动与配置方式:由 `gs.Run()` 驱动
生命周期,handler 是 IoC bean,所有配置来自 `conf/app.properties`,而非
tRPC 原生的 `trpc_go.yaml`。

本示例刻意展示的设计取舍:把 tRPC-Go 的配置统一进 Go-Spring 的属性体系。
contrib 本地的适配包 `trpcgs/` 会把 `spring.trpc.server` 前缀下的属性翻译
为一个 tRPC 的 `*Config`,再调用 `trpc.NewServerWithConfig(cfg)` —— **完全
不使用 `trpc_go.yaml` 文件**。server 生命周期被包装成 `gs.Server`,融入
Go-Spring 的启停管线;handler `GreetServiceImpl` 注册为 Go-Spring IoC bean
(一个 `trpcgs.ServiceRegister` bean),与 `contrib/kitex` 挂载 service 的
方式保持一致。

这是本目录下的第一个 tRPC-Go 示例,直接使用**直连**方式拨号
(`ip://127.0.0.1:8000`),不引入任何注册中心,因此运行时无需 docker/etcd。

这是一个可运行示例,**不是**可复用的 starter 模块。

## 拓扑

```
┌────────────────┐                              ┌────────────────┐
│    provider    │                              │    consumer    │
│  gs.Run()      │◀──── tRPC / protobuf ────────│   一次性       │
│  :8000         │       ip://127.0.0.1:8000    │  断言后退出    │
└────────────────┘                              └────────────────┘
       ▲                                                │
       │ trpcgs.SimpleTrpcServer                        │ NewGreetServiceClientProxy
       │ (gs.Server 适配器)                             │ (client.WithTarget)
       │ 从 spring.trpc.server.*                        │
       │ 组装 *Config                                   │
```

## 目录结构

```
contrib/trpc-go/trpc/
├── idl/greet.proto              # protobuf IDL(package trpc.helloworld.greet)
├── idl/greet.pb.go              # 生成产物(请勿手改)
├── idl/greet.trpc.go            # 生成的 tRPC 桩代码(请勿手改)
├── idl/gen-code.sh              # 重新生成 greet.pb.go / greet.trpc.go
├── trpcgs/config.go             # Config + ServiceRegister(属性 → tRPC *Config)
├── trpcgs/server.go             # SimpleTrpcServer:gs.Server 适配器,包住 trpc.NewServerWithConfig
├── trpcgs/logbridge.go          # log.SetLogger 桥接:把 tRPC 日志接入 go-spring log
├── provider/handler.go          # GreetServiceImpl,导出为 trpcgs.ServiceRegister bean
├── provider/main.go             # gs.Run()
├── provider/conf/app.properties # provider 配置
├── consumer/main.go             # 直连客户端,断言响应后发 SIGTERM 退出
├── consumer/conf/app.properties # consumer 配置
└── scripts/smoke-test.sh        # 冒烟脚本:构建并启动 provider,跑 consumer,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install trpc.group/trpc-go/trpc-cmdline/trpc@latest

# 从 IDL 生成桩代码(或直接执行 ./idl/gen-code.sh)
cd idl && trpc create --rpconly --protofile greet.proto -o .
```

`--rpconly` 让 tRPC CLI **只**产出 RPC 桩(`greet.pb.go`、`greet.trpc.go`),
而不是带有自己 `main.go` / `trpc_go.yaml` 的完整脚手架。这两个文件由
provider 与 consumer 共享。重跑 `./idl/gen-code.sh` 只会重新生成它们,
不会覆盖改造后的 provider/consumer/`trpcgs` 代码。

## 配置

Provider(`provider/conf/app.properties`):

```properties
# 关闭内置 HTTP server;provider 只暴露 tRPC。
spring.http.server.enabled=false

# tRPC 绑定地址;通过 ${spring.trpc.server} 前缀读取。
spring.trpc.server.addr=127.0.0.1:8000

# tRPC server 注册的服务名(与 proto 中 package.Service 对应)。
spring.trpc.server.service.name=trpc.helloworld.greet.GreetService
```

`trpcgs/logbridge.go` 通过 `log.SetLogger` 把 tRPC-Go 内部日志转发到
go-spring 的 log 模块。这个首版示例不带任何可观测后端(直连、无 docker),
因此没有配置 `FileLogger` sink:桥接后的 tRPC 日志会落到 go-spring 默认的
`ConsoleLogger`,直接在 stdout 上可见地证明桥接生效。可观测版本可在此加一个
`logging.logger.root` 的 `FileLogger`,把它们路由到 file → Promtail → Loki 管线。

Consumer(`consumer/conf/app.properties`):

```properties
spring.http.server.enabled=false

# 直连目标地址;本示例不接注册中心。
spring.trpc.consumer.target=ip://127.0.0.1:8000
```

## 信号处理 —— 一个注意点

tRPC-Go 的 `server.Serve()` 会自己注册 OS 信号处理器
(`SIGINT`、`SIGTERM`、`SIGSEGV`、`SIGUSR2`)。它与 Go-Spring 的生命周期
共存:Go-Spring 关停时会调用 `SimpleTrpcServer.Stop()`,后者进而调用
`server.Close(nil)` 以干净地解阻塞 `Serve()`。把 tRPC-Go 嵌入到另一个
生命周期宿主内时,需注意这一"信号共管"的事实。

## 运行

终端 A —— 启动 provider(长驻):

```bash
go run ./provider
```

终端 B —— 启动 consumer(直连 `127.0.0.1:8000`,断言后退出):

```bash
go run ./consumer
```

预期 consumer 输出:

```
response from provider: Hello, Go-Spring!
```

或运行一次性冒烟脚本(构建并启动 provider,等待 `8000` 端口就绪,跑
consumer,然后全部清理 —— 无需 docker):

```bash
bash scripts/smoke-test.sh
```
