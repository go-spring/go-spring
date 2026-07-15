# go-kratos(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个 [Kratos](https://go-kratos.dev/) `Greeter` 示例:从 `kratos` 工具链脚手架
生成的项目出发,再改造成 Go-Spring 的启动与配置方式——由 `gs.Run()` 驱动生命周期,
各层通过 Go-Spring IoC 容器装配(取代 `google/wire`),服务监听地址来自
`conf/app.properties`(取代 Kratos 的 YAML 配置)。

示例暴露 kratos 的三种 transport:脚手架生成的 **HTTP**(`:8000`)与 **gRPC**
(`:9000`)Greeter 端点,以及来自
[`kratos-transport`](https://github.com/tx7do/kratos-transport) 生态的
**WebSocket**(`:9002`)端点;并接入 **etcd 注册中心**做真实的**服务注册与发现**:
provider 启动时把 `kratos-greeter` 这个 kratos.App 注册进 etcd;consumer 不知道
provider 的 host:port,而是通过 kratos 的 `discovery:///` scheme 从同一 etcd
解析出可用地址再发起调用。这体现的是 Kratos 标榜的微服务治理能力,而非早期的
无注册中心直连。

三种 transport 之所以能装进同一个 kratos.App,是因为它们都实现 kratos 的
`transport.Server` 接口,可以共存;不能共存的 transport(比如需要外部
mosquitto broker 的 MQTT)才值得拆到独立子目录,kitex 示例就是按这个规则做的。

这是一个可运行的示例,**不是**可复用的 starter 模块。

## 拓扑

```
                ┌──────────────┐
   register     │     etcd     │   discover
  ┌────────────▶│  :2379       │◀────────────┐
  │             └──────────────┘             │
  │ kratos-greeter                           │ 解析 provider 地址
  │ → grpc://<host>:9000                     │
  │ → http://<host>:8000                     │
  │ → ws://<host>:9002                       │
┌─┴────────────────┐                    ┌──────┴─────┐
│  provider        │◀──── gRPC ────────│  consumer  │
│  gs.Run()        │  SayHello(name)   │  一次性调用 │
│  :8000/:9000/    │                   │  断言后退出 │
│  :9002 (ws)      │◀── WebSocket ─────│            │
│                  │  {type:1,name}    │            │
└──────────────────┘                    └────────────┘
```

## 目录结构

```
contrib/go-kratos/
├── api/helloworld/v1/          # protoc 生成的 gRPC + HTTP stub(请勿编辑)
├── internal/biz/               # 业务逻辑(GreeterUsecase + GreeterRepo 接口)
├── internal/data/              # 数据层(Data、greeterRepo)+ 共享的 kratos logger bean
├── internal/service/           # 服务层(GreeterService)
├── provider/handler.go         # ServiceRegister bean,把 GreeterService 同时
│                               #   绑定到 HTTP、gRPC、WebSocket 三个 transport
├── provider/server.go          # KratosServer 适配器(gs.Server)+ Config,组合
│                               #   kratos.App 与三个 transport,并注入 etcd Registrar
├── provider/main.go            # gs.Run(),长驻并注册到 etcd
├── consumer/main.go            # 从 etcd 发现 provider,先走 gRPC 调 SayHello,
│                               #   再拨 WebSocket 做一次断言,失败即非零退出
├── conf/app.properties         # provider 配置
├── docker-compose.yml          # 本地 etcd
└── scripts/smoke-test.sh       # 冒烟脚本:起 etcd+provider,跑 consumer,自动清理
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
`api/` stub 原样不动,并按 provider + consumer 双进程模式重连其余部分。

`api/helloworld/v1/*.pb.go`、`*_grpc.pb.go`、`*_http.pb.go` 这些 stub 可以通过
运行 `./scripts/gen-code.sh` 从 `.proto` 重新生成(内部是对 `kratos proto client` 的薄封装)。
同一份 `.proto` 会同时产出 HTTP 与 gRPC stub,一个 Kratos `App` 还会同时承载
WebSocket transport —— 这就是为什么与 kitex 示例不同,本项目**不**按协议拆成
子目录。WebSocket 承载的是应用自定义的消息帧而非 proto RPC,所以它的
请求/响应结构手写在 `provider/handler.go`(见 `WSHelloRequest` / `WSHelloReply`),
和 consumer 通过一份文本 envelope 约定共用,没有额外的 codegen 步骤。

## 为什么 WebSocket 放这里、MQTT 不放

[`kratos-transport`](https://github.com/tx7do/kratos-transport) 生态在 Kratos
之上补充了很多 transport —— WebSocket、MQTT、NATS、Kafka、RabbitMQ 等。它们都
实现同一份 `transport.Server` 接口,理论上都能塞进 `kratos.Server(...)` 与
HTTP+gRPC 并列。本仓的原则是:**能共存的协议放同一个项目,不能共存的才拆
子目录**。

- **WebSocket** 只需要一个 TCP 监听,没有外部依赖,所以直接跟 HTTP+gRPC
  同处一个 kratos.App,而不是单独开 `contrib/go-kratos/websocket/` 子目录。
- **MQTT** 必须依赖外部 broker(如 docker 里的 eclipse-mosquitto)。技术上
  当然可以在 `docker-compose.yml` 里加一个 broker 容器,但那样 MQTT 就不再
  是"多一个 transport"这么轻量了,而是变成 pub/sub 语义的示例。这里有意
  跳过 MQTT;真正的 MQTT 示例应该独立到自己的子目录,带上自己的 broker。

## 改造对照:原生 Kratos → Go-Spring + 注册发现

| 关注点         | Kratos 脚手架                                            | Go-Spring 版本                                                                            |
| -------------- | -------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| 启动           | `kratos.New(...).Run()` 占有进程                         | `KratosServer` 实现 `gs.Server`,由 `gs.Run()` 驱动 Run/Stop                              |
| 依赖装配       | `google/wire` `ProviderSet` + 生成的 `wire_gen.go`       | 每层 `init()` + `gs.Provide`;`provider/main.go` 用空导入触发注册                          |
| handler        | `internal/server` 中 `v1.RegisterGreeterHTTPServer(...)` | `ServiceRegister` bean 把 `GreeterService` 同时绑到 HTTP、gRPC、WebSocket 三个 transport |
| 是否启用       | 总是开启                                                 | `KratosServer` 通过 `gs.OnBean` 条件依赖 `ServiceRegister` bean                            |
| 配置来源       | `configs/config.yaml` 扫描进 `conf.proto` `Bootstrap`    | `conf/app.properties`,经 `value:"${spring.kratos.http}"` / `${spring.kratos.grpc}` 绑定  |
| 服务注册       | 无(直连)                                               | provider `kratos.Registrar(etcd.New(clientv3.New(...)))` 注册进 etcd                       |
| 服务发现       | consumer `transgrpc.WithEndpoint("host:port")` 直连      | consumer `transgrpc.WithEndpoint("discovery:///<name>") + WithDiscovery(etcd.New(...))`   |
| 关停           | `kratos.App` 自己捕获 SIGTERM                            | 由 Go-Spring 协调优雅关停(SIGTERM → `Stop()` → `App.Stop()`,注销 etcd 注册)             |

`provider/server.go` 里的适配器是关键:Kratos 的服务注册发生在 `kratos.App` 层面
(而非单个 transport),因此把 `khttp.Server`、`kgrpc.Server` 与 kratos-transport
的 `websocket.Server` 都构建好后一并交给 `kratos.New(...)`,并注入
`kratos.Registrar(etcdRegistry)`。`App.Run` 会绑定每一个监听、把服务实例(三个
endpoint,按 kratos "kind" 打标)发布进 etcd 后永久阻塞,因此放在一个仅在
`sig.TriggerAndWait()` 之后启动的 goroutine 中运行;`Run` 阻塞在 done channel 上,
由 `Stop()` 关闭,再把控制权交回 Go-Spring 的关停流程(由它调用 `App.Stop`
逐个注销并停止各 transport)。

consumer 侧只提供 etcd 地址和服务名:通过 `transgrpc.WithDiscovery(r)` 装上
`discovery:///` scheme 后,kratos 会在 etcd 里查到一个存活的 provider 并用 gRPC
调用它。WebSocket 那一路则直接拨 `ws://` 地址(`--ws ws://127.0.0.1:9002/`),
因为 kratos-transport 的 WS client 没有 discovery 钩子,单纯为了演示多一种
transport 而硬造 discovery 反而遮蔽了它的重点。gRPC 那一路证明发现能跑通,
WebSocket 这一路证明它能与另两种 transport 共处一个 App。

## WebSocket wire 格式

kratos-transport 的 WebSocket 是**按消息类型分派**的裸帧管道,不是 RPC。本示例
使用 `PayloadTypeBinary`,每一帧的格式是:

```
<4 字节小端 uint32 messageType><JSON 序列化的 payload>
```

`messageType` 是应用自定义的整型判别式,server 端据此把帧路由到对应 handler;
`payload` 是应用结构体的 JSON 编码。Greeter 示例用 `messageType=1`,请求
`{"name":"<x>"}`,响应 `{"message":"Hello <x>"}`。因为不是 RPC,所以没有 proto
契约;那一条常量和两个结构体就是全部的约定,由 provider(`provider/handler.go`)
和 consumer(`consumer/main.go`)分别持有。

之所以选二进制而不是库自带的 text envelope,是因为在这一版库里 text 模式的
wire 格式是**非对称的**:server 收帧时会拆 `{"type","payload"}` envelope,但
回帧时又只吐 codec 裸字节(不包一层),意味着 consumer 得为两个方向准备两套
解析。二进制模式对称 —— server 出去和进来的头 4 字节都一样 —— 一套
marshal/unmarshal 就够。库版本 pin 在 `v1.3.1` 的原因见 `provider/server.go` 的注释。

## 注册中心的选择

本示例统一用 **etcd** 便于与其他 contrib 示例横向对比。Kratos contrib 同样支持
**Consul**、**Nacos**、**ZooKeeper**、**Polaris** 等:只需把 provider 的
`etcd.New(clientv3.New(...))` 与 consumer 的对应调用换成 `consul.New(...)` /
`nacos.New(...)` / `zookeeper.New(...)` / `polaris.New(...)`,并调整 client 配置
即可。选用 Nacos 时还能通过其自带的 `:8848/nacos` 控制台直接查看已注册的服务列表。

## 配置

```properties
# 关闭内置 HTTP server,provider 只暴露 kratos transport。
spring.http.server.enabled=false

# 应用名 —— kratos.App 注册到 etcd 的键。
spring.kratos.name=kratos-greeter

# Kratos HTTP transport,经 ${spring.kratos.http} 前缀绑定。
spring.kratos.http.addr=0.0.0.0:8000
spring.kratos.http.timeout=1s

# Kratos gRPC transport,经 ${spring.kratos.grpc} 前缀绑定。
spring.kratos.grpc.addr=0.0.0.0:9000
spring.kratos.grpc.timeout=1s

# Kratos WebSocket transport(kratos-transport/transport/websocket),
# 经 ${spring.kratos.ws} 前缀绑定,与 HTTP+gRPC 共处一个 kratos.App。
spring.kratos.ws.addr=0.0.0.0:9002
spring.kratos.ws.path=/

# etcd 注册中心地址,与 docker-compose.yml 一致。
spring.kratos.registry.etcd=127.0.0.1:2379
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
Response from discovered provider (gRPC): Hello Kratos
Response from discovered provider (WebSocket): Hello Kratos-WS
```

或一键冒烟(自动起 etcd + provider、跑 consumer、清理):

```bash
bash scripts/smoke-test.sh
```
