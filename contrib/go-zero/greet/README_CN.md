# go-zero(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [go-zero](https://go-zero.dev) `Greet` 示例:先用 go-zero 工具链生成代码,
再改造成 Go-Spring 的启动与配置方式 —— 由 `gs.Run()` 驱动生命周期,provider
作为 IoC bean 注入,监听地址取自 `conf/app.properties`,而不是写死在 `main()`
里。

采用 **zrpc(gRPC)** 服务,并接入 **etcd 注册中心** 做真实的**服务注册与发现**:
provider 启动时把 `greet.rpc` 这个 key 注册进 etcd;consumer 不知道 provider
的 host:port,而是从同一 etcd 解析出可用地址再发起调用。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 为什么用 zrpc 而不是 REST —— 与其他示例的关键差异

与其他四个 contrib 示例(dubbo-go、kitex、kratos、goframe)不同,**go-zero 的
REST 服务(`rest.Server`)不内建任何服务发现能力**,注册中心相关能力全部只在
**zrpc**(go-zero 的 gRPC 层)中提供。为了展示 go-zero 真实的服务治理能力,
这个示例必须走 zrpc —— 用 REST 只能是硬编码直连,无法称为「注册发现」。

因此本示例与常见的 go-zero REST 教程结构不同:IDL 使用 protobuf,provider 是
zrpc server,consumer 是 zrpc client。`spring.http.server.enabled=false`。

## 拓扑

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │  greet.rpc  └──────────────┘  greet.rpc  │
  │             (key)              (key)     │ 解析 provider 地址
  │ → grpc://<host>:8081                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      Greet(name)       │  一次性调用 │
│ :8081      │──────────────────────▶│  断言后退出 │
└────────────┘       echo name        └────────────┘
```

## 目录结构

```
contrib/go-zero/greet/
├── greet.proto             # Protobuf IDL
├── pb/greet.pb.go          # protoc 生成的消息(请勿手改)
├── pb/greet_grpc.pb.go     # protoc 生成的 gRPC 桩代码(请勿手改)
├── provider/handler.go     # GreetProvider,导出为 ServiceRegister bean
├── provider/server.go      # ZrpcServer 适配器(gs.Server)+ Config,配置 etcd registry
├── provider/main.go        # gs.Run(),长驻并注册到 etcd
├── consumer/main.go        # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties     # provider 配置
├── docker-compose.yml      # 本地 etcd
└── check.sh                # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

两条路都可以,产物都是标准的 `pb.RegisterGreetServer` / `pb.NewGreetClient`
桩代码,provider 和 consumer 直接引用。

### 方案 A:`protoc`(本示例采用)

```bash
# 工具(一次性)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 从 IDL 生成消息 + gRPC 桩代码
protoc --proto_path=. \
  --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  greet.proto
mv greet.pb.go greet_grpc.pb.go pb/
```

### 方案 B:`goctl rpc protoc`

```bash
# 工具(一次性)
go install github.com/zeromicro/go-zero/tools/goctl@latest

goctl rpc protoc greet.proto \
  --go_out=./pb --go-grpc_out=./pb --zrpc_out=.
```

`goctl` 会额外生成 `etc/*.yaml` 与 `internal/{config,logic,server,svc}` 目录。
本示例有意不使用那套目录 —— 生命周期与配置都交由 Go-Spring 管理,只保留 `pb/`
产物即可。

## 改造:原生 go-zero → Go-Spring + 注册发现

| 关注点   | 原生 go-zero(REST 脚手架)                | Go-Spring 版本(zrpc + etcd)                                                       |
| -------- | ------------------------------------------ | ----------------------------------------------------------------------------------- |
| 传输层   | `rest.Server`(HTTP)                      | `zrpc.RpcServer`(gRPC)—— 服务发现的前提                                            |
| IDL      | `greet.api`                                | `greet.proto` + `pb/*.pb.go` / `pb/*_grpc.pb.go`                                    |
| 启动     | `main()` 中 `server.Start()` 阻塞          | `ZrpcServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop                          |
| handler  | `handler.RegisterHandlers(server, svcCtx)` | `gs.Provide(func() ServiceRegister { return pb.RegisterGreetServer(...) })`         |
| 是否启用 | 总是开启                                   | 通过 `gs.OnBean` 条件依赖 `ServiceRegister` bean                                    |
| 监听地址 | 写死在 YAML                                | 取自 `conf/app.properties` 的 `${spring.zrpc.server.listen-on}`                     |
| 服务注册 | 无(REST 无发现能力)                      | provider `zrpc.RpcServerConf{Etcd: discov.EtcdConf{Hosts, Key}}` 注册进 etcd        |
| 服务发现 | 无                                         | consumer `zrpc.RpcClientConf{Etcd: discov.EtcdConf{Hosts, Key}}`,按 key 从 etcd 解析 |
| 关停     | 进程自持                                   | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,注销 etcd 注册)                     |

`provider/server.go` 里的适配器是关键:`zrpc.RpcServer.Start()` 会绑定监听端口、
把 provider 注册进 etcd 后永久阻塞,因此将其放到一个仅在 `sig.TriggerAndWait()`
之后启动的 goroutine 中运行,`Run` 则阻塞在一个 done channel 上,由 `Stop()`
关闭它,再由 Go-Spring 调用 `zrpc.RpcServer.Stop()` 完成关停。

consumer 侧只提供 etcd 地址与 key,不提供 provider 地址:zrpc 会用同一 key
在 etcd 中查到一个存活的 provider 并调用。

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 zrpc 端点。
spring.http.server.enabled=false

# zrpc 监听地址,经 ${spring.zrpc.server} 前缀读取。
spring.zrpc.server.listen-on=0.0.0.0:8081

# etcd 注册中心地址与 key,与 docker-compose.yml 一致。
spring.zrpc.server.etcd.addr=127.0.0.1:2379
spring.zrpc.server.etcd.key=greet.rpc
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
Response from discovered provider: Hello, go-zero!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash check.sh
```
