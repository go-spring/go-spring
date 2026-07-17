# go-kratos —— WebSocket(Go-Spring 风格)

[English](README.md) | [中文](README_CN.md)

一个以 Go-Spring 方式驱动的 [go-kratos](https://github.com/go-kratos/kratos)
`Greeter` 示例:`gs.Run()` 掌管生命周期,服务是一个 IoC bean,kratos WebSocket
transport server(经
[`github.com/tx7do/kratos-transport`](https://github.com/tx7do/kratos-transport),
锁定 v1.3.1)由 [`starter-kratos/ws`](../../../starter/starter-kratos)
starter 贡献,而非在 `main()` 里手工接线。

与 HTTP、gRPC 两半不同,kratos-transport WebSocket 承载的是**应用自定义的帧消息,
而不是 proto RPC**,并且其客户端**没有服务发现钩子**。因此 provider 启动时仍会
把自己注册进 **etcd 注册中心**(以证明生命周期端到端可用),但 consumer 是直接
dial `ws://` URL,而不是通过 `discovery:///<name>` 解析。

这是一个可运行的示例,**不是**可复用的 starter 模块。同一 `Greeter` 服务的 HTTP
与 gRPC 两半在隔壁 [`../http`](../http) 与 [`../grpc`](../grpc) —— 各自是独立
module,接线自己的 kratos transport,因此 import 其中一个绝不会拖入其它的依赖。

## 拓扑

```
                ┌──────────────┐
    注册         │     etcd     │
  ┌────────────▶│  :2379       │
  │  kratos-ws  └──────────────┘
  │  (name)
  │
┌─┴───────────┐        WS :9002          ┌─────────────┐
│  provider   │◀─────────────────────────│  consumer   │
│ gs.Run()    │      SayHello("Kratos")  │ gs.Run()    │
└─────────────┘   直接 ws:// dial         └─────────────┘
```

## 结构

```
ws/
├── provider/            gs.Run() + ServiceRegister bean(handler.go 把
│   │                    message-type 1 绑定到 handler,而不是 proto RPC)
│   └── conf/app.properties
├── consumer/            直连 dial 的 WebSocket 客户端(不走 etcd 发现)
│   └── conf/app.properties
├── idl/helloworld/v1/   proto + 生成的 stub(gen-code.sh 重新生成)
├── docker-compose.yml   仅 etcd
└── scripts/smoke-test.sh
```

线上格式:每一帧都是 `<4 字节小端 uint32 messageType><JSON payload>`
(PayloadTypeBinary)。server 与 client 通过带外约定 message type 为 `1`。

## 运行

```bash
# 1. 启动 etcd。
docker compose up -d

# 2. 启动 provider(把 kratos-ws 注册进 etcd,监听 WS :9002)。
go run ./provider

# 3. 另开一个终端,运行 consumer(直接 dial ws://127.0.0.1:9002/)。
go run ./consumer
# → Response from discovered provider (WebSocket): Hello Kratos-WS
```

或一键跑完整回环:

```bash
./scripts/smoke-test.sh
```

## 配置

kratos WebSocket server 从 `${spring.kratos.ws.server}` 前缀绑定(见
`provider/conf/app.properties`):

| 键 | 默认值 | 含义 |
| --- | --- | --- |
| `spring.kratos.ws.server.name` | `kratos-ws` | 发布进 etcd 的服务名 |
| `spring.kratos.ws.server.addr` | `0.0.0.0:9002` | WebSocket 监听地址 |
| `spring.kratos.ws.server.path` | `/` | WebSocket 升级路径 |
| `spring.kratos.ws.server.etcd.addr` | *(空)* | etcd 端点;空 = 不注册 |

HTTP / gRPC 的可观测(tracing/metrics)让路给
[`starter-otel`](../../../starter/starter-otel)。WebSocket 在此处**有意**不做
埋点:kratos-transport 的 WS server 没有可挂载的中间件链,因此在应用层自行接入
tracer 之前,该 transport 是一个盲点。
