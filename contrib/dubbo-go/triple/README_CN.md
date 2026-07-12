# dubbo-go — Triple(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个由 **protobuf** IDL 通过 `protoc-gen-go-triple` 生成的
[Dubbo-go](https://dubbo.apache.org/zh-cn/overview/mannual/golang-sdk/)
`GreetService` 示例,并通过可复用的 **starter-dubbo** 模块以 Go-Spring 的方式装配:
由它提供 `gs.Server` 适配器,`gs.Run()` 驱动生命周期,provider 只是一个
`ServiceRegister` bean,协议与注册中心都取自 `conf/app.properties`,而不是
写死在 `main()` 里。

采用 **Triple** 协议 —— Dubbo 在 Go 上的旗舰协议,基于 protobuf-over-HTTP/2,
与 gRPC 线格式兼容;并接入 **etcd 注册中心**做真实的**服务注册与发现**:
provider 启动时把 `greet.GreetService` 注册进 etcd;consumer 不知道 provider
的 host:port,而是从同一 etcd 解析出可用地址再发起调用。这体现的是 Dubbo
标榜的微服务治理能力,而非早期的无注册中心直连。

本示例与经典 Dubbo/Hessian2 版本 [`../dubbo`](../dubbo) 互为补充。dubbo-go v3
推荐使用 Triple;Hessian2 仍保留用于与 Java Dubbo 服务互通。

这是一个**可运行示例**,并非可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ greet.GreetService                       │ 解析 provider 地址
  │ → tri://<host>:20000                     │
┌─┴──────────┐                        ┌──────┴─────┐
│  provider  │◀───────── RPC ─────────│  consumer  │
│ gs.Run()   │      Greet(name)       │  一次性调用 │
│ :20000     │──────────────────────▶│  断言后退出 │
└────────────┘       echo name        └────────────┘
```

## 目录结构

```
contrib/dubbo-go/triple/
├── proto/greet.proto        # Protobuf IDL
├── proto/greet.pb.go        # protoc 生成的消息(请勿手改)
├── proto/greet.triple.go    # Triple 生成的桩代码(请勿手改)
├── gen.sh                   # 从 IDL 重新生成 proto/*.go
├── provider/handler.go      # GreetProvider + StarterDubbo.ServiceRegister bean(server 由 starter-dubbo 提供)
├── provider/main.go         # gs.Run(),长驻并注册到 etcd
├── consumer/main.go         # 通过 etcd 发现 provider,调用并断言后退出
├── conf/app.properties      # provider 配置
├── docker-compose.yml       # 本地 etcd
└── check.sh                 # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
```

## 如何生成

```bash
# 工具(一次性)
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install github.com/dubbogo/protoc-gen-go-triple/v3@latest

# 从 IDL 生成消息 + Triple 桩代码(或直接执行 ./gen.sh)
protoc --proto_path=proto \
  --go_out=paths=source_relative:./proto \
  --go-triple_out=paths=source_relative:./proto \
  proto/greet.proto
```

生成器会在 `proto/` 下产出 `greet.pb.go` 和 `greet.triple.go`;`proto/` 由
provider 与 consumer 共享。重新执行 `./gen.sh` 只会再生成这两个文件,不会覆盖
改造后的业务代码。

> 注意:在 `runtime.Version()` 带实验后缀(如 `go1.26.1-X:jsonv2`)的 go1.26
> 工具链上,`protoc-gen-go-triple` v3.0.3 会在解析版本时 panic。需从源码重新
> 编译,并把版本串截断为纯数字部分。

## 改造:原生 Dubbo-go → Go-Spring + 注册发现

| 关注点   | Dubbo-go 脚手架                            | Go-Spring 版本                                                            |
| -------- | ------------------------------------------ | ------------------------------------------------------------------------- |
| 启动     | `main()` 中 `srv.Serve()` 阻塞             | starter-dubbo 的 `DubboServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop |
| handler  | `RegisterGreetServiceHandler(srv, &impl)`  | `gs.Provide(func() StarterDubbo.ServiceRegister { ... })`,服务无关地绑定 |
| 是否启用 | 总是开启                                   | 通过 `gs.OnBean` 条件依赖 `ServiceRegister` bean                          |
| 端口     | 写死的默认值                               | 取自 `conf/app.properties` 的 `${spring.dubbo.server.protocols.tri.port}` |
| 服务注册 | 无(直连)                                | map 驱动的 `${spring.dubbo.server.registries.etcdv3}` 配置 → 注册进 etcd  |
| 服务发现 | consumer `WithClientURL("host:port")` 直连 | consumer `client.WithClientRegistry(...)`,按接口名从 etcd 解析地址        |
| 关停     | 进程自持                                   | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()`,注销 etcd 注册)           |

`gs.Server` 适配器位于可复用的 **starter-dubbo** 模块,这是关键:Dubbo-go 的
`Serve()` 会绑定监听端口、把 provider 注册进 etcd 后永久阻塞,因此 starter 将其
放到一个仅在 `sig.TriggerAndWait()` 之后启动的 goroutine 中运行,`Run` 则阻塞在
一个 done channel 上,由 `Stop()` 关闭它,把控制权交回 Go-Spring 的关停流程。

consumer 侧只提供 etcd 地址,不提供 provider 地址:`greet.GreetService` 这个
接口名由生成的桩代码内置,Dubbo 据此在 etcd 中查到一个存活的 provider 并调用。

## 注册中心的选择

本示例统一用 **etcd** 便于与其他 contrib 示例横向对比。Dubbo-go 原生同样支持
**Nacos**、**ZooKeeper**、**Polaris** 等:只需在 `${spring.dubbo.server.registries}`
下按 dubbo-go 的注册中心名(`nacos` / `zookeeper` / `polaris`)再加一个条目并填上
`address`,同时把 consumer 的对应选项换掉即可。选用 Nacos 时还能通过其自带的
`:8848/nacos` 控制台直接查看已注册的服务列表。

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 Dubbo 端点。
spring.http.server.enabled=false

# 由 starter-dubbo 装配的 Dubbo server。协议是 map 驱动的:
# ${spring.dubbo.server.protocols} 下的 key 即 dubbo-go 协议名,这里是 20000 上的 Triple。
spring.dubbo.server.protocols.tri.port=20000

# etcd 注册中心,map 驱动:${spring.dubbo.server.registries} 下的 key 即
# dubbo-go 注册中心名。与 docker-compose.yml 一致。
spring.dubbo.server.registries.etcdv3.address=127.0.0.1:2379
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
Response from discovered provider: Hello, Dubbo-Go!
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash check.sh
```
