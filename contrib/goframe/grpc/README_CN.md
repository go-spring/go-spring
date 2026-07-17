# goframe — gRPC(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个由 **protobuf** IDL(用 `protoc` + 标准 Go 插件生成)、并由 goframe 的
gRPC 层(`github.com/gogf/gf/contrib/rpc/grpcx/v2`)提供的
[GoFrame](https://goframe.org) `EchoService` 示例,改造成 Go-Spring 的启动与
配置方式:由 `gs.Run()` 驱动生命周期,`grpcx.GrpcServer` 作为 IoC bean 注入,
handler 导出为 `echo.EchoServiceServer` bean,监听地址取自
`conf/app.properties`,而非 `manifest/config/config.yaml`。

同时接入 **etcd 注册中心**(通过 `github.com/gogf/gf/contrib/registry/etcd/v2`)
实现真正的**服务注册与发现**:provider 启动时把 `goframe.grpc.echo` 注册进
etcd;consumer 不知道 provider 的 host:port,而是通过 grpcx 的 discovery
resolver 从同一 etcd 解析出可用地址。

这是 **gRPC** 协议版本。HTTP 版本(goframe `*ghttp.Server` + `gf gen ctrl`
代码生成链)见 [`../http`](../http)。两者拆开是因为 goframe 用了两种不同的
server 与两条不同的 codegen 流水线,没必要硬塞进一个 provider。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
    注册        │     etcd     │    发现
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ goframe.grpc.echo                        │ 解析 provider 地址
  │ → <host>:8001                            │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── gRPC ────────│  consumer  │
│ gs.Run()   │      Echo(message)     │  一次性调用 │
│ :8001      │──────────────────────▶│  断言后退出 │
└────────────┘       echo message     └────────────┘
```

## 目录结构

```
contrib/goframe/grpc/
├── idl/echo.proto              # protobuf IDL
├── idl/echo/                   # protoc 生成的 Go 代码(请勿手改)
├── idl/gen-code.sh             # 从 IDL 重新生成 idl/echo/
├── provider/handler.go         # EchoServiceImpl,导出为 echo.EchoServiceServer bean
├── provider/server.go          # GoFrameGrpcServer 适配器(gs.Server)+ Config,配置 etcd registry
├── provider/main.go            # gs.Run(),长驻并注册到 etcd
├── consumer/main.go            # 通过 etcd 发现,调用 Echo 并断言后退出
├── conf/app.properties         # provider 配置
├── docker-compose.yml          # 本地 etcd
└── scripts/smoke-test.sh       # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 从 IDL 重新生成 gRPC 代码(或直接执行 ./idl/gen-code.sh)
protoc \
    --proto_path=idl \
    --go_out=. \
    --go_opt=module=go-spring.org/goframe/grpc \
    --go-grpc_out=. \
    --go-grpc_opt=module=go-spring.org/goframe/grpc \
    idl/echo.proto
```

`echo.proto` 中的 `option go_package = "go-spring.org/goframe/grpc/idl/echo;echo";`
把生成路径固定到 module 根目录下的 `idl/echo/`。重新执行 `./idl/gen-code.sh`
只会覆盖 `idl/echo/`,不会碰改造后的 provider/consumer 代码。

与 goframe 的 HTTP `gf gen ctrl`(解析 `api/*/v*/` 类型再生成 controller)不同,
gRPC 的代码生成走的是原生 `protoc` + `protoc-gen-go` + `protoc-gen-go-grpc`。
`gf gen pb` 只是同一命令的薄封装,不是本示例的必需项。

## 改造:原生 grpcx → Go-Spring + 注册发现

| 关注点   | grpcx 脚手架                                                                | Go-Spring 版本                                                                                                        |
| -------- | --------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- |
| 启动     | `s.Run()` 在 `main()` 中阻塞,自持 `gproc` 信号处理                          | `GoFrameGrpcServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop;用 `s.Start()` + park-on-done                        |
| Handler  | `echo.RegisterEchoServiceServer(s.Server, &EchoServiceImpl{})` 直接写在 main | `gs.Provide(&EchoServiceImpl{}).Export(gs.As[echo.EchoServiceServer]())`,构造函数中完成 `Register…`                    |
| 是否启用 | 始终启用                                                                     | 通过 `gs.OnBean` 条件化:存在 `echo.EchoServiceServer` bean 时才启用                                                    |
| 地址     | `grpcx.Server.NewConfig()` 从 `config.yaml` 读 `grpc.address`                | 从 `conf/app.properties` 通过 `${goframe.grpc.address}` 读取                                                            |
| 服务注册 | 无(直连)                                                                    | provider 在 `grpcx.Server.New` 前调用 `gsvc.SetRegistry(etcd.New(addr))`                                                |
| 服务发现 | consumer `grpc.NewClient("host:port")`                                       | consumer `grpcx.Client.MustNewGrpcClientConn(serviceName)`(gsvc scheme 通过 etcd 解析)                                 |
| 关停     | `gproc` 自持                                                                | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,`GracefulStop` + etcd 注销)                                              |

`provider/server.go` 中的适配器是关键。`grpcx.GrpcServer` 在构造时会读取一次
`gsvc.GetRegistry()`(见 grpcx 源码中的 `registrar: gsvc.GetRegistry()`),
因此构造函数在调用 `grpcx.Server.New` 之前先设置好 etcd registry。
`s.Start()` 非阻塞,所以 `Run` 阻塞在一个 done channel 上,由 `Stop()` 关闭以
把控制权交回 Go-Spring 的关停流程,后者进一步触发 `s.Stop()`(GracefulStop
+ etcd 注销)。使用 `Start` 而非 `Run` 是有意的:`grpcx.Server.Run` 会安装
自己的 `gproc` 信号处理,与 Go-Spring 的生命周期冲突。

consumer 侧只知道 etcd 地址,不知道 provider 的 host:port:它把服务名
(`goframe.grpc.echo`,与 `conf/app.properties` 中的 `goframe.grpc.name` 一致)
传给 `grpcx.Client.MustNewGrpcClientConn`,后者构造 `gsvc://<name>` 目标,
由 grpcx 的 resolver 从 etcd 解析。

## 注册中心的选择

本示例统一使用 **etcd**,便于与其他 contrib 示例横向对比。
`github.com/gogf/gf/contrib/registry/*` 同样提供 **Nacos**、**ZooKeeper** 与
**Polaris** 适配器,均满足同一个 `gsvc.Registry` 接口:把
`github.com/gogf/gf/contrib/registry/etcd/v2` 换成
`.../registry/nacos/v2` / `.../registry/zookeeper/v2` / `.../registry/polaris/v2`,
再相应地改 `goframe.grpc.registry.etcd` 即可。

## 配置

```properties
# 关闭 Go-Spring 内置 HTTP server,provider 只暴露 gRPC。
spring.http.server.enabled=false

# grpcx.GrpcServer 监听地址。
goframe.grpc.address=:8001

# provider 注册使用的服务名;consumer 从 etcd 中按此名解析地址。
goframe.grpc.name=goframe.grpc.echo

# etcd 注册中心地址,与 docker-compose.yml 一致。
goframe.grpc.registry.etcd=127.0.0.1:2379
```

## 运行

先起注册中心:

```bash
docker compose up -d      # 或 docker-compose up -d
```

终端 A —— 启动 provider(长驻,注册进 etcd):

```bash
go run ./provider
```

终端 B —— 启动 consumer(从 etcd 发现并调用):

```bash
go run ./consumer
```

consumer 预期输出:

```
Response from discovered provider: Hello, GoFrame gRPC!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash scripts/smoke-test.sh
```
