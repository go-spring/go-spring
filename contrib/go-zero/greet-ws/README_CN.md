# go-zero — WebSocket（Go-Spring 风格）

[English](README.md) | [中文](README_CN.md)

一个 [go-zero](https://go-zero.dev) 的 `Greet` 示例，通过 **WebSocket** 提供服务，
并按 Go-Spring 的方式启动与配置：`gs.Run()` 驱动生命周期，业务逻辑是一个 IoC bean，
监听地址来自 `conf/app.properties` 而非在 `main()` 中硬编码。

这是 `contrib/go-zero/` 下的第三个协议子项目，与
[`../greet-api`](../greet-api)（HTTP）和 [`../greet-rpc`](../greet-rpc)（zRPC/gRPC）
并列。WebSocket 单独立一个示例是因为：它是 go-zero 在 `rest.Server` 之上
唯一暴露的**非** HTTP 请求/响应范式，而且它天然带来一种 `.api` DSL 无法表达的形态
（长连接、按帧循环处理）。

由此产生两个结论，都是有意的：

- **没有 etcd、没有服务发现。** WS 复用 `rest.Server`，与 `greet-api` 一致。
  go-zero 的注册中心逻辑只存在于 zRPC 层，因此 consumer 直接连接固定的
  `host:port`。
- **没有 goctl 生成的文件。** goctl 的 `.api` DSL 只理解请求/响应式 HTTP 接口，
  无法描述 WS 路由或帧类型。这里所有文件都是手写的：帧类型放在 `idl/greet.go`，
  升级循环与 `GreetLogic` bean 放在 `provider/handler.go`。
  `idl/gen-code.sh` 是一个明确的 no-op，仅用于与两个兄弟子项目保持相同的入口约定。

这是一个可运行的示例，**不是**可复用的 starter 模块。

## 拓扑

```
┌────────────┐        WS /greet             ┌────────────┐
│  provider  │◀─────────────────────────────│  consumer  │
│ gs.Run()   │  {"name":"Hello, go-zero!"}  │ 一次性     │
│  :8890     │──────────────────────────────▶│ 断言并退出 │
└────────────┘   {"greeting":"Hello, ..."}  └────────────┘
        （持久 WebSocket，各方向一帧）
```

## 目录结构

```
contrib/go-zero/greet-ws/
├── idl/gen-code.sh                     # 说明性 no-op（go-zero WS 无 IDL 生成）
├── idl/greet.go                        # 手写：WS 帧载荷（JSON），即“IDL”
├── provider/handler.go                 # 手写：HandlerRegister bean、WS 升级循环、GreetLogic bean
├── provider/server.go                  # RestServer 适配器（gs.Server）+ Config
├── provider/main.go                    # gs.Run()；常驻进程
├── consumer/main.go                    # WS 拨号，断言 echo，退出
├── conf/app.properties                 # provider 配置（含可观测）
├── docker-compose.yml                  # 可观测后端（prometheus/jaeger/loki/promtail）
├── docker/                             # prometheus.yml + promtail-config.yml
└── scripts/smoke-test.sh               # 冒烟测试：起后端 → 起 provider → 跑 consumer → 断言 → 收尾
```

## WebSocket 与 `greet-api` / `greet-rpc` 的差异

| 关注点      | `greet-api`（.api HTTP）                       | `greet-rpc`（zRPC/gRPC）                    | `greet-ws`（本项目）                                                       |
| ----------- | ---------------------------------------------- | ------------------------------------------- | -------------------------------------------------------------------------- |
| 服务器      | `rest.Server`                                  | `zrpc.RpcServer`                            | `rest.Server`（与 greet-api 相同）                                         |
| IDL / 代码生成 | `greet.api` + `goctl api go`                | `greet.proto` + `goctl rpc protoc`          | 无 — go-zero 的 WS 没有 IDL                                                |
| 传输形态    | 一次 HTTP 请求 → 一次响应，连接可池化          | 一次 gRPC 调用 → 一次响应，HTTP/2 多路复用  | 一条 TCP 连接被升级，双向持续按帧收发直到关闭                              |
| Handler 形态| 解析 → 调 logic → JSON 渲染                    | proto 生成的方法                            | upgrade → `conn.ReadMessage` for 循环 → 每帧分发                           |
| 服务发现    | 无（rest.Server 没有注册中心）                 | 通过 zRPC 的 `EtcdConf` 走 etcd             | 无（WS 继承了 rest.Server 的限制）                                         |
| Consumer    | `http.Post` + JSON 解码                        | zRPC 客户端，resolver `etcd://…`            | `websocket.Dialer.Dial` + 一次帧交换                                       |
| 启动        | `RestServer` 实现 `gs.Server`；`gs.Run()` 驱动 | `RpcServer` 实现 `gs.Server`；`gs.Run()` 驱动 | 与 greet-api 完全一致 —— 适配器代码相同                                    |

`provider/server.go` 的适配器代码因此与 `greet-api` 是有意保持一致的：
WebSocket 由**同一个** `rest.Server` 承载；变化的只是注册进去的
`HandlerRegister` bean 内部行为 —— 调 `httpx.OkJsonCtx`（HTTP）还是升级为 WS。

## 配置

```properties
# 关闭 Go-Spring 内置 HTTP 服务器；provider 只暴露下面绑定的 go-zero rest.Server。
spring.http.server.enabled=false

# go-zero rest.Server 设置，通过 ${spring.rest.server} 前缀读取。
# 端口 8890（而非 greet-api 的 8888），以便两个示例可并存运行不冲突。
spring.rest.server.name=greet-ws
spring.rest.server.host=0.0.0.0
spring.rest.server.port=8890

# 可观测（仅 provider），详见下文可观测章节。
spring.rest.server.tracing.endpoint=127.0.0.1:4317
spring.rest.server.metrics.port=6060
spring.rest.server.log.mode=file
spring.rest.server.log.path=../logs
```

## 可观测

WS 与 `greet-api` 共用同一个 `rest.Server`，因此可观测接法完全一致：
`rest.MustNewServer` 内部会调用 `service.ServiceConf.SetUp()`，自动启动
tracing agent、metrics DevServer 与 logx；`rest.Server` 的中间件
（Trace/Prometheus/Metrics/Log，默认全开）随后为每次请求埋点 —— 包括
开启 WS 连接的 HTTP upgrade 请求。**我们没有手写任何 OpenTelemetry / Prometheus
代码** —— `provider/server.go` 只是从 `conf/app.properties` 里读取字段填进
`ServiceConf`。

| 支柱   | go-zero 字段            | 后端（docker-compose.yml）              |
| ------ | ----------------------- | ---------------------------------------- |
| Tracing | `ServiceConf.Telemetry` | Jaeger via OTLP/gRPC（:4317，UI 16686）  |
| Metrics | `ServiceConf.DevServer` | Prometheus 抓 :6060/metrics（UI 9099）   |
| Logging | `ServiceConf.Log`（logx）| JSON 文件 → Promtail → Loki（:3100）    |

只有 **provider** 被埋点；consumer 是裸的 `gorilla/websocket` 客户端。
logx 会把当前 trace/span 打进每条日志，因此 Loki 里的日志能与 Jaeger
里的 span 关联。

起后端并跑一次带断言的冒烟测试：

```bash
docker compose up -d
bash scripts/smoke-test.sh   # 断言 /metrics 暴露 go_* 进程指标
```

冒烟断言故意用 `go_*`（始终在线）而不是 `http_server_requests_*`：因为 WS 是
长连接，`rest.Server` 的 Prometheus 中间件只在连接关闭时才落这条 HTTP 请求样本，
断言时机不稳定。consumer 断开后，`http_server_requests_*` 也会针对 WS
upgrade 请求出现。

provider 运行且发生过一次请求后的手动验证：

- **Metrics**：Prometheus UI http://127.0.0.1:9099，查询 `go_goroutines`
  作为在线信号；WS 请求指标 `http_server_requests_*` 会在连接关闭后出现。
- **Traces**：Jaeger UI http://127.0.0.1:16686，服务选 `greet-ws`。
- **Logs**：`curl -s 'http://127.0.0.1:3100/loki/api/v1/query_range?query=%7Bjob%3D%22greet-ws%22%7D'`。

## 运行

终端 A —— 启动 provider（常驻）：

```bash
go run ./provider
```

终端 B —— 启动 consumer（WS 拨号，一帧往返，自断言）：

```bash
go run ./consumer
```

consumer 预期输出：

```
Response from provider: Hello, go-zero!
```

或直接跑一次冒烟测试（起 provider、跑 consumer、然后收尾）：

```bash
bash scripts/smoke-test.sh
```

## 关于 `idl/gen-code.sh`

`idl/gen-code.sh` 是一个明确的 no-op —— 只是打印一条说明后退出。WebSocket 无法用
go-zero 的 `.api` DSL 表达，`goctl api go` 对路由和帧类型都无话可说。
可对比 `../greet-api/idl/gen-code.sh`（驱动 `goctl api go`）与 `../greet-rpc/idl/gen-code.sh`
（驱动 `goctl rpc protoc`）。要改 WS 字段或加路由，请直接编辑 `idl/greet.go`
与 `provider/handler.go`。
