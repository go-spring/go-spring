# go-zero — zRPC/gRPC(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [go-zero](https://go-zero.dev) `Greet` 示例:桩代码由 **protobuf** IDL
通过 `goctl rpc protoc` 生成,再改造成 Go-Spring 的启动与配置方式 —— 由
`gs.Run()` 驱动生命周期,handler 作为 IoC bean 注入,监听地址取自
`conf/app.properties`,而不是写死在 `main()` 里。

服务跑在 **zRPC**(go-zero 的 gRPC 层)之上,并接入 **etcd 注册中心** 做真实的
**服务注册与发现**:provider 启动时把 `greet.rpc` 这个 key 注册进 etcd;
consumer 不知道 provider 的 host:port,而是从同一 etcd 解析出可用地址再发起
调用。

这是 go-zero 示例中的 RPC 半边。HTTP/REST 那一半 —— 同一个 `Greet` 服务,
但由 `.api` 文件通过 `goctl api go` 生成 —— 在旁边的
[`../greet-api`](../greet-api)。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 为什么用 zRPC?为什么 REST 那边没有 etcd?

与其他框架示例(dubbo-go、kitex、kratos、goframe)不同,**go-zero 的 REST
服务(`rest.Server`)不内建任何服务发现能力**,注册中心相关能力只存在于
**zRPC** 中。为了展示 go-zero 真实的服务治理能力,本示例走 zRPC —— REST
版本只能是硬编码直连,谈不上「注册发现」。相邻的 `greet-api/` 因此保留同一
套 Go-Spring 装配模式,但去掉 etcd,由 consumer 直接走 HTTP 调用 provider。

## 拓扑

```
                ┌──────────────┐
    注册        │     etcd     │    发现
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
contrib/go-zero/greet-rpc/
├── greet.proto             # Protobuf IDL
├── gen.sh                  # 基于 greet.proto 通过 goctl 重新生成 pb/
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

```bash
# 工具(一次性)
go install github.com/zeromicro/go-zero/tools/goctl@latest
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

# 从 IDL 生成 pb/(或直接执行 ./gen.sh)
goctl rpc protoc greet.proto --go_out=./pb --go-grpc_out=./pb --zrpc_out=<tmp>
```

`goctl rpc protoc` 默认还会在 `--zrpc_out` 下生成 `etc/*.yaml` 与
`internal/{config,logic,server,svc}` 目录树,那本来是原生 go-zero 项目用来
托管生命周期与配置的部分;本示例把它整个丢弃,只保留 `pb/` 产物 —— 生命周期
与配置交由 Go-Spring 管理。`gen.sh` 把 `--zrpc_out` 指向一个 `mktemp -d`
的临时目录并在退出时删除,重跑不会影响手写的 provider/consumer。

## 改造:原生 go-zero → Go-Spring + 注册发现

| 关注点   | 原生 go-zero zRPC 脚手架                    | Go-Spring 版本(zRPC + etcd)                                                       |
| -------- | ------------------------------------------ | ----------------------------------------------------------------------------------- |
| 启动     | `main()` 中 `server.Start()` 阻塞          | `ZrpcServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop                          |
| handler  | `server.RegisterGreetServer(srv, logic)`   | `gs.Provide(func() ServiceRegister { return pb.RegisterGreetServer(...) })`         |
| 是否启用 | 总是开启                                   | 通过 `gs.OnBean` 条件依赖 `ServiceRegister` bean                                    |
| 监听地址 | 写死在 YAML                                | 取自 `conf/app.properties` 的 `${spring.zrpc.server.listen-on}`                     |
| 服务注册 | main 中直接构造 zrpc.RpcServerConf         | Config 结构体从 `${spring.zrpc.server}` 前缀绑定                                    |
| 关停     | 进程自持                                   | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,注销 etcd 注册)                     |

`provider/server.go` 里的适配器是关键:`zrpc.RpcServer.Start()` 会绑定监听
端口、把 provider 注册进 etcd 后永久阻塞,因此将其放到一个仅在
`sig.TriggerAndWait()` 之后启动的 goroutine 中运行,`Run` 则阻塞在一个 done
channel 上,由 `Stop()` 关闭它,再由 Go-Spring 调用 `zrpc.RpcServer.Stop()`
完成关停。

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 zRPC 端点。
spring.http.server.enabled=false

# zRPC 监听地址,经 ${spring.zrpc.server} 前缀读取。
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
